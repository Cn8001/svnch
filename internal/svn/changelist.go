package svn

import (
	"fmt"
	"strings"
)

// ChangelistArgs files paths under name; svn only accepts versioned files, never folders or new files.
func ChangelistArgs(name string, paths ...string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "-") {
		return nil, fmt.Errorf("changelist needs a name: %w", ErrInvalidRequest)
	}
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	return append([]string{name}, t...), nil
}

func RemoveChangelistArgs(paths ...string) ([]string, error) {
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	return append([]string{"--remove"}, t...), nil
}
