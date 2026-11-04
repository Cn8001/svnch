package svn

import (
	"errors"
	"fmt"
)

func UpdateArgs(paths ...string) ([]string, error) {
	return targets(paths...)
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
