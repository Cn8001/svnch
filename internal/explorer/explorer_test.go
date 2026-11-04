package explorer

import (
	"io/fs"
	"os"
	"testing"
	"time"
)

type fakeInfo struct{ dir bool }

func (f fakeInfo) Name() string       { return "x" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return 0 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

func statOf(files map[string]bool) func(string) (fs.FileInfo, error) {
	return func(p string) (fs.FileInfo, error) {
		dir, ok := files[p]
		if !ok {
			return nil, os.ErrNotExist
		}
		return fakeInfo{dir}, nil
	}
}

func TestExeOf_ReadsBackTheWrittenCommand(t *testing.T) {
	// Arrange
	m := New(`C:\Program Files\svnch\svnch.exe`)

	// Act
	got := exeOf(m.command("update", "%1"))

	// Assert
	if got != m.exe {
		t.Errorf("exeOf = %q, want %q", got, m.exe)
	}
}

func TestExeOf_UnquotedCommandIsNotOurs(t *testing.T) {
	// Arrange
	cmd := `notepad.exe "%1"`

	// Act
	got := exeOf(cmd)

	// Assert
	if got != "" {
		t.Errorf("exeOf = %q, want empty", got)
	}
}

func TestParseLaunch_ExplorerEntryOnAFile(t *testing.T) {
	// Arrange
	stat := statOf(map[string]bool{"/wc/a.txt": false})

	// Act
	l := ParseLaunch([]string{"--diff", "/wc/a.txt"}, stat)

	// Assert
	if l.Action != "diff" || l.Path != "/wc/a.txt" || l.IsDir {
		t.Errorf("launch = %+v", l)
	}
}

func TestParseLaunch_UnknownFlagsAndMissingPathsAreSkipped(t *testing.T) {
	// Arrange
	stat := statOf(map[string]bool{"/wc": true})

	// Act
	l := ParseLaunch([]string{"--psn_0_123", "-x", "/gone", "/wc"}, stat)

	// Assert
	if l.Action != "open" || l.Path != "/wc" || !l.IsDir {
		t.Errorf("launch = %+v", l)
	}
}

func TestParseLaunch_PlainStartHasNoPath(t *testing.T) {
	// Arrange
	stat := statOf(nil)

	// Act
	l := ParseLaunch(nil, stat)

	// Assert
	if l.Path != "" {
		t.Errorf("launch = %+v", l)
	}
}
