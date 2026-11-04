package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Cn8001/svnch/internal/svn"
)

type fakeRunner struct {
	out        []byte
	err        error
	calls      int
	dir        string
	subcommand string
	args       []string
	username   string
	password   string
}

func (f *fakeRunner) SetCredentials(username, password string) {
	f.username, f.password = username, password
}

func (f *fakeRunner) Credentials() (string, string) { return f.username, f.password }

func (f *fakeRunner) Run(_ context.Context, dir, subcommand string, args ...string) ([]byte, error) {
	f.calls++
	f.dir, f.subcommand, f.args = dir, subcommand, args
	return f.out, f.err
}

func TestCheckout_ReturnsCheckedOutRevision(t *testing.T) {
	// Arrange
	runner := &fakeRunner{out: []byte("A    wc/a.txt\nChecked out revision 88.\n")}
	s := New(runner, runner, http.DefaultClient)

	// Act
	rev, err := s.Checkout(context.Background(), svn.CheckoutRequest{URL: "svn://host/repo", Path: "wc"})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev != 88 {
		t.Errorf("rev = %d, want 88", rev)
	}
	if runner.subcommand != "checkout" || !slices.Equal(runner.args, []string{"--", "svn://host/repo", "wc"}) {
		t.Errorf("ran %s %q", runner.subcommand, runner.args)
	}
}

func TestCheckout_InvalidRequestDoesNotRunSvn(t *testing.T) {
	// Arrange
	runner := &fakeRunner{}
	s := New(runner, runner, http.DefaultClient)

	// Act
	_, err := s.Checkout(context.Background(), svn.CheckoutRequest{Path: "wc"})

	// Assert
	if !errors.Is(err, svn.ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
	if runner.calls != 0 {
		t.Errorf("runner called %d times, want 0", runner.calls)
	}
}

func TestInfo_KeepsSvnErrorMatchable(t *testing.T) {
	// Arrange
	runner := &fakeRunner{err: &svn.CommandError{Subcommand: "info", Code: "E155007"}}
	s := New(runner, runner, http.DefaultClient)

	// Act
	_, err := s.Info(context.Background(), "/tmp")

	// Assert
	if !errors.Is(err, svn.ErrNotWorkingCopy) {
		t.Errorf("err = %v, want ErrNotWorkingCopy", err)
	}
}

func TestStatus_ParsesEntries(t *testing.T) {
	// Arrange
	runner := &fakeRunner{out: []byte(`<status><target path="/wc"><entry path="/wc/a"><wc-status item="modified" props="none" revision="3"></wc-status></entry></target></status>`)}
	s := New(runner, runner, http.DefaultClient)

	// Act
	entries, err := s.Status(context.Background(), "/wc")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Item != "modified" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestCommit_EmptyMessageDoesNotRunSvn(t *testing.T) {
	// Arrange
	runner := &fakeRunner{}
	s := New(runner, runner, http.DefaultClient)

	// Act
	_, err := s.Commit(context.Background(), svn.CommitRequest{Paths: []string{"/wc"}})

	// Assert
	if !errors.Is(err, svn.ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
	if runner.calls != 0 {
		t.Errorf("runner called %d times, want 0", runner.calls)
	}
}

func TestDiff_ReturnsRawText(t *testing.T) {
	// Arrange
	diff := "Index: a.txt\n===\n--- a.txt\t(revision 3)\n+++ a.txt\t(working copy)\n@@ -1 +1 @@\n-old\n+new\n"
	runner := &fakeRunner{out: []byte(diff)}
	s := New(runner, runner, http.DefaultClient)

	// Act
	got, err := s.Diff(context.Background(), svn.DiffRequest{Target: "/wc/a.txt"})

	// Assert
	if err != nil || got != diff {
		t.Errorf("got (%q, %v)", got, err)
	}
}

func TestShelve_RunsInsideWorkingCopy(t *testing.T) {
	// Arrange
	runner := &fakeRunner{}
	s := New(runner, runner, http.DefaultClient)

	// Act
	err := s.Shelve(context.Background(), "/wc", "wip", "")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.dir != "/wc" || runner.subcommand != svn.ShelveCommand {
		t.Errorf("ran %s in %q", runner.subcommand, runner.dir)
	}
}

func TestHelp_RejectsFlagAsTopic(t *testing.T) {
	// Arrange
	runner := &fakeRunner{}
	s := New(runner, runner, http.DefaultClient)

	// Act
	_, err := s.Help(context.Background(), "--version")

	// Assert
	if !errors.Is(err, svn.ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
	if runner.calls != 0 {
		t.Errorf("runner called %d times, want 0", runner.calls)
	}
}

func TestLogin_WrongPasswordIsCleared(t *testing.T) {
	// Arrange
	runner := &fakeRunner{err: &svn.CommandError{Subcommand: "info", Code: "E215004"}}
	s := New(runner, runner, http.DefaultClient)

	// Act
	err := s.Login(context.Background(), "https://svn.example.org/repo", "me", "wrong")

	// Assert
	if !errors.Is(err, svn.ErrAuthFailed) {
		t.Errorf("err = %v, want ErrAuthFailed", err)
	}
	if runner.username != "" || runner.password != "" {
		t.Errorf("credentials kept: %q %q", runner.username, runner.password)
	}
}

func TestLogin_KeepsWorkingCredentials(t *testing.T) {
	// Arrange
	runner := &fakeRunner{out: []byte(`<lists><list path="https://svn.example.org/repo"></list></lists>`)}
	s := New(runner, runner, http.DefaultClient)

	// Act
	err := s.Login(context.Background(), "https://svn.example.org/repo", "me", "secret")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.username != "me" || runner.password != "secret" {
		t.Errorf("credentials = %q %q", runner.username, runner.password)
	}
}

func TestToFailure_MapsAuthAndKeepsSvnCode(t *testing.T) {
	// Arrange
	err := fmt.Errorf("update: %w", &svn.CommandError{Subcommand: "update", Code: "E215004", Stderr: "svn: E215004: Authentication failed"})

	// Act
	f := ToFailure(err)

	// Assert
	if f.Code != "AUTH" || f.SvnCode != "E215004" {
		t.Errorf("failure = %+v", f)
	}
}

func TestList_ServerRootFallsBackToRepositoryIndex(t *testing.T) {
	// Arrange
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Collection of Repositories</title></head><body><h2>Collection of Repositories</h2>
<ul><li><a href="alpha/">alpha/</a></li><li><a href="beta%20two/">beta two/</a></li></ul>
<hr noshade><em>Powered by <a href="http://subversion.apache.org/">Apache Subversion</a></em></body></html>`)
	}))
	defer web.Close()
	runner := &fakeRunner{err: &svn.CommandError{Subcommand: "list", Code: "E175002"}}
	s := New(runner, runner, web.Client())

	// Act
	got, err := s.List(context.Background(), web.URL+"/svn")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "beta two" || got[1].Kind != "dir" {
		t.Errorf("entries = %+v", got)
	}
}

func TestList_ServerRootAskingForPasswordIsAuth(t *testing.T) {
	// Arrange
	var sent string
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent, _, _ = r.BasicAuth()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer web.Close()
	runner := &fakeRunner{err: &svn.CommandError{Subcommand: "list", Code: "E175002"}, username: "me", password: "pw"}
	s := New(runner, runner, web.Client())

	// Act
	_, err := s.List(context.Background(), web.URL+"/svn")

	// Assert
	if !errors.Is(err, svn.ErrAuthFailed) {
		t.Errorf("err = %v, want ErrAuthFailed", err)
	}
	if sent != "me" {
		t.Errorf("index sent username %q, want the session login", sent)
	}
}

func TestList_OtherPageKeepsSvnError(t *testing.T) {
	// Arrange
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Welcome</title></head><body><a href="docs/">docs</a></body></html>`)
	}))
	defer web.Close()
	svnErr := &svn.CommandError{Subcommand: "list", Code: "E175002"}
	runner := &fakeRunner{err: svnErr}
	s := New(runner, runner, web.Client())

	// Act
	_, err := s.List(context.Background(), web.URL)

	// Assert
	var ce *svn.CommandError
	if !errors.As(err, &ce) || ce != svnErr {
		t.Errorf("err = %v, want the svn error", err)
	}
}

type slowRunner struct{ fakeRunner }

func (r *slowRunner) Run(ctx context.Context, _, _ string, _ ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestCheckConnection_GivesUpOnASilentServer(t *testing.T) {
	// Arrange
	runner := &slowRunner{}
	s := New(runner, runner, http.DefaultClient)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()

	// Act
	_, err := s.CheckConnection(ctx, "https://svn.example.org/repo")

	// Assert
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
	if took := time.Since(start); took > 15*time.Second {
		t.Errorf("took %s, want about 10s", took)
	}
}

func TestBranch_CopiesOnTheServer(t *testing.T) {
	// Arrange
	runner := &fakeRunner{out: []byte("\nCommitted revision 91.\n")}
	s := New(runner, runner, http.DefaultClient)

	// Act
	rev, err := s.Branch(context.Background(), svn.BranchRequest{From: "https://h/r/trunk", To: "https://h/r/branches/x", Message: "Create branch x"})

	// Assert
	if err != nil || rev != 91 {
		t.Fatalf("got (%d, %v)", rev, err)
	}
	if runner.subcommand != "copy" {
		t.Errorf("ran %s, want copy", runner.subcommand)
	}
}

func TestSwitch_ReturnsRevision(t *testing.T) {
	// Arrange
	runner := &fakeRunner{out: []byte("U    a.txt\nUpdated to revision 92.\n")}
	s := New(runner, runner, http.DefaultClient)

	// Act
	rev, err := s.Switch(context.Background(), "/wc", "https://h/r/branches/x")

	// Assert
	if err != nil || rev != 92 || runner.subcommand != "switch" {
		t.Errorf("got (%d, %v) from %s", rev, err, runner.subcommand)
	}
}

func TestDeleteRemote_RefusesLocalPaths(t *testing.T) {
	// Arrange
	runner := &fakeRunner{}
	s := New(runner, runner, http.DefaultClient)

	// Act
	_, err := s.DeleteRemote(context.Background(), []string{"C:\\work\\game"}, "Delete game")

	// Assert
	if !errors.Is(err, svn.ErrInvalidRequest) || runner.calls != 0 {
		t.Errorf("err = %v after %d svn calls, want ErrInvalidRequest and none", err, runner.calls)
	}
}

func TestMakeDir_ReturnsRevision(t *testing.T) {
	// Arrange
	runner := &fakeRunner{out: []byte("\nCommitted revision 7.\n")}
	s := New(runner, runner, http.DefaultClient)

	// Act
	rev, err := s.MakeDir(context.Background(), "https://h/r/docs", "Add docs")

	// Assert
	if err != nil || rev != 7 || runner.subcommand != "mkdir" {
		t.Errorf("got (%d, %v) from %s", rev, err, runner.subcommand)
	}
}

// scriptRunner answers each svn subcommand with its own output, for methods that run more than one.
type scriptRunner struct {
	fakeRunner
	outs map[string][]byte
	ran  []string
}

func (r *scriptRunner) Run(_ context.Context, _, subcommand string, args ...string) ([]byte, error) {
	r.ran = append(r.ran, subcommand+" "+strings.Join(args, " "))
	return r.outs[subcommand], nil
}

func TestDiscard_RevertsAndDeletesOnlyWhatSvnCallsNew(t *testing.T) {
	// Arrange
	wc := t.TempDir()
	newFile := filepath.Join(wc, "new.txt")
	keep := filepath.Join(wc, "mod.go")
	for _, f := range []string{newFile, keep} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	status := fmt.Sprintf(`<status><target path=%q><entry path=%q><wc-status item="unversioned" props="none"></wc-status></entry><entry path=%q><wc-status item="modified" props="none"></wc-status></entry></target></status>`, wc, newFile, keep)
	runner := &scriptRunner{outs: map[string][]byte{"status": []byte(status)}}
	s := New(runner, runner, http.DefaultClient)

	// Act
	res, err := s.Discard(context.Background(), wc, []string{wc})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reverted != 1 || res.Removed != 1 {
		t.Errorf("result = %+v", res)
	}
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Errorf("new file still there: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("modified file was deleted: %v", err)
	}
	if len(runner.ran) != 2 || !strings.HasPrefix(runner.ran[1], "revert ") || !strings.HasSuffix(runner.ran[1], keep) {
		t.Errorf("ran %q", runner.ran)
	}
}

func TestReadLocal_RefusesFilesOutsideTheWorkingCopy(t *testing.T) {
	// Arrange
	wc := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	s := New(runner, runner, http.DefaultClient)

	// Act
	_, err := s.ReadLocal(context.Background(), wc, outside)

	// Assert
	if !errors.Is(err, svn.ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
}

func TestReadLocal_ReadsConflictMarkers(t *testing.T) {
	// Arrange
	wc := t.TempDir()
	file := filepath.Join(wc, "a.txt")
	text := "<<<<<<< .mine\nmine\n=======\ntheirs\n>>>>>>> .r5\n"
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	s := New(runner, runner, http.DefaultClient)

	// Act
	got, err := s.ReadLocal(context.Background(), wc, file)

	// Assert
	if err != nil || got.Text != text || got.Binary {
		t.Errorf("got (%+v, %v)", got, err)
	}
}

func TestIgnore_AppendsToTheFoldersList(t *testing.T) {
	// Arrange
	dir := filepath.FromSlash("/wc/build")
	list := fmt.Sprintf(`<properties><target path=%q><property name="svn:ignore">bin
</property></target></properties>`, dir)
	runner := &scriptRunner{outs: map[string][]byte{"proplist": []byte(list)}}
	s := New(runner, runner, http.DefaultClient)

	// Act
	err := s.Ignore(context.Background(), []string{filepath.Join(dir, "obj")})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "propset -- svn:ignore bin\nobj\n " + dir
	if len(runner.ran) != 2 || runner.ran[1] != want {
		t.Errorf("ran %q, want second call %q", runner.ran, want)
	}
}

func TestBlame_JoinsCommitsAndText(t *testing.T) {
	// Arrange
	blame := `<blame><target path="a.go"><entry line-number="1"><commit revision="9"><author>bob</author><date>2026-10-01T10:00:00Z</date></commit></entry></target></blame>`
	runner := &scriptRunner{outs: map[string][]byte{"blame": []byte(blame), "cat": []byte("package a\n")}}
	s := New(runner, runner, http.DefaultClient)

	// Act
	lines, err := s.Blame(context.Background(), "/wc/a.go")

	// Assert
	if err != nil || len(lines) != 1 || lines[0].Text != "package a" || lines[0].Author != "bob" {
		t.Errorf("got (%+v, %v)", lines, err)
	}
}

func TestCreatePatch_WritesTheDiff(t *testing.T) {
	// Arrange
	dest := filepath.Join(t.TempDir(), "fix.patch")
	runner := &scriptRunner{outs: map[string][]byte{"diff": []byte("Index: a.go\n")}}
	s := New(runner, runner, http.DefaultClient)

	// Act
	err := s.CreatePatch(context.Background(), []string{"/wc/a.go"}, dest)

	// Assert
	got, readErr := os.ReadFile(dest)
	if err != nil || readErr != nil || string(got) != "Index: a.go\n" {
		t.Errorf("patch = %q (%v, %v)", got, err, readErr)
	}
}
