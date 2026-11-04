package svn

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func CleanupArgs(wc string) ([]string, error) { return targets(wc) }

func UpgradeArgs(wc string) ([]string, error) { return targets(wc) }

// RelocateArgs points the working copy at the same repository under a new address, after the server moved; svn
// checks that the repository UUID matches.
func RelocateArgs(to, wc string) ([]string, error) {
	if err := requireURLs(to); err != nil {
		return nil, err
	}
	return targets(to, wc)
}

func LockArgs(message string, steal bool, paths ...string) ([]string, error) {
	var args []string
	if strings.TrimSpace(message) != "" {
		args = append(args, "-m", message)
	}
	if steal {
		args = append(args, "--force")
	}
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	return append(args, t...), nil
}

func UnlockArgs(force bool, paths ...string) ([]string, error) {
	var args []string
	if force {
		args = append(args, "--force")
	}
	t, err := targets(paths...)
	if err != nil {
		return nil, err
	}
	return append(args, t...), nil
}

// LocalExportArgs copies a working copy's files, local edits included, to dest without the .svn folder.
func LocalExportArgs(path, dest string) ([]string, error) {
	if dest == "" {
		return nil, fmt.Errorf("export needs a destination: %w", ErrInvalidRequest)
	}
	t, err := targets(path, dest)
	if err != nil {
		return nil, err
	}
	return append([]string{"--force"}, t...), nil
}

// ImportArgs adds an unversioned folder's contents to the repository at url in one commit.
func ImportArgs(path, url, message string) ([]string, error) {
	if err := requireMessage(message); err != nil {
		return nil, err
	}
	if err := requireURLs(url); err != nil {
		return nil, err
	}
	t, err := targets(path, url)
	if err != nil {
		return nil, err
	}
	return append([]string{"-m", message}, t...), nil
}

func PatchArgs(patchFile, wc string) ([]string, error) { return targets(patchFile, wc) }

// BlameLine is one line of a file with the commit that last changed it.
type BlameLine struct {
	Line     int    `json:"line"`
	Text     string `json:"text"`
	Revision int64  `json:"revision"` // 0 for a line changed locally and not committed
	Author   string `json:"author"`
	Date     string `json:"date"`
}

func BlameArgs(target string) ([]string, error) {
	t, err := targets(target)
	if err != nil {
		return nil, err
	}
	return append([]string{"--xml"}, t...), nil
}

// ParseBlame pairs svn blame --xml, which has the commits but not the text, with the file's lines.
func ParseBlame(data []byte, text string) ([]BlameLine, error) {
	var doc struct {
		Entries []struct {
			Line   int `xml:"line-number,attr"`
			Commit struct {
				Revision int64  `xml:"revision,attr"`
				Author   string `xml:"author"`
				Date     string `xml:"date"`
			} `xml:"commit"`
		} `xml:"target>entry"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn blame: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]BlameLine, 0, len(doc.Entries))
	for _, e := range doc.Entries {
		l := BlameLine{Line: e.Line, Revision: e.Commit.Revision, Author: e.Commit.Author, Date: e.Commit.Date}
		if e.Line >= 1 && e.Line <= len(lines) {
			l.Text = lines[e.Line-1]
		}
		out = append(out, l)
	}
	return out, nil
}

// CreateRepository runs svnadmin, which ships next to svn, to make a repository on disk and returns its file:// url.
func CreateRepository(ctx context.Context, svnBinary, path string) (string, error) {
	admin := "svnadmin"
	if dir := filepath.Dir(svnBinary); dir != "." && dir != "" {
		name := "svnadmin"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			admin = filepath.Join(dir, name)
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("create repository: %w", err)
	}
	if out, err := command(ctx, admin, "create", "--", abs).CombinedOutput(); err != nil {
		return "", fmt.Errorf("create repository %s: %s: %w", abs, strings.TrimSpace(string(out)), err)
	}
	return FileURL(abs), nil
}

// FileURL turns a local path into the file:// url svn wants, C:\repo becoming file:///C:/repo.
func FileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

// BlameTextArgs reads the committed text a blame describes; unlike CatArgs it takes a working copy path too, where cat
// gives the base, the version blame is about.
func BlameTextArgs(target string) ([]string, error) { return targets(target) }
