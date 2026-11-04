package svn

import (
	"encoding/xml"
	"fmt"
)

type ListEntry struct {
	Name       string `xml:"name" json:"name"`
	Kind       string `xml:"kind,attr" json:"kind"` // file or dir
	Size       int64  `xml:"size" json:"size"`      // 0 for a dir
	LastCommit Commit `xml:"commit" json:"lastCommit"`
}

func ListArgs(target string) ([]string, error) {
	t, err := targets(target)
	if err != nil {
		return nil, err
	}
	return append([]string{"--xml"}, t...), nil
}

func ParseList(data []byte) ([]ListEntry, error) {
	var doc struct {
		Lists []struct {
			Entries []ListEntry `xml:"entry"`
		} `xml:"list"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn list: %w", err)
	}
	entries := []ListEntry{}
	for _, l := range doc.Lists {
		entries = append(entries, l.Entries...)
	}
	return entries, nil
}
