package svn

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Progress is how far a running svn command has come: the items it has handled so far and the latest one.
type Progress struct {
	Command string `json:"command"`
	Items   int    `json:"items"`
	Item    string `json:"item"`
}

// statusColumns is the line svn prints per item while updating, checking out, switching, merging or exporting: up
// to four status letters, a space, and the path.
var statusColumns = regexp.MustCompile(`^[ADUCGERB ][ADUCGERB ][ B][ C] (.+)$`)

// commitVerbs are the words svn commit starts a line with for each item it sends.
var commitVerbs = []string{"Sending ", "Adding ", "Deleting ", "Replacing "}

// itemLine reads one line of svn's output and tells whether it names an item being handled, and which.
func itemLine(line string) (string, bool) {
	for _, v := range commitVerbs {
		if rest, ok := strings.CutPrefix(line, v); ok {
			return strings.TrimSpace(rest), true
		}
	}
	m := statusColumns.FindStringSubmatch(line)
	if m == nil || strings.TrimSpace(line[:4]) == "" {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// progressWriter keeps everything svn writes to stdout, as the caller parses it at the end, and meanwhile counts the
// item lines and reports them at most every interval, so a checkout of many files does not flood the window.
type progressWriter struct {
	out      bytes.Buffer
	line     []byte
	p        Progress
	report   func(Progress)
	interval time.Duration
	now      func() time.Time
	sent     time.Time
}

func newProgressWriter(command string, report func(Progress)) *progressWriter {
	return &progressWriter{p: Progress{Command: command}, report: report, interval: 100 * time.Millisecond, now: time.Now}
}

func (w *progressWriter) Write(b []byte) (int, error) {
	w.out.Write(b)
	for _, c := range b {
		if c != '\n' {
			w.line = append(w.line, c)
			continue
		}
		if item, ok := itemLine(strings.TrimRight(string(w.line), "\r")); ok {
			w.p.Items++
			w.p.Item = item
		}
		w.line = w.line[:0]
	}
	if t := w.now(); t.Sub(w.sent) >= w.interval {
		w.sent = t
		w.report(w.p)
	}
	return len(b), nil
}

// done sends the final count, which the throttling may have held back.
func (w *progressWriter) done() {
	w.report(w.p)
}

// IncomingArgs asks the server what an update would bring, without changing the working copy.
func IncomingArgs(path string) ([]string, error) {
	t, err := targets(path)
	if err != nil {
		return nil, err
	}
	return append([]string{"--xml", "-u"}, t...), nil
}

// ParseIncoming counts the items svn status -u says the server has newer versions of.
func ParseIncoming(data []byte) (int, error) {
	var doc struct {
		Entries []struct {
			Repos struct {
				Item string `xml:"item,attr"`
			} `xml:"repos-status"`
		} `xml:"target>entry"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return 0, fmt.Errorf("parse svn status -u: %w", err)
	}
	n := 0
	for _, e := range doc.Entries {
		if e.Repos.Item != "" && e.Repos.Item != "none" {
			n++
		}
	}
	return n, nil
}

func CountArgs(url string) ([]string, error) {
	t, err := targets(url)
	if err != nil {
		return nil, err
	}
	return append([]string{"-R"}, t...), nil
}

// ParseCount counts what svn list -R printed, one file or folder per line, the same items a checkout adds.
func ParseCount(out []byte) int {
	n := 0
	for line := range strings.Lines(string(out)) {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}
