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
	subcommand string
	args       []string
}

func (f *fakeRunner) Run(_ context.Context, subcommand string, args ...string) ([]byte, error) {
	f.calls++
	f.subcommand, f.args = subcommand, args
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
