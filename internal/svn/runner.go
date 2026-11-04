package svn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	ErrSvnNotFound    = errors.New("svn executable not found")
	ErrNotWorkingCopy = errors.New("not a working copy")
)

type Runner interface {
	Run(ctx context.Context, subcommand string, args ...string) ([]byte, error)
}

type ExecRunner struct {
	binary string
}

func NewExecRunner(binary string) *ExecRunner {
	return &ExecRunner{binary: binary}
}

func (r *ExecRunner) Run(ctx context.Context, subcommand string, args ...string) ([]byte, error) {
	full := append([]string{subcommand, "--non-interactive"}, args...)

	cmd := exec.CommandContext(ctx, r.binary, full...)
	// Keeps stderr in English so error codes and messages read the same on every machine.
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("run %s: %w", r.binary, ErrSvnNotFound)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("svn %s: %w", subcommand, ctxErr)
		}
		msg := strings.TrimSpace(stderr.String())
		return nil, &CommandError{
			Subcommand: subcommand,
			Code:       errorCode(msg),
			Stderr:     msg,
			Err:        err,
		}
	}
	return stdout.Bytes(), nil
}

type CommandError struct {
	Subcommand string
	Code       string // svn error code such as E155007, empty when stderr carries none
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

// Is lets callers match svn error codes with errors.Is against this package's sentinels.
func (e *CommandError) Is(target error) bool {
	switch e.Code {
	case "E155007":
		return target == ErrNotWorkingCopy
	}
	return false
}

// errorCode returns the first E code, because svn prints warnings (W codes) before the error that failed the command.
func errorCode(stderr string) string {
	for line := range strings.Lines(stderr) {
		rest, ok := strings.CutPrefix(line, "svn: ")
		if !ok || len(rest) < 8 || rest[0] != 'E' || rest[7] != ':' {
			continue
		}
		if isDigits(rest[1:7]) {
			return rest[:7]
		}
	}
	return ""
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
