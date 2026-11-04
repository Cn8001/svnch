package svn

import (
	"errors"
	"slices"
	"testing"
)

func TestMaintenanceArgs(t *testing.T) {
	cases := map[string]struct {
		build func() ([]string, error)
		want  []string
	}{
		"relocate":     {func() ([]string, error) { return RelocateArgs("https://new/r", "/wc") }, []string{"--", "https://new/r", "/wc"}},
		"lock":         {func() ([]string, error) { return LockArgs("editing", false, "/wc/a.png") }, []string{"-m", "editing", "--", "/wc/a.png"}},
		"steal lock":   {func() ([]string, error) { return LockArgs("", true, "/wc/a.png") }, []string{"--force", "--", "/wc/a.png"}},
		"unlock":       {func() ([]string, error) { return UnlockArgs(false, "/wc/a.png") }, []string{"--", "/wc/a.png"}},
		"local export": {func() ([]string, error) { return LocalExportArgs("/wc", "/out/wc") }, []string{"--force", "--", "/wc", "/out/wc"}},
		"import":       {func() ([]string, error) { return ImportArgs("/new", "https://h/r/new", "Import") }, []string{"-m", "Import", "--", "/new", "https://h/r/new"}},
		"patch":        {func() ([]string, error) { return PatchArgs("/tmp/fix.patch", "/wc") }, []string{"--", "/tmp/fix.patch", "/wc"}},
		"blame":        {func() ([]string, error) { return BlameArgs("/wc/a.go") }, []string{"--xml", "--", "/wc/a.go"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			args, err := c.build()

			// Assert
			if err != nil || !slices.Equal(args, c.want) {
				t.Errorf("args = %q (%v), want %q", args, err, c.want)
			}
		})
	}
}

func TestRelocateArgs_NeedsAUrl(t *testing.T) {
	// Act
	_, err := RelocateArgs("/not/a/url", "/wc")

	// Assert
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
}

func TestParseBlame_PairsCommitsWithLines(t *testing.T) {
	// Arrange
	data := []byte(`<blame><target path="a.go">
<entry line-number="1"><commit revision="5"><author>alice</author><date>2026-10-01T10:00:00Z</date></commit></entry>
<entry line-number="2"></entry>
</target></blame>`)

	// Act
	lines, err := ParseBlame(data, "package a\r\nfunc x() {}\r\n")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 || lines[0].Text != "package a" || lines[0].Author != "alice" || lines[1].Revision != 0 || lines[1].Text != "func x() {}" {
		t.Errorf("lines = %+v", lines)
	}
}

func TestFileURL_WindowsAndUnix(t *testing.T) {
	// Act
	unix := FileURL("/srv/repo")

	// Assert
	if unix != "file:///srv/repo" {
		t.Errorf("unix = %q", unix)
	}
}

func TestParseStatus_LockedItem(t *testing.T) {
	// Arrange
	data := []byte(`<status><target path="/wc"><entry path="/wc/a.png"><wc-status item="normal" props="none"><lock><token>opaquelocktoken:1</token></lock></wc-status></entry></target></status>`)

	// Act
	entries, err := ParseStatus(data)

	// Assert
	if err != nil || len(entries) != 1 || !entries[0].Locked {
		t.Errorf("entries = %+v, err = %v", entries, err)
	}
}
