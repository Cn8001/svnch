package svn

import (
	"fmt"
	"slices"
)

// The ways svn can settle a conflict without asking: the whole file from one side, only the clashing parts from one
// side, or working, which keeps the file as it is now, after it was edited by hand.
const (
	AcceptWorking        = "working"
	AcceptMineFull       = "mine-full"
	AcceptTheirsFull     = "theirs-full"
	AcceptMineConflict   = "mine-conflict"
	AcceptTheirsConflict = "theirs-conflict"
)

func ResolveArgs(accept string, paths ...string) ([]string, error) {
	if !slices.Contains([]string{AcceptWorking, AcceptMineFull, AcceptTheirsFull, AcceptMineConflict, AcceptTheirsConflict}, accept) {
		return nil, fmt.Errorf("resolve with %q: %w", accept, ErrInvalidRequest)
	}
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	return append([]string{"--accept", accept}, t...), nil
}

// Inside tells whether path is in the working copy and is not the working copy itself; svnch reads and deletes local
// files only there.
func Inside(wc, path string) bool {
	return inside(wc, path)
}
