package svn

import (
	"fmt"
	"strings"
)

// Shelving is still experimental in svn 1.14, so its commands carry the x- prefix.
const (
	ShelveCommand    = "x-shelve"
	UnshelveCommand  = "x-unshelve"
	ShelfListCommand = "x-shelf-list"
)

func ShelveArgs(name, message string) ([]string, error) {
	if name == "" {
		return nil, fmt.Errorf("shelve needs a name: %w", ErrInvalidRequest)
	}
	var args []string
	if message != "" {
		args = append(args, "-m", message)
	}
	return append(args, "--", name, "."), nil
}

func UnshelveArgs(name string) ([]string, error) {
	if name == "" {
		return nil, fmt.Errorf("unshelve needs a name: %w", ErrInvalidRequest)
	}
	return []string{"--", name}, nil
}

// ParseShelfList keeps only the names; the rest of each line is a human summary whose wording changes between versions.
func ParseShelfList(out []byte) []string {
	names := []string{}
	for line := range strings.Lines(string(out)) {
		if fields := strings.Fields(line); len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	return names
}
