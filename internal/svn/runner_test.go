package svn

import (
	"context"
	"errors"
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

func TestExecRunner_MissingBinary(t *testing.T) {
	// Arrange
	r := NewExecRunner("svnch-no-such-binary")

	// Act
	_, err := r.Run(context.Background(), "info")

	// Assert
	if !errors.Is(err, ErrSvnNotFound) {
		t.Errorf("err = %v, want ErrSvnNotFound", err)
	}
}
