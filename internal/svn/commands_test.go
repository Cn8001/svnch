package svn

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestArgs_Builders(t *testing.T) {
	cases := map[string]struct {
		build func() ([]string, error)
		want  []string
	}{
		"status":            {func() ([]string, error) { return StatusArgs("/wc") }, []string{"--xml", "--no-ignore", "--", "/wc"}},
		"update":            {func() ([]string, error) { return UpdateArgs("/wc") }, []string{"--", "/wc"}},
		"commit":            {func() ([]string, error) { return CommitArgs(CommitRequest{Paths: []string{"/wc/a"}, Message: "fix"}) }, []string{"-m", "fix", "--", "/wc/a"}},
		"add":               {func() ([]string, error) { return AddArgs("/wc/new/a") }, []string{"--parents", "--", "/wc/new/a"}},
		"delete":            {func() ([]string, error) { return DeleteArgs(false, "/wc/a") }, []string{"--", "/wc/a"}},
		"delete keep local": {func() ([]string, error) { return DeleteArgs(true, "/wc/a") }, []string{"--keep-local", "--", "/wc/a"}},
		"revert":            {func() ([]string, error) { return RevertArgs("/wc/a") }, []string{"--depth", "infinity", "--", "/wc/a"}},
		"move":              {func() ([]string, error) { return MoveArgs("/wc/a", "/wc/b") }, []string{"--", "/wc/a", "/wc/b"}},
		"list":              {func() ([]string, error) { return ListArgs("^/trunk") }, []string{"--xml", "--", "^/trunk"}},
		"diff local":        {func() ([]string, error) { return DiffArgs(DiffRequest{Target: "/wc/a"}) }, []string{"--internal-diff", "--", "/wc/a"}},
		"diff change":       {func() ([]string, error) { return DiffArgs(DiffRequest{Target: "/wc", Change: 7}) }, []string{"--internal-diff", "-c", "7", "--", "/wc"}},
		"log":               {func() ([]string, error) { return LogArgs(LogRequest{Target: "/wc", Limit: 50, Verbose: true}) }, []string{"--xml", "-l", "50", "-v", "--", "/wc"}},
		"merge sync":        {func() ([]string, error) { return MergeArgs(MergeRequest{Source: "^/trunk", Target: "/wc"}) }, []string{"--accept", "postpone", "--", "^/trunk", "/wc"}},
		"merge cherry pick": {func() ([]string, error) {
			return MergeArgs(MergeRequest{Source: "^/trunk", Target: "/wc", Change: 12, DryRun: true})
		}, []string{"--accept", "postpone", "-c", "12", "--dry-run", "--", "^/trunk", "/wc"}},
		"merge undo revision": {func() ([]string, error) {
			return MergeArgs(MergeRequest{Source: "https://h/repo/trunk", Target: "/wc", Change: -12})
		}, []string{"--accept", "postpone", "-c", "-12", "--", "https://h/repo/trunk", "/wc"}},
		"update to revision": {func() ([]string, error) { return UpdateToArgs("/wc", 40) }, []string{"-r", "40", "--", "/wc"}},
		"branch": {func() ([]string, error) {
			return BranchArgs(BranchRequest{From: "^/trunk", To: "^/branches/x", Message: "Create branch x"})
		}, []string{"--parents", "-m", "Create branch x", "--", "^/trunk", "^/branches/x"}},
		"switch":            {func() ([]string, error) { return SwitchArgs("^/branches/x", "/wc") }, []string{"--", "^/branches/x", "/wc"}},
		"changelist":        {func() ([]string, error) { return ChangelistArgs(" ui ", "/wc/a") }, []string{"ui", "--", "/wc/a"}},
		"changelist remove": {func() ([]string, error) { return RemoveChangelistArgs("/wc/a") }, []string{"--remove", "--", "/wc/a"}},
		"remote delete":     {func() ([]string, error) { return RemoteDeleteArgs("Delete x", "https://h/r/x") }, []string{"-m", "Delete x", "--", "https://h/r/x"}},
		"remote move":       {func() ([]string, error) { return RemoteMoveArgs("https://h/r/a", "https://h/r/b", "Rename") }, []string{"-m", "Rename", "--", "https://h/r/a", "https://h/r/b"}},
		"mkdir":             {func() ([]string, error) { return MkdirArgs("^/docs", "Add docs") }, []string{"--parents", "-m", "Add docs", "--", "^/docs"}},
		"cat":               {func() ([]string, error) { return CatArgs("https://h/r/a.txt") }, []string{"--", "https://h/r/a.txt"}},
		"resolve":           {func() ([]string, error) { return ResolveArgs(AcceptTheirsFull, "/wc/a", "/wc/b") }, []string{"--accept", "theirs-full", "--", "/wc/a", "/wc/b"}},
		"export":            {func() ([]string, error) { return ExportArgs("https://h/r/a.png", "/tmp/a.png") }, []string{"--force", "--", "https://h/r/a.png", "/tmp/a.png"}},
		"shelve":            {func() ([]string, error) { return ShelveArgs("wip", "half done") }, []string{"-m", "half done", "--", "wip", "."}},
		"unshelve":          {func() ([]string, error) { return UnshelveArgs("wip") }, []string{"--", "wip"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			args, err := c.build()

			// Assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(args, c.want) {
				t.Errorf("args = %q, want %q", args, c.want)
			}
		})
	}
}

func TestArgs_RejectBadRequests(t *testing.T) {
	cases := map[string]func() ([]string, error){
		"commit without message":         func() ([]string, error) { return CommitArgs(CommitRequest{Paths: []string{"/wc"}, Message: "  "}) },
		"commit without paths":           func() ([]string, error) { return CommitArgs(CommitRequest{Message: "fix"}) },
		"add without paths":              func() ([]string, error) { return AddArgs() },
		"move without target":            func() ([]string, error) { return MoveArgs("/wc/a", "") },
		"diff negative change":           func() ([]string, error) { return DiffArgs(DiffRequest{Target: "/wc", Change: -1}) },
		"log negative limit":             func() ([]string, error) { return LogArgs(LogRequest{Target: "/wc", Limit: -1}) },
		"merge without source":           func() ([]string, error) { return MergeArgs(MergeRequest{Target: "/wc"}) },
		"shelve without name":            func() ([]string, error) { return ShelveArgs("", "") },
		"unshelve without name":          func() ([]string, error) { return UnshelveArgs("") },
		"update to revision 0":           func() ([]string, error) { return UpdateToArgs("/wc", 0) },
		"branch without message":         func() ([]string, error) { return BranchArgs(BranchRequest{From: "^/trunk", To: "^/branches/x"}) },
		"switch without url":             func() ([]string, error) { return SwitchArgs("", "/wc") },
		"changelist flag name":           func() ([]string, error) { return ChangelistArgs("--remove", "/wc/a") },
		"remote delete of a local path":  func() ([]string, error) { return RemoteDeleteArgs("Delete", "/wc/a") },
		"remote delete without message":  func() ([]string, error) { return RemoteDeleteArgs(" ", "https://h/r/x") },
		"move onto itself":               func() ([]string, error) { return RemoteMoveArgs("https://h/r/a", "https://h/r/a", "m") },
		"cat of a local path":            func() ([]string, error) { return CatArgs("/wc/a.txt") },
		"resolve with an unknown choice": func() ([]string, error) { return ResolveArgs("postpone", "/wc/a") },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := build()

			// Assert
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestParseStatus_FlattensTargets(t *testing.T) {
	// Arrange
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<status>
<target path="/wc">
<entry path="/wc/a.txt"><wc-status item="modified" props="none" revision="5"></wc-status></entry>
<entry path="/wc/new.txt"><wc-status item="unversioned" props="none"></wc-status></entry>
<entry path="/wc/b.txt"><wc-status item="added" props="none" revision="-1" copied="true" tree-conflicted="true"></wc-status></entry>
</target>
<target path="/wc/ext">
<entry path="/wc/ext/c.txt"><wc-status item="deleted" props="modified" revision="2"></wc-status></entry>
</target>
</status>`)

	// Act
	entries, err := ParseStatus(data)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []StatusEntry{
		{Path: "/wc/a.txt", Item: "modified", Props: "none", Revision: 5},
		{Path: "/wc/new.txt", Item: "unversioned", Props: "none"},
		{Path: "/wc/b.txt", Item: "added", Props: "none", Revision: -1, Copied: true, TreeConflicted: true},
		{Path: "/wc/ext/c.txt", Item: "deleted", Props: "modified", Revision: 2},
	}
	if !slices.Equal(entries, want) {
		t.Errorf("entries = %+v\nwant      %+v", entries, want)
	}
}

func TestParseStatus_CleanWorkingCopyIsEmptyNotNil(t *testing.T) {
	// Act
	entries, err := ParseStatus([]byte(`<status><target path="/wc"></target></status>`))

	// Assert
	if err != nil || entries == nil || len(entries) != 0 {
		t.Errorf("got (%v, %v), want an empty slice", entries, err)
	}
}

func TestParseUpdate_NothingChanged(t *testing.T) {
	// Act
	rev, err := ParseUpdate([]byte("Updating '.':\nAt revision 31.\n"))

	// Assert
	if err != nil || rev != 31 {
		t.Errorf("got (%d, %v), want (31, nil)", rev, err)
	}
}

func TestParseCommit_NothingToCommit(t *testing.T) {
	// Act
	rev, err := ParseCommit(nil)

	// Assert
	if err != nil || rev != 0 {
		t.Errorf("got (%d, %v), want (0, nil)", rev, err)
	}
}

func TestParseCommit_ReadsRevision(t *testing.T) {
	// Act
	rev, err := ParseCommit([]byte("Sending        a.txt\nTransmitting file data .done\nCommitting transaction...\nCommitted revision 32.\n"))

	// Assert
	if err != nil || rev != 32 {
		t.Errorf("got (%d, %v), want (32, nil)", rev, err)
	}
}

func TestParseList_FilesAndDirs(t *testing.T) {
	// Arrange
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<lists><list path="https://svn.example.org/repo/trunk">
<entry kind="dir"><name>src</name><commit revision="9"><author>bob</author><date>2026-01-02T03:04:05.000000Z</date></commit></entry>
<entry kind="file"><name>README</name><size>120</size><commit revision="4"><author>alice</author><date>2026-01-01T00:00:00.000000Z</date></commit></entry>
</list></lists>`)

	// Act
	entries, err := ParseList(data)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []ListEntry{
		{Name: "src", Kind: "dir", LastCommit: Commit{Revision: 9, Author: "bob", Date: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}},
		{Name: "README", Kind: "file", Size: 120, LastCommit: Commit{Revision: 4, Author: "alice", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}
	if !slices.Equal(entries, want) {
		t.Errorf("entries = %+v\nwant      %+v", entries, want)
	}
}

func TestParseLog_WithChangedPaths(t *testing.T) {
	// Arrange
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<log>
<logentry revision="12">
<author>alice</author>
<date>2026-03-04T05:06:07.000000Z</date>
<paths>
<path action="M" kind="file" prop-mods="false" text-mods="true">/trunk/a.txt</path>
<path action="A" kind="dir" prop-mods="false" text-mods="false">/trunk/lib</path>
</paths>
<msg>Fix the thing
second line</msg>
</logentry>
<logentry revision="11"><author>bob</author><date>2026-03-03T00:00:00.000000Z</date><msg></msg></logentry>
</log>`)

	// Act
	entries, err := ParseLog(data)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	first := entries[0]
	if first.Revision != 12 || first.Author != "alice" || first.Message != "Fix the thing\nsecond line" {
		t.Errorf("first entry = %+v", first)
	}
	wantPaths := []ChangedPath{{Path: "/trunk/a.txt", Action: "M", Kind: "file"}, {Path: "/trunk/lib", Action: "A", Kind: "dir"}}
	if !slices.Equal(first.Paths, wantPaths) {
		t.Errorf("paths = %+v, want %+v", first.Paths, wantPaths)
	}
}

func TestParseLog_EmptyIsNotNil(t *testing.T) {
	// Act
	entries, err := ParseLog([]byte(`<log></log>`))

	// Assert
	if err != nil || entries == nil {
		t.Errorf("got (%v, %v), want an empty slice", entries, err)
	}
}

func TestParseShelfList_KeepsNames(t *testing.T) {
	// Arrange
	out := []byte("wip         version 1, 3 minutes ago, 2 paths changed\n half done\n\nspike       version 2, 1 day ago, 5 paths changed\n")

	// Act
	names := ParseShelfList(out)

	// Assert
	if !slices.Equal(names, []string{"wip", "spike"}) {
		t.Errorf("names = %q", names)
	}
}

func TestParseStatus_ChangelistEntriesCarryTheirList(t *testing.T) {
	// Arrange
	data := []byte(`<status>
<target path="/wc"><entry path="/wc/a"><wc-status item="modified" props="none"></wc-status></entry></target>
<changelist name="ui"><entry path="/wc/b"><wc-status item="modified" props="none"></wc-status></entry></changelist>
</status>`)

	// Act
	entries, err := ParseStatus(data)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 || entries[0].Changelist != "" || entries[1].Path != "/wc/b" || entries[1].Changelist != "ui" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestParseCat_NulMeansBinary(t *testing.T) {
	// Arrange
	text := []byte("hello\nworld\n")
	bin := []byte{0x89, 'P', 'N', 'G', 0, 1, 2}

	// Act
	gotText, gotBin := ParseCat(text), ParseCat(bin)

	// Assert
	if gotText.Binary || gotText.Text != "hello\nworld\n" {
		t.Errorf("text = %+v", gotText)
	}
	if !gotBin.Binary || gotBin.Text != "" || gotBin.Size != len(bin) {
		t.Errorf("binary = %+v", gotBin)
	}
}
