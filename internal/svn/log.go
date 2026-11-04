package svn

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"time"
)

type ChangedPath struct {
	Path   string `xml:",chardata" json:"path"`
	Action string `xml:"action,attr" json:"action"` // A, M, D or R
	Kind   string `xml:"kind,attr" json:"kind"`
}

type LogEntry struct {
	Revision int64         `xml:"revision,attr" json:"revision"`
	Author   string        `xml:"author" json:"author"`
	Date     time.Time     `xml:"date" json:"date"`
	Message  string        `xml:"msg" json:"message"`
	Paths    []ChangedPath `xml:"paths>path" json:"paths"`
}

type LogRequest struct {
	Target  string `json:"target"`
	Limit   int    `json:"limit"`   // 0 means no limit
	Verbose bool   `json:"verbose"` // list the paths each revision changed
}

func LogArgs(req LogRequest) ([]string, error) {
	if req.Limit < 0 {
		return nil, fmt.Errorf("log limit %d: %w", req.Limit, ErrInvalidRequest)
	}
	args := []string{"--xml"}
	if req.Limit > 0 {
		args = append(args, "-l", strconv.Itoa(req.Limit))
	}
	if req.Verbose {
		args = append(args, "-v")
	}
	t, err := targets(req.Target)
	if err != nil {
		return nil, err
	}
	return append(args, t...), nil
}

func ParseLog(data []byte) ([]LogEntry, error) {
	var doc struct {
		Entries []LogEntry `xml:"logentry"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn log: %w", err)
	}
	if doc.Entries == nil {
		return []LogEntry{}, nil
	}
	return doc.Entries, nil
}
