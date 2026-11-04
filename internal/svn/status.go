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
	Changelist     string `json:"changelist"` // empty when the item is in no changelist
	Locked         bool   `json:"locked"`     // this working copy holds the item's lock
}

// StatusArgs lists ignored items too, for the collapsed Ignored group; svn stops at an ignored folder, so a build
// output tree is one entry, not thousands.
func StatusArgs(path string) ([]string, error) {
	t, err := targets(path)
	if err != nil {
		return nil, err
	}
	return append([]string{"--xml", "--no-ignore"}, t...), nil
}

type statusXMLEntry struct {
	Path string `xml:"path,attr"`
	WC   struct {
		Item           string `xml:"item,attr"`
		Props          string `xml:"props,attr"`
		Revision       int64  `xml:"revision,attr"`
		Copied         bool   `xml:"copied,attr"`
		TreeConflicted bool   `xml:"tree-conflicted,attr"`
		Lock           *struct {
			Token string `xml:"token"`
		} `xml:"lock"`
	} `xml:"wc-status"`
}

// ParseStatus flattens the targets; svn lists items that are in a changelist in a <changelist> block of their own
// next to the targets, so those come out with the list's name.
func ParseStatus(data []byte) ([]StatusEntry, error) {
	var doc struct {
		Targets []struct {
			Entries []statusXMLEntry `xml:"entry"`
		} `xml:"target"`
		Changelists []struct {
			Name    string           `xml:"name,attr"`
			Entries []statusXMLEntry `xml:"entry"`
		} `xml:"changelist"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn status: %w", err)
	}

	entries := []StatusEntry{}
	add := func(e statusXMLEntry, changelist string) {
		entries = append(entries, StatusEntry{
			Path:           e.Path,
			Item:           e.WC.Item,
			Props:          e.WC.Props,
			Revision:       e.WC.Revision,
			Copied:         e.WC.Copied,
			TreeConflicted: e.WC.TreeConflicted,
			Changelist:     changelist,
			Locked:         e.WC.Lock != nil,
		})
	}
	for _, t := range doc.Targets {
		for _, e := range t.Entries {
			add(e, "")
		}
	}
	for _, cl := range doc.Changelists {
		for _, e := range cl.Entries {
			add(e, cl.Name)
		}
	}
	return entries, nil
}
