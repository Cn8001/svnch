package svn

import (
	"context"
	"errors"
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
