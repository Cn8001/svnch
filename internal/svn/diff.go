package svn

import (
	"fmt"
	"strconv"
)

type DiffRequest struct {
	Target string `json:"target"`
	Change int64  `json:"change"` // 0 diffs local edits against BASE, N shows what revision N changed
}

// --internal-diff keeps the output a unified diff even when the user's svn config names an external diff tool.
func DiffArgs(req DiffRequest) ([]string, error) {
	if req.Change < 0 {
		return nil, fmt.Errorf("diff change %d: %w", req.Change, ErrInvalidRequest)
	}
	args := []string{"--internal-diff"}
	if req.Change > 0 {
		args = append(args, "-c", strconv.FormatInt(req.Change, 10))
	}
	t, err := targets(req.Target)
	if err != nil {
		return nil, err
	}
	return append(args, t...), nil
}
