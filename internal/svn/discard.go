package svn

import (
	"path/filepath"
	"strings"
)

// DiscardPlan is what throwing away local changes takes: versioned changes go back with revert, and what svn has
// never committed (new files, and added ones once reverted) has to be deleted from disk.
type DiscardPlan struct {
	Revert []string
	Remove []string
}

type DiscardResult struct {
	Reverted int `json:"reverted"`
	Removed  int `json:"removed"`
}

// PlanDiscard works from status entries, never from what the caller asked for, so only items svn itself reports as
// new are deleted, and never anything outside the working copy or the working copy itself.
func PlanDiscard(wc string, entries []StatusEntry) DiscardPlan {
	var p DiscardPlan
	for _, e := range entries {
		switch e.Item {
		case "ignored", "normal", "external", "":
			if e.Props == "modified" || e.Props == "conflicted" || e.TreeConflicted {
				p.Revert = append(p.Revert, e.Path)
			}
		case "unversioned":
			if inside(wc, e.Path) {
				p.Remove = append(p.Remove, e.Path)
			}
		case "added":
			p.Revert = append(p.Revert, e.Path)
			if inside(wc, e.Path) {
				p.Remove = append(p.Remove, e.Path)
			}
		default:
			p.Revert = append(p.Revert, e.Path)
		}
	}
	return p
}

func inside(wc, path string) bool {
	rel, err := filepath.Rel(wc, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// RevertRoots is the asked-for paths that svn can revert: a path that is itself new to svn has nothing to go back to,
// and reverting it would fail, so it is left for the delete.
func RevertRoots(paths []string, entries []StatusEntry) []string {
	item := map[string]string{}
	for _, e := range entries {
		item[e.Path] = e.Item
	}
	var out []string
	for _, p := range paths {
		if it := item[p]; it != "unversioned" && it != "ignored" {
			out = append(out, p)
		}
	}
	return out
}
