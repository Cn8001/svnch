package svn

import (
	"errors"
	"fmt"
	"strconv"
)

func UpdateArgs(paths ...string) ([]string, error) {
	return targets(paths...)
}

// UpdateToArgs moves path back (or forward) to one revision; nothing on the server changes.
func UpdateToArgs(path string, revision int64) ([]string, error) {
	if revision <= 0 {
		return nil, fmt.Errorf("update to revision %d: %w", revision, ErrInvalidRequest)
	}
	t, err := targets(path)
	if err != nil {
		return nil, err
	}
	return append([]string{"-r", strconv.FormatInt(revision, 10)}, t...), nil
}

// ParseUpdate reads the final revision; svn prints "At revision N." when nothing changed.
func ParseUpdate(out []byte) (int64, error) {
	rev, ok, err := lastRevision(out, "Updated to revision ", "At revision ")
	if err != nil {
		return 0, fmt.Errorf("parse update: %w", err)
	}
	if !ok {
		return 0, errors.New("parse update: no revision line in output")
	}
	return rev, nil
}
