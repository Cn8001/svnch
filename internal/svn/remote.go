package svn

import (
	"bytes"
	"fmt"
	"strings"
)

// The remote commands act on the repository directly and each makes a commit, so they take only urls: a local path
// here would quietly turn a server-side delete into a working copy one.
func requireURLs(urls ...string) error {
	for _, u := range urls {
		if !strings.Contains(u, "://") && !strings.HasPrefix(u, "^/") {
			return fmt.Errorf("%q is not a repository url: %w", u, ErrInvalidRequest)
		}
	}
	return nil
}

func requireMessage(message string) error {
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("a change on the server needs a message: %w", ErrInvalidRequest)
	}
	return nil
}

func remoteArgs(message string, lead []string, urls ...string) ([]string, error) {
	if err := requireMessage(message); err != nil {
		return nil, err
	}
	if err := requireURLs(urls...); err != nil {
		return nil, err
	}
	t, err := targets(urls...)
	if err != nil {
		return nil, err
	}
	return append(append(lead, "-m", message), t...), nil
}

func RemoteDeleteArgs(message string, urls ...string) ([]string, error) {
	return remoteArgs(message, nil, urls...)
}

func RemoteMoveArgs(from, to, message string) ([]string, error) {
	if from == "" || to == "" || from == to {
		return nil, fmt.Errorf("move needs two different urls: %w", ErrInvalidRequest)
	}
	return remoteArgs(message, nil, from, to)
}

func MkdirArgs(url, message string) ([]string, error) {
	return remoteArgs(message, []string{"--parents"}, url)
}

func CatArgs(url string) ([]string, error) {
	if err := requireURLs(url); err != nil {
		return nil, err
	}
	return targets(url)
}

// ExportArgs saves one file or folder from the repository without making a working copy; --force because the save
// dialog has already asked about replacing an existing file.
func ExportArgs(url, dest string) ([]string, error) {
	if dest == "" {
		return nil, fmt.Errorf("export needs a destination: %w", ErrInvalidRequest)
	}
	if err := requireURLs(url); err != nil {
		return nil, err
	}
	t, err := targets(url, dest)
	if err != nil {
		return nil, err
	}
	return append([]string{"--force"}, t...), nil
}

type FileContent struct {
	Text   string `json:"text"` // empty for a binary file
	Binary bool   `json:"binary"`
	Size   int    `json:"size"`
}

// ParseCat treats a NUL byte in the first 8 KB as binary, the same test svn and git use to decide a file is not text.
func ParseCat(out []byte) FileContent {
	head := out[:min(len(out), 8<<10)]
	if bytes.IndexByte(head, 0) >= 0 {
		return FileContent{Binary: true, Size: len(out)}
	}
	return FileContent{Text: string(out), Size: len(out)}
}
