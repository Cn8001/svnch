package svn

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestPlanDiscard_RevertsChangesAndRemovesNewFiles(t *testing.T) {
	// Arrange
	wc := filepath.FromSlash("/wc")
	p := func(s string) string { return filepath.FromSlash(s) }
	entries := []StatusEntry{
		{Path: p("/wc/mod.go"), Item: "modified"},
		{Path: p("/wc/new.txt"), Item: "unversioned"},
		{Path: p("/wc/added.go"), Item: "added"},
		{Path: p("/wc/bin"), Item: "ignored"},
		{Path: p("/wc/gone.go"), Item: "missing"},
	}

	// Act
	plan := PlanDiscard(wc, entries)

	// Assert
	if want := []string{p("/wc/mod.go"), p("/wc/added.go"), p("/wc/gone.go")}; !slices.Equal(plan.Revert, want) {
		t.Errorf("revert = %q, want %q", plan.Revert, want)
	}
	if want := []string{p("/wc/new.txt"), p("/wc/added.go")}; !slices.Equal(plan.Remove, want) {
		t.Errorf("remove = %q, want %q", plan.Remove, want)
	}
}

func TestPlanDiscard_NeverRemovesOutsideOrTheWorkingCopyItself(t *testing.T) {
	// Arrange
	wc := filepath.FromSlash("/wc")
	entries := []StatusEntry{
		{Path: filepath.FromSlash("/wc"), Item: "unversioned"},
		{Path: filepath.FromSlash("/elsewhere/x"), Item: "unversioned"},
		{Path: filepath.FromSlash("/wc/../wc2/y"), Item: "unversioned"},
	}

	// Act
	plan := PlanDiscard(wc, entries)

	// Assert
	if len(plan.Remove) != 0 {
		t.Errorf("remove = %q, want nothing", plan.Remove)
	}
}

func TestRevertRoots_SkipsAskedPathsThatAreNew(t *testing.T) {
	// Arrange
	entries := []StatusEntry{{Path: "/wc/new.txt", Item: "unversioned"}, {Path: "/wc/a.go", Item: "modified"}}

	// Act
	roots := RevertRoots([]string{"/wc", "/wc/new.txt", "/wc/a.go"}, entries)

	// Assert
	if !slices.Equal(roots, []string{"/wc", "/wc/a.go"}) {
		t.Errorf("roots = %q", roots)
	}
}
