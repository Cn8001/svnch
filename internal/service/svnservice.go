package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Cn8001/svnch/internal/svn"
)

// Credentials is the runner's session login, kept apart from Runner so tests and read-only callers need not fake it.
type Credentials interface {
	SetCredentials(username, password string)
	Credentials() (username, password string)
}

type SVNService struct {
	runner svn.Runner
	creds  Credentials
	web    *http.Client // reads the repository index at a server root, which svn itself cannot list
}

func New(runner svn.Runner, creds Credentials, web *http.Client) *SVNService {
	return &SVNService{runner: runner, creds: creds, web: web}
}

// Login checks the credentials against url and keeps them for the session; a wrong password is cleared again so the
// next command fails with the same auth error instead of a confusing one. It checks with List because the url may be
// a server root that only the repository index answers for.
func (s *SVNService) Login(ctx context.Context, url, username, password string) error {
	if url == "" || username == "" {
		return fmt.Errorf("login needs a url and a username: %w", svn.ErrInvalidRequest)
	}
	s.creds.SetCredentials(username, password)
	if _, err := s.List(ctx, url); err != nil {
		s.creds.SetCredentials("", "")
		return fmt.Errorf("login to %s: %w", url, err)
	}
	return nil
}

func (s *SVNService) Logout() {
	s.creds.SetCredentials("", "")
}

// LoggedInAs is the username svnch passes for this session; svn may also log in on its own with a password it saved.
func (s *SVNService) LoggedInAs() string {
	username, _ := s.creds.Credentials()
	return username
}

// run takes the args builder's error as well, so each command reads as one line of build, run, parse.
func (s *SVNService) run(ctx context.Context, dir, subcommand string, args []string, argErr error) ([]byte, error) {
	if argErr != nil {
		return nil, argErr
	}
	return s.runner.Run(ctx, dir, subcommand, args...)
}

func (s *SVNService) Info(ctx context.Context, target string) (svn.Info, error) {

	// Give 30 seconds to info command
	runctx, stop := context.WithTimeout(ctx, time.Second*30)
	defer stop()

	out, err := s.runner.Run(runctx, "", "info", svn.InfoArgs(target)...)

	if err != nil {
		return svn.Info{}, fmt.Errorf("info %s: %w", target, err)
	}
	return svn.ParseInfo(out)
}

// Checkout has no timeout of its own because a large repository can take many minutes; the caller cancels through ctx.
func (s *SVNService) Checkout(ctx context.Context, req svn.CheckoutRequest) (int64, error) {
	args, err := svn.CheckoutArgs(req)
	out, err := s.run(ctx, "", "checkout", args, err)
	if err != nil {
		return 0, fmt.Errorf("checkout %s: %w", req.URL, err)
	}
	return svn.ParseCheckout(out)
}

func (s *SVNService) Status(ctx context.Context, path string) ([]svn.StatusEntry, error) {
	args, err := svn.StatusArgs(path)
	out, err := s.run(ctx, "", "status", args, err)
	if err != nil {
		return nil, fmt.Errorf("status %s: %w", path, err)
	}
	return svn.ParseStatus(out)
}

func (s *SVNService) Update(ctx context.Context, paths []string) (int64, error) {
	args, err := svn.UpdateArgs(paths...)
	out, err := s.run(ctx, "", "update", args, err)
	if err != nil {
		return 0, fmt.Errorf("update: %w", err)
	}
	return svn.ParseUpdate(out)
}

func (s *SVNService) UpdateTo(ctx context.Context, path string, revision int64) (int64, error) {
	args, err := svn.UpdateToArgs(path, revision)
	out, err := s.run(ctx, "", "update", args, err)
	if err != nil {
		return 0, fmt.Errorf("update %s to r%d: %w", path, revision, err)
	}
	return svn.ParseUpdate(out)
}

// Branch returns the revision the copy was committed as.
func (s *SVNService) Branch(ctx context.Context, req svn.BranchRequest) (int64, error) {
	args, err := svn.BranchArgs(req)
	out, err := s.run(ctx, "", "copy", args, err)
	if err != nil {
		return 0, fmt.Errorf("branch %s: %w", req.To, err)
	}
	return svn.ParseCommit(out)
}

func (s *SVNService) Switch(ctx context.Context, wc, url string) (int64, error) {
	args, err := svn.SwitchArgs(url, wc)
	out, err := s.run(ctx, "", "switch", args, err)
	if err != nil {
		return 0, fmt.Errorf("switch to %s: %w", url, err)
	}
	return svn.ParseUpdate(out)
}

func (s *SVNService) SetChangelist(ctx context.Context, name string, paths []string) error {
	args, err := svn.ChangelistArgs(name, paths...)
	if _, err := s.run(ctx, "", "changelist", args, err); err != nil {
		return fmt.Errorf("changelist %s: %w", name, err)
	}
	return nil
}

func (s *SVNService) RemoveChangelist(ctx context.Context, paths []string) error {
	args, err := svn.RemoveChangelistArgs(paths...)
	if _, err := s.run(ctx, "", "changelist", args, err); err != nil {
		return fmt.Errorf("remove from changelist: %w", err)
	}
	return nil
}

// DeleteRemote, MoveRemote and MakeDir change the repository directly; each returns the revision it committed.
func (s *SVNService) DeleteRemote(ctx context.Context, urls []string, message string) (int64, error) {
	args, err := svn.RemoteDeleteArgs(message, urls...)
	out, err := s.run(ctx, "", "delete", args, err)
	if err != nil {
		return 0, fmt.Errorf("delete on server: %w", err)
	}
	return svn.ParseCommit(out)
}

func (s *SVNService) MoveRemote(ctx context.Context, from, to, message string) (int64, error) {
	args, err := svn.RemoteMoveArgs(from, to, message)
	out, err := s.run(ctx, "", "move", args, err)
	if err != nil {
		return 0, fmt.Errorf("move %s on server: %w", from, err)
	}
	return svn.ParseCommit(out)
}

func (s *SVNService) MakeDir(ctx context.Context, url, message string) (int64, error) {
	args, err := svn.MkdirArgs(url, message)
	out, err := s.run(ctx, "", "mkdir", args, err)
	if err != nil {
		return 0, fmt.Errorf("new folder %s: %w", url, err)
	}
	return svn.ParseCommit(out)
}

// Cat reads a file straight from the repository; the browser only asks for small files, so the whole of it is fine.
func (s *SVNService) Cat(ctx context.Context, url string) (svn.FileContent, error) {
	args, err := svn.CatArgs(url)
	out, err := s.run(ctx, "", "cat", args, err)
	if err != nil {
		return svn.FileContent{}, fmt.Errorf("open %s: %w", url, err)
	}
	return svn.ParseCat(out), nil
}

func (s *SVNService) Export(ctx context.Context, url, dest string) error {
	args, err := svn.ExportArgs(url, dest)
	if _, err := s.run(ctx, "", "export", args, err); err != nil {
		return fmt.Errorf("save %s: %w", url, err)
	}
	return nil
}

// Commit returns 0 when there was nothing to commit.
func (s *SVNService) Commit(ctx context.Context, req svn.CommitRequest) (int64, error) {
	args, err := svn.CommitArgs(req)
	out, err := s.run(ctx, "", "commit", args, err)
	if err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return svn.ParseCommit(out)
}

func (s *SVNService) Add(ctx context.Context, paths []string) error {
	args, err := svn.AddArgs(paths...)
	if _, err := s.run(ctx, "", "add", args, err); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	return nil
}

// Delete with keepLocal stops versioning the paths but leaves the files on disk.
func (s *SVNService) Delete(ctx context.Context, paths []string, keepLocal bool) error {
	args, err := svn.DeleteArgs(keepLocal, paths...)
	if _, err := s.run(ctx, "", "delete", args, err); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

func (s *SVNService) Revert(ctx context.Context, paths []string) error {
	args, err := svn.RevertArgs(paths...)
	if _, err := s.run(ctx, "", "revert", args, err); err != nil {
		return fmt.Errorf("revert: %w", err)
	}
	return nil
}

// Discard throws away the local changes under paths. It reads the status again rather than trusting the caller, so
// only what svn reports as never committed is deleted, and returns how many items it reverted and deleted.
func (s *SVNService) Discard(ctx context.Context, wc string, paths []string) (svn.DiscardResult, error) {
	var entries []svn.StatusEntry
	for _, p := range paths {
		e, err := s.Status(ctx, p)
		if err != nil {
			return svn.DiscardResult{}, fmt.Errorf("discard: %w", err)
		}
		entries = append(entries, e...)
	}
	plan := svn.PlanDiscard(wc, entries)
	if len(plan.Revert) > 0 {
		if err := s.Revert(ctx, plan.Revert); err != nil {
			return svn.DiscardResult{}, fmt.Errorf("discard: %w", err)
		}
	}
	for _, p := range plan.Remove {
		if err := os.RemoveAll(p); err != nil {
			return svn.DiscardResult{Reverted: len(plan.Revert)}, fmt.Errorf("discard, delete %s: %w", p, err)
		}
	}
	return svn.DiscardResult{Reverted: len(plan.Revert), Removed: len(plan.Remove)}, nil
}

func (s *SVNService) Resolve(ctx context.Context, accept string, paths []string) error {
	args, err := svn.ResolveArgs(accept, paths...)
	if _, err := s.run(ctx, "", "resolve", args, err); err != nil {
		return fmt.Errorf("resolve with %s: %w", accept, err)
	}
	return nil
}

// localLimit keeps the conflict view to files a person reads; anything bigger is opened in its own program.
const localLimit = 2 << 20

// ReadLocal reads a file of the working copy for the conflict view, and only one inside wc.
func (s *SVNService) ReadLocal(ctx context.Context, wc, path string) (svn.FileContent, error) {
	if !svn.Inside(wc, path) {
		return svn.FileContent{}, fmt.Errorf("%s is not in the working copy: %w", path, svn.ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return svn.FileContent{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return svn.FileContent{}, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, localLimit+1))
	if err != nil {
		return svn.FileContent{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > localLimit {
		return svn.FileContent{Binary: true, Size: len(data)}, nil
	}
	return svn.ParseCat(data), nil
}

func (s *SVNService) Move(ctx context.Context, from, to string) error {
	args, err := svn.MoveArgs(from, to)
	if _, err := s.run(ctx, "", "move", args, err); err != nil {
		return fmt.Errorf("move %s to %s: %w", from, to, err)
	}
	return nil
}

// CheckConnection is List with a short limit: it answers "can I reach this" while a server is being added, and a
// wrong address or a missing VPN should say so in seconds rather than after List's full minute.
func (s *SVNService) CheckConnection(ctx context.Context, url string) ([]svn.ListEntry, error) {
	ctx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	return s.List(ctx, url)
}

// List has a timeout because a server that never answers would otherwise hold the browser until the OS gives up on the
// connection, which can be minutes.
func (s *SVNService) List(ctx context.Context, target string) ([]svn.ListEntry, error) {
	ctx, stop := context.WithTimeout(ctx, time.Minute)
	defer stop()
	args, err := svn.ListArgs(target)
	out, err := s.run(ctx, "", "list", args, err)
	if err == nil {
		return svn.ParseList(out)
	}
	if !serverRoot(target, err) {
		return nil, fmt.Errorf("list %s: %w", target, err)
	}
	username, password := s.creds.Credentials()
	repos, idxErr := svn.ListCollection(ctx, s.web, target, username, password)
	switch {
	case idxErr == nil:
		return repos, nil
	case errors.Is(idxErr, svn.ErrAuthFailed):
		return nil, fmt.Errorf("list %s: %w", target, idxErr)
	}
	// svn's error stays the one to match on; the index's says why the server root did not list either.
	return nil, fmt.Errorf("list %s: %w; as a server root: %v", target, err, idxErr)
}

// serverRoot tells whether a failed list may have been pointed at the folder holding the repositories; only a web
// server can answer for that folder, and an auth or certificate failure already says what is wrong.
func serverRoot(target string, err error) bool {
	var ce *svn.CommandError
	web := strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
	return web && errors.As(err, &ce) && !errors.Is(err, svn.ErrAuthFailed) && !errors.Is(err, svn.ErrCertUntrusted)
}

func (s *SVNService) Diff(ctx context.Context, req svn.DiffRequest) (string, error) {
	args, err := svn.DiffArgs(req)
	out, err := s.run(ctx, "", "diff", args, err)
	if err != nil {
		return "", fmt.Errorf("diff %s: %w", req.Target, err)
	}
	return string(out), nil
}

func (s *SVNService) Log(ctx context.Context, req svn.LogRequest) ([]svn.LogEntry, error) {
	args, err := svn.LogArgs(req)
	out, err := s.run(ctx, "", "log", args, err)
	if err != nil {
		return nil, fmt.Errorf("log %s: %w", req.Target, err)
	}
	return svn.ParseLog(out)
}

// Merge returns svn's own summary, since its lines (U a.txt, C b.txt) are what a user reads to judge a merge.
func (s *SVNService) Merge(ctx context.Context, req svn.MergeRequest) (string, error) {
	args, err := svn.MergeArgs(req)
	out, err := s.run(ctx, "", "merge", args, err)
	if err != nil {
		return "", fmt.Errorf("merge %s: %w", req.Source, err)
	}
	return string(out), nil
}

// The shelve commands act on the working copy around the current directory, so they run inside wc.
func (s *SVNService) Shelve(ctx context.Context, wc, name, message string) error {
	args, err := svn.ShelveArgs(name, message)
	if _, err := s.run(ctx, wc, svn.ShelveCommand, args, err); err != nil {
		return fmt.Errorf("shelve %s: %w", name, err)
	}
	return nil
}

func (s *SVNService) Unshelve(ctx context.Context, wc, name string) error {
	args, err := svn.UnshelveArgs(name)
	if _, err := s.run(ctx, wc, svn.UnshelveCommand, args, err); err != nil {
		return fmt.Errorf("unshelve %s: %w", name, err)
	}
	return nil
}

func (s *SVNService) Shelves(ctx context.Context, wc string) ([]string, error) {
	out, err := s.runner.Run(ctx, wc, svn.ShelfListCommand)
	if err != nil {
		return nil, fmt.Errorf("list shelves: %w", err)
	}
	return svn.ParseShelfList(out), nil
}

// Help with an empty subcommand lists every command.
func (s *SVNService) Help(ctx context.Context, subcommand string) (string, error) {
	var args []string
	if subcommand != "" {
		if strings.HasPrefix(subcommand, "-") {
			return "", fmt.Errorf("help topic %q: %w", subcommand, svn.ErrInvalidRequest)
		}
		args = []string{subcommand}
	}
	out, err := s.runner.Run(ctx, "", "help", args...)
	if err != nil {
		return "", fmt.Errorf("help: %w", err)
	}
	return string(out), nil
}
