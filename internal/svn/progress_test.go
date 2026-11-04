package svn

import (
	"testing"
	"time"
)

func TestItemLine_UpdateCheckoutAndCommitLines(t *testing.T) {
	cases := map[string]struct {
		line string
		item string
		ok   bool
	}{
		"added":            {"A    wc/a.txt", "wc/a.txt", true},
		"props only":       {" U   wc/dir", "wc/dir", true},
		"merged":           {"G    wc/b.go", "wc/b.go", true},
		"conflict":         {"C    wc/c.go", "wc/c.go", true},
		"commit send":      {"Sending        README.txt", "README.txt", true},
		"commit add":       {"Adding         src/main.go", "src/main.go", true},
		"summary":          {"Updated to revision 12.", "", false},
		"updating header":  {"Updating 'wc':", "", false},
		"transmitting":     {"Transmitting file data .done", "", false},
		"blank columns":    {"     path", "", false},
		"checked out line": {"Checked out revision 3.", "", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			item, ok := itemLine(c.line)

			// Assert
			if ok != c.ok || item != c.item {
				t.Errorf("itemLine(%q) = (%q, %v), want (%q, %v)", c.line, item, ok, c.item, c.ok)
			}
		})
	}
}

func TestProgressWriter_CountsAcrossSplitWritesAndKeepsOutput(t *testing.T) {
	// Arrange
	var got []Progress
	w := newProgressWriter("update", func(p Progress) { got = append(got, p) })
	clock := time.Unix(0, 0)
	w.now = func() time.Time { return clock }
	text := "Updating '.':\r\nA    a.txt\r\nU    b.t"

	// Act
	w.Write([]byte(text))
	clock = clock.Add(time.Second)
	w.Write([]byte("xt\nUpdated to revision 4.\n"))
	w.done()

	// Assert
	if w.out.String() != text+"xt\nUpdated to revision 4.\n" {
		t.Errorf("output = %q", w.out.String())
	}
	last := got[len(got)-1]
	if last.Items != 2 || last.Item != "b.txt" || last.Command != "update" {
		t.Errorf("last progress = %+v", last)
	}
}

func TestParseIncoming_CountsNewerItems(t *testing.T) {
	// Arrange
	data := []byte(`<status><target path="/wc">
<entry path="/wc/a"><wc-status item="normal" props="none"></wc-status><repos-status item="modified" props="none"></repos-status></entry>
<entry path="/wc/b"><wc-status item="modified" props="none"></wc-status></entry>
<entry path="/wc/c"><wc-status item="none" props="none"></wc-status><repos-status item="added" props="none"></repos-status></entry>
<against revision="9"/></target></status>`)

	// Act
	n, err := ParseIncoming(data)

	// Assert
	if err != nil || n != 2 {
		t.Errorf("got (%d, %v), want 2", n, err)
	}
}

func TestParseCount_OneItemPerLine(t *testing.T) {
	// Act
	n := ParseCount([]byte("a.txt\nsrc/\nsrc/main.go\n\n"))

	// Assert
	if n != 3 {
		t.Errorf("n = %d, want 3", n)
	}
}
