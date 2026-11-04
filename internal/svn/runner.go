package svn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrSvnNotFound    = errors.New("svn executable not found")
	ErrNotWorkingCopy = errors.New("not a working copy")
	ErrInvalidRequest = errors.New("invalid request")
	ErrAuthFailed     = errors.New("authentication failed")
	ErrCertUntrusted  = errors.New("server certificate not trusted")
	ErrNeedsCleanup   = errors.New("working copy locked by an interrupted operation")
)

type Runner interface {
	Run(ctx context.Context, dir, subcommand string, args ...string) ([]byte, error) // dir empty means the process directory
}

type ExecRunner struct {
	binary string
	mucc   string // svnmucc, which ships next to svn; only it can set a property on a url with a message

	mu       sync.Mutex
	username string
	password string
	progress func(Progress) // told how far each command has come, while it runs
}

func NewExecRunner(binary string) *ExecRunner {
	return &ExecRunner{binary: binary, mucc: sibling(binary, "svnmucc")}
}

// sibling is the tool name next to the svn binary when it is there, so svnch uses svn's own copy of it, and the bare
// name for PATH otherwise.
func sibling(binary, name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if dir := filepath.Dir(binary); dir != "." && dir != "" {
		if p := filepath.Join(dir, name); fileExists(p) {
			return p
		}
	}
	return name
}

// SetCredentials keeps a login for the rest of the session; svn also saves it when its password store allows, but
// without a keyring many systems refuse to store it, so every later command passes it again. Empty clears it.
func (r *ExecRunner) SetCredentials(username, password string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.username, r.password = username, password
}

// OnProgress hands every command's progress to report, which the app passes on to the window.
func (r *ExecRunner) OnProgress(report func(Progress)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress = report
}

func (r *ExecRunner) Credentials() (username, password string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.username, r.password
}

func (r *ExecRunner) Run(ctx context.Context, dir, subcommand string, args ...string) ([]byte, error) {
	return r.exec(ctx, r.binary, dir, subcommand, []string{subcommand}, args)
}

// Mucc runs svnmucc with its options and then its actions, svnmucc's own order.
func (r *ExecRunner) Mucc(ctx context.Context, args ...string) ([]byte, error) {
	return r.exec(ctx, r.mucc, "", "svnmucc", nil, args)
}

// exec puts lead, --non-interactive, the session login and args together; the login goes in the middle because svn
// wants options after the subcommand and svnmucc before its actions, and both read --password-from-stdin.
func (r *ExecRunner) exec(ctx context.Context, binary, dir, what string, lead, args []string) ([]byte, error) {
	full := append(lead, "--non-interactive")
	r.mu.Lock()
	username, password, report := r.username, r.password, r.progress
	r.mu.Unlock()
	if username != "" {
		// The password goes through stdin so it never shows in the process list.
		full = append(full, "--username", username, "--password-from-stdin")
	}
	full = append(full, args...)

	cmd := command(ctx, binary, full...)
	if username != "" {
		// No newline: svn on Windows ends the line at \r\n only, so a trailing \n became part of the password. Without
		// one, svn reads to the end of stdin on every system.
		cmd.Stdin = strings.NewReader(password)
	}
	cmd.Dir = dir
	// LC_ALL=C keeps stderr in English so codes and messages read the same everywhere; svn 1.14 only knows its
	// shelving commands (x-shelve and the rest) when SVN_EXPERIMENTAL_COMMANDS names the shelf3 version.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "SVN_EXPERIMENTAL_COMMANDS=shelf3")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var stdout interface{ Bytes() []byte }
	if report != nil {
		w := newProgressWriter(what, report)
		defer w.done()
		cmd.Stdout = w
		stdout = &w.out
	} else {
		buf := &bytes.Buffer{}
		cmd.Stdout = buf
		stdout = buf
	}

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("run %s: %w", binary, ErrSvnNotFound)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("svn %s: %w", what, ctxErr)
		}
		msg := strings.TrimSpace(stderr.String())
		return nil, &CommandError{
			Subcommand: what,
			Code:       errorCode(msg),
			Causes:     errorCodes(msg),
			Stderr:     msg,
			Err:        err,
		}
	}
	return stdout.Bytes(), nil
}

type CommandError struct {
	Subcommand string
	Code       string   // svn error code such as E155007, empty when stderr carries none
	Causes     []string // every E code in stderr; svn chains them, and the reason is often not the first
	Stderr     string
	Err        error
}

func (e *CommandError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("svn %s: %v", e.Subcommand, e.Err)
	}
	return fmt.Sprintf("svn %s: %s", e.Subcommand, e.Stderr)
}

func (e *CommandError) Unwrap() error { return e.Err }

// Is lets callers match svn error codes with errors.Is against this package's sentinels. It looks at the whole chain:
// a password-protected http server fails with E170013 "unable to connect" first and the E215004 that says why after.
func (e *CommandError) Is(target error) bool {
	codes := e.Causes
	if len(codes) == 0 {
		codes = []string{e.Code}
	}
	for _, c := range codes {
		if sentinel(c) == target {
			return true
		}
	}
	return false
}

func sentinel(code string) error {
	switch code {
	case "E155007":
		return ErrNotWorkingCopy
	case "E215004", "E170001":
		return ErrAuthFailed
	case "E230001":
		return ErrCertUntrusted
	case "E155004", "E155037":
		return ErrNeedsCleanup
	}
	return nil
}

// errorCode returns the first E code, because svn prints warnings (W codes) before the error that failed the command.
func errorCode(stderr string) string {
	if codes := errorCodes(stderr); len(codes) > 0 {
		return codes[0]
	}
	return ""
}

func errorCodes(stderr string) []string {
	var codes []string
	for line := range strings.Lines(stderr) {
		rest, ok := strings.CutPrefix(line, "svn: ")
		if !ok || len(rest) < 8 || rest[0] != 'E' || rest[7] != ':' {
			continue
		}
		if isDigits(rest[1:7]) {
			codes = append(codes, rest[:7])
		}
	}
	return codes
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// pegSafe stops svn reading the text after an @ in a path as a peg revision.
func pegSafe(target string) string {
	if strings.Contains(target, "@") {
		return target + "@"
	}
	return target
}

// targets ends option parsing first, so a path that starts with a dash is never read as a flag.
func targets(paths ...string) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no path given: %w", ErrInvalidRequest)
	}
	out := []string{"--"}
	for _, p := range paths {
		if p == "" {
			return nil, fmt.Errorf("empty path: %w", ErrInvalidRequest)
		}
		out = append(out, pegSafe(p))
	}
	return out, nil
}

// lastRevision reads N from the last "<prefix>N." line, since checkout, update and commit have no --xml output.
func lastRevision(out []byte, prefixes ...string) (int64, bool, error) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range slices.Backward(lines) {
		line = strings.TrimSpace(line)
		for _, p := range prefixes {
			rest, ok := strings.CutPrefix(line, p)
			if !ok {
				continue
			}
			rev, err := strconv.ParseInt(strings.TrimSuffix(rest, "."), 10, 64)
			if err != nil {
				return 0, false, fmt.Errorf("parse revision %q: %w", line, err)
			}
			return rev, true, nil
		}
	}
	return 0, false, nil
}
