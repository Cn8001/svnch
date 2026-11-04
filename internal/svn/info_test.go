package svn

import (
	"slices"
	"testing"
	"time"
)

const infoXML = `<?xml version="1.0" encoding="UTF-8"?>
<info>
<entry kind="dir" path="." revision="1234">
<url>https://svn.example.org/repo/trunk</url>
<relative-url>^/trunk</relative-url>
<repository>
<root>https://svn.example.org/repo</root>
<uuid>13f79535-47bb-0310-9956-ffa450edef68</uuid>
</repository>
<wc-info>
<wcroot-abspath>/work/trunk</wcroot-abspath>
<schedule>normal</schedule>
<depth>infinity</depth>
</wc-info>
<commit revision="1230">
<author>alice</author>
<date>2026-10-01T09:30:00.123456Z</date>
</commit>
</entry>
</info>`

func TestParseInfo_WorkingCopy(t *testing.T) {
	// Arrange
	data := []byte(infoXML)

	// Act
	info, err := ParseInfo(data)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Info{
		Path:            ".",
		Kind:            "dir",
		Revision:        1234,
		URL:             "https://svn.example.org/repo/trunk",
		RelativeURL:     "^/trunk",
		RepositoryRoot:  "https://svn.example.org/repo",
		RepositoryUUID:  "13f79535-47bb-0310-9956-ffa450edef68",
		WorkingCopyRoot: "/work/trunk",
		Schedule:        "normal",
		Depth:           "infinity",
		LastCommit: Commit{
			Revision: 1230,
			Author:   "alice",
			Date:     time.Date(2026, 10, 1, 9, 30, 0, 123456000, time.UTC),
		},
	}
	if info != want {
		t.Errorf("info = %+v\nwant   %+v", info, want)
	}
}

func TestParseInfo_NoEntry(t *testing.T) {
	// Arrange
	data := []byte(`<info></info>`)

	// Act
	_, err := ParseInfo(data)

	// Assert
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestInfoArgs_EscapesAtSign(t *testing.T) {
	// Act
	args := InfoArgs("/work/me@home")

	// Assert
	want := []string{"--xml", "--", "/work/me@home@"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
}
