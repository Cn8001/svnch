package svn

import (
	"fmt"
	"strings"
)

type CommitRequest struct {
	Paths   []string `json:"paths"`
	Message string   `json:"message"`
}

func CommitArgs(req CommitRequest) ([]string, error) {
	if strings.TrimSpace(req.Message) == "" {
		return nil, fmt.Errorf("commit needs a message: %w", ErrInvalidRequest)
	}
	t, err := targets(req.Paths...)
	if err != nil {
		return nil, err
	}
	return append([]string{"-m", req.Message}, t...), nil
}

// ParseCommit returns 0 when there was nothing to commit, because svn then prints nothing.
func ParseCommit(out []byte) (int64, error) {
	rev, _, err := lastRevision(out, "Committed revision ")
	if err != nil {
		return 0, fmt.Errorf("parse commit: %w", err)
	}
	return rev, nil
}
