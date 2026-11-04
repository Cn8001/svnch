package svn

import (
	"fmt"
	"strings"
)

// BranchRequest copies From to To on the server; in svn a branch is that copy, made in one commit without
// transferring any files.
type BranchRequest struct {
	From    string `json:"from"` // the url being branched, usually the working copy's own
	To      string `json:"to"`   // e.g. ^/branches/feature
	Message string `json:"message"`
}

// BranchArgs passes --parents so a repository without a branches folder yet gets one.
func BranchArgs(req BranchRequest) ([]string, error) {
	if req.From == "" || req.To == "" || strings.TrimSpace(req.Message) == "" {
		return nil, fmt.Errorf("branch needs a source, a target and a message: %w", ErrInvalidRequest)
	}
	t, err := targets(req.From, req.To)
	if err != nil {
		return nil, err
	}
	return append([]string{"--parents", "-m", req.Message}, t...), nil
}

// SwitchArgs points the working copy at another branch; svn prints the same revision lines as update.
func SwitchArgs(url, wc string) ([]string, error) {
	if url == "" || wc == "" {
		return nil, fmt.Errorf("switch needs a url and a working copy: %w", ErrInvalidRequest)
	}
	return targets(url, wc)
}
