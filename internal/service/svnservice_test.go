package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Cn8001/svnch/internal/svn"
)

type fakeRunner struct {
	out        []byte
	err        error
	calls      int
	dir        string
	subcommand string
	args       []string
}

func (f *fakeRunner) Run(_ context.Context, dir, subcommand string, args ...string) ([]byte, error) {
	f.calls++
	f.dir, f.subcommand, f.args = dir, subcommand, args
	return f.out, f.err
}

func TestCheckout_ReturnsCheckedOutRevision(t *testing.T) {
	// Arrange
	runner := &fakeRunner{out: []byte("A    wc/a.txt\nChecked out revision 88.\n")}
	s := New(runner)

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
	s := New(runner)

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
	s := New(runner)

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
	s := New(runner)

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
	s := New(runner)

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
	s := New(runner)

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
	s := New(runner)

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
	s := New(runner)

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
