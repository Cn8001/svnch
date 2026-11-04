package svn

import (
	"fmt"
	"strconv"
)

type MergeRequest struct {
	Source string `json:"source"` // URL or ^/branches/x
	Target string `json:"target"` // working copy path
	Change int64  `json:"change"` // cherry-pick one revision, negative to undo it, 0 for a full sync merge
	DryRun bool   `json:"dryRun"`
}

// --accept postpone leaves conflicts marked in the working copy, since there is no terminal to resolve them in.
func MergeArgs(req MergeRequest) ([]string, error) {
	if req.Source == "" || req.Target == "" {
		return nil, fmt.Errorf("merge needs a source and a target: %w", ErrInvalidRequest)
	}
	args := []string{"--accept", "postpone"}
	if req.Change != 0 {
		args = append(args, "-c", strconv.FormatInt(req.Change, 10))
	}
	if req.DryRun {
		args = append(args, "--dry-run")
	}
	t, err := targets(req.Source, req.Target)
	if err != nil {
		return nil, err
	}
	return append(args, t...), nil
}
