package svn

import (
	"encoding/xml"
	"fmt"
)

type StatusEntry struct {
	Path           string `json:"path"`
	Item           string `json:"item"`  // modified, added, deleted, unversioned, missing, conflicted, normal...
	Props          string `json:"props"` // none, normal, modified, conflicted
	Revision       int64  `json:"revision"`
	Copied         bool   `json:"copied"`
	TreeConflicted bool   `json:"treeConflicted"`
}

func StatusArgs(path string) ([]string, error) {
	t, err := targets(path)
	if err != nil {
		return nil, err
	}
	return append([]string{"--xml"}, t...), nil
}

func ParseStatus(data []byte) ([]StatusEntry, error) {
	var doc struct {
		Targets []struct {
			Entries []struct {
				Path string `xml:"path,attr"`
				WC   struct {
					Item           string `xml:"item,attr"`
					Props          string `xml:"props,attr"`
					Revision       int64  `xml:"revision,attr"`
					Copied         bool   `xml:"copied,attr"`
					TreeConflicted bool   `xml:"tree-conflicted,attr"`
				} `xml:"wc-status"`
			} `xml:"entry"`
		} `xml:"target"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn status: %w", err)
	}

	entries := []StatusEntry{}
	for _, t := range doc.Targets {
		for _, e := range t.Entries {
			entries = append(entries, StatusEntry{
				Path:           e.Path,
				Item:           e.WC.Item,
				Props:          e.WC.Props,
				Revision:       e.WC.Revision,
				Copied:         e.WC.Copied,
				TreeConflicted: e.WC.TreeConflicted,
			})
		}
	}
	return entries, nil
}
