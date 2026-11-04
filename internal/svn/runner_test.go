package svn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestErrorCode_SkipsWarningsForFirstError(t *testing.T) {
	// Arrange
	stderr := "svn: warning: W155010: The node '/x' was not found.\nsvn: E200009: Could not display info for all targets"

	// Act
	code := errorCode(stderr)

	// Assert
	if code != "E200009" {
		t.Errorf("code = %q, want E200009", code)
	}
}

func TestErrorCode_NoCode(t *testing.T) {
	// Act
	code := errorCode("something went wrong")

	// Assert
	if code != "" {
		t.Errorf("code = %q, want empty", code)
	}
}

func TestCommandError_MatchesNotWorkingCopy(t *testing.T) {
	// Arrange
	err := error(&CommandError{Subcommand: "info", Code: "E155007", Stderr: "svn: E155007: '/tmp' is not a working copy"})

	// Act
	matched := errors.Is(err, ErrNotWorkingCopy)

	// Assert
	if !matched {
		t.Error("expected errors.Is to match ErrNotWorkingCopy")
	}
}

func TestTargets_EndsOptionsAndEscapesAtSign(t *testing.T) {
	// Act
	args, err := targets("-odd", "a@b")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"--", "-odd", "a@b@"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
}

func TestTargets_RejectsEmpty(t *testing.T) {
	for name, paths := range map[string][]string{"none": nil, "blank": {"a", ""}} {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := targets(paths...)

			// Assert
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestLastRevision_TakesLastMatchingLine(t *testing.T) {
	// Arrange
	out := []byte("Updating 'a':\nAt revision 5.\nUpdating 'b':\nUpdated to revision 9.\n")

	// Act
	rev, ok, err := lastRevision(out, "Updated to revision ", "At revision ")

	// Assert
	if err != nil || !ok || rev != 9 {
		t.Errorf("got (%d, %v, %v), want (9, true, nil)", rev, ok, err)
	}
}

func TestLastRevision_NoMatch(t *testing.T) {
	// Act
	_, ok, err := lastRevision([]byte("nothing here"), "Committed revision ")

	// Assert
	if ok || err != nil {
		t.Errorf("got (%v, %v), want (false, nil)", ok, err)
	}
}

func TestExecRunner_MissingBinary(t *testing.T) {
	// Arrange
	r := NewExecRunner("svnch-no-such-binary")

	// Act
	_, err := r.Run(context.Background(), "", "info")

	// Assert
	if !errors.Is(err, ErrSvnNotFound) {
		t.Errorf("err = %v, want ErrSvnNotFound", err)
	}
}

func TestCommandError_AuthLaterInTheChainIsAuth(t *testing.T) {
	// Arrange
	stderr := "svn: E170013: Unable to connect to a repository at URL 'https://svn.example.org/repo'\n" +
		"svn: E215004: No more credentials or we tried too many times.\nAuthentication failed\n"
	err := error(&CommandError{Subcommand: "list", Code: errorCode(stderr), Causes: errorCodes(stderr), Stderr: stderr})

	// Act
	auth := errors.Is(err, ErrAuthFailed)

	// Assert
	if !auth {
		t.Errorf("errors.Is(ErrAuthFailed) = false for %q", stderr)
	}
	if errors.Is(err, ErrNotWorkingCopy) {
		t.Errorf("matched ErrNotWorkingCopy too")
	}
}

func TestCommandError_CertLaterInTheChainIsCert(t *testing.T) {
	// Arrange
	stderr := "svn: E170013: Unable to connect to a repository at URL 'https://svn.example.org/repo'\n" +
		"svn: E230001: Server SSL certificate verification failed: issuer is not trusted\n"
	err := error(&CommandError{Subcommand: "list", Code: errorCode(stderr), Causes: errorCodes(stderr), Stderr: stderr})

	// Act
	cert := errors.Is(err, ErrCertUntrusted)

	// Assert
	if !cert {
		t.Errorf("errors.Is(ErrCertUntrusted) = false for %q", stderr)
	}
}

func TestExecRunner_PasswordGoesToStdinWithoutNewline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as svn")
	}
	// Arrange
	dir := t.TempDir()
	out := filepath.Join(dir, "stdin")
	fake := filepath.Join(dir, "svn")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\ncat > \"$SVNCH_TEST_STDIN\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SVNCH_TEST_STDIN", out)
	r := NewExecRunner(fake)
	r.SetCredentials("me", "secret")

	// Act
	_, err := r.Run(context.Background(), "", "info", "--", "https://h/r")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "secret" {
		t.Errorf("svn read %q from stdin, want %q", got, "secret")
	}
}

func TestCommandError_LockedWorkingCopyNeedsCleanup(t *testing.T) {
	// Arrange
	stderr := "svn: E155004: Run 'svn cleanup' to remove locks (type 'svn help cleanup' for details)\n" +
		"svn: E155004: Working copy 'C:\\wc\\Assets' locked.\n"
	err := error(&CommandError{Subcommand: "update", Code: errorCode(stderr), Causes: errorCodes(stderr), Stderr: stderr})

	// Act
	needs := errors.Is(err, ErrNeedsCleanup)

	// Assert
	if !needs {
		t.Errorf("errors.Is(ErrNeedsCleanup) = false for %q", stderr)
	}
}

func TestExecRunner_ReportsProgressWhileKeepingOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as svn")
	}
	// Arrange
	dir := t.TempDir()
	fake := filepath.Join(dir, "svn")
	script := "#!/bin/sh\nprintf 'A    a.txt\\nA    b.txt\\nUpdated to revision 2.\\n'\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := NewExecRunner(fake)
	var last Progress
	r.OnProgress(func(p Progress) { last = p })

	// Act
	out, err := r.Run(context.Background(), "", "update", "--", "/wc")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev, _ := ParseUpdate(out); rev != 2 {
		t.Errorf("output = %q", out)
	}
	if last.Items != 2 || last.Item != "b.txt" || last.Command != "update" {
		t.Errorf("progress = %+v", last)
	}
}
