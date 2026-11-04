//go:build integration

// The integration test runs every SVNService method against a real svn and a repository it creates on disk, so it
// catches what the unit tests with a fake runner cannot: svn's real output, its real errors, and how the commands
// combine. It needs svn and svnadmin on PATH:
//
//	go test -race -tags integration ./internal/service/
package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Cn8001/svnch/internal/svn"
)

type world struct {
	t    *testing.T
	ctx  context.Context
	s    *SVNService
	repo string // file:// url of the repository root
	dir  string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	bin, err := exec.LookPath("svn")
	if err != nil {
		t.Skip("svn is not on PATH")
	}
	// A home of its own, so svn's config and auth stores are the test's and not the user's.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	// The long form of the path: Windows hands out the temp folder by its 8.3 short name, and svn, which writes the
	// long one, then takes the same folder for two and refuses a move inside it as "already locked".
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url, err := svn.CreateRepository(context.Background(), bin, filepath.Join(dir, "repo"))
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	r := svn.NewExecRunner(bin)
	return &world{t: t, ctx: context.Background(), s: New(r, r, http.DefaultClient), repo: url, dir: dir}
}

func (w *world) must(err error, what string) {
	w.t.Helper()
	if err != nil {
		w.t.Fatalf("%s: %v", what, err)
	}
}

func (w *world) write(path, text string) {
	w.t.Helper()
	w.must(os.MkdirAll(filepath.Dir(path), 0o755), "mkdir")
	w.must(os.WriteFile(path, []byte(text), 0o644), "write "+path)
}

func (w *world) read(path string) string {
	w.t.Helper()
	b, err := os.ReadFile(path)
	w.must(err, "read "+path)
	return string(b)
}

func (w *world) status(wc string) map[string]svn.StatusEntry {
	w.t.Helper()
	entries, err := w.s.Status(w.ctx, wc)
	w.must(err, "status")
	m := map[string]svn.StatusEntry{}
	for _, e := range entries {
		rel, _ := filepath.Rel(wc, e.Path)
		m[filepath.ToSlash(rel)] = e
	}
	return m
}

// unlocked fails the test when a step left the working copy locked, which svn reports as wc-locked in its status.
func (w *world) unlocked(wc, after string) {
	w.t.Helper()
	bin, _ := exec.LookPath("svn")
	out, err := exec.Command(bin, "status", "--xml", "-v", "--non-interactive", wc).CombinedOutput()
	if err != nil {
		w.t.Fatalf("after %s: status: %v\n%s", after, err, out)
	}
	if strings.Contains(string(out), `wc-locked="true"`) {
		w.t.Fatalf("after %s the working copy is locked:\n%s", after, out)
	}
}

func (w *world) checkout(url, name string) string {
	w.t.Helper()
	path := filepath.Join(w.dir, name)
	_, err := w.s.Checkout(w.ctx, svn.CheckoutRequest{URL: url, Path: path})
	w.must(err, "checkout "+name)
	return path
}

// commit updates first, as svnch's Commit does: svn refuses a folder's property change while the folder is behind.
func (w *world) commit(wc, message string, paths ...string) int64 {
	w.t.Helper()
	if len(paths) == 0 {
		paths = []string{wc}
	}
	_, err := w.s.Update(w.ctx, []string{wc})
	w.must(err, "update before commit")
	rev, err := w.s.Commit(w.ctx, svn.CommitRequest{Paths: paths, Message: message})
	w.must(err, "commit "+message)
	if rev == 0 {
		w.t.Fatalf("commit %q committed nothing", message)
	}
	return rev
}

func TestIntegration_EverySVNServiceMethod(t *testing.T) {
	w := newWorld(t)
	ctx := w.ctx
	trunk := w.repo + "/trunk"

	_, err := w.s.MakeDir(ctx, trunk, "Add trunk")
	w.must(err, "mkdir trunk")
	_, err = w.s.MakeDir(ctx, w.repo+"/branches", "Add branches")
	w.must(err, "mkdir branches")

	wc := w.checkout(trunk, "wc")
	other := w.checkout(trunk, "other")
	readme := filepath.Join(wc, "README.txt")
	code := filepath.Join(wc, "src", "main.go")

	t.Run("info", func(t *testing.T) {
		info, err := w.s.Info(ctx, wc)
		w.must(err, "info")
		if info.RelativeURL != "^/trunk" || info.URL != trunk || info.RepositoryRoot != w.repo {
			t.Errorf("info = %+v", info)
		}
	})
	w.unlocked(wc, "info")
	w.unlocked(other, "info")

	t.Run("add and commit", func(t *testing.T) {
		w.write(readme, "line one\nline two\nline three\n")
		w.write(code, "package main\n")
		if st := w.status(wc); st["README.txt"].Item != "unversioned" || st["src"].Item != "unversioned" {
			t.Fatalf("before add: %+v", st)
		}
		w.must(w.s.Add(ctx, []string{readme, filepath.Join(wc, "src")}), "add")
		if st := w.status(wc); st["README.txt"].Item != "added" || st["src/main.go"].Item != "added" {
			t.Fatalf("after add: %+v", st)
		}
		w.commit(wc, "First files")
		if st := w.status(wc); len(st) != 0 {
			t.Errorf("after commit: %+v", st)
		}
	})
	w.unlocked(wc, "add and commit")
	w.unlocked(other, "add and commit")

	t.Run("diff, log and blame", func(t *testing.T) {
		w.write(readme, "line one\nline two changed\nline three\n")
		diff, err := w.s.Diff(ctx, svn.DiffRequest{Target: readme})
		w.must(err, "diff")
		if !strings.Contains(diff, "+line two changed") || !strings.Contains(diff, "-line two") {
			t.Errorf("diff = %q", diff)
		}
		rev := w.commit(wc, "Change line two")
		log, err := w.s.Log(ctx, svn.LogRequest{Target: trunk, Limit: 10, Verbose: true})
		w.must(err, "log")
		if len(log) == 0 || log[0].Revision != rev || log[0].Message != "Change line two" || len(log[0].Paths) == 0 {
			t.Errorf("log = %+v", log)
		}
		lines, err := w.s.Blame(ctx, readme)
		w.must(err, "blame")
		if len(lines) != 3 || lines[1].Text != "line two changed" || lines[1].Revision != rev || lines[0].Revision == rev {
			t.Errorf("blame = %+v", lines)
		}
		changes, err := w.s.Diff(ctx, svn.DiffRequest{Target: readme, Change: rev})
		w.must(err, "diff -c")
		if !strings.Contains(changes, "+line two changed") {
			t.Errorf("diff -c %d = %q", rev, changes)
		}
	})
	w.unlocked(wc, "diff, log and blame")
	w.unlocked(other, "diff, log and blame")

	t.Run("progress totals", func(t *testing.T) {
		n, err := w.s.Incoming(ctx, other)
		w.must(err, "incoming")
		if n < 2 {
			t.Errorf("incoming = %d, want the two committed files", n)
		}
		files, err := w.s.CountFiles(ctx, trunk)
		w.must(err, "count")
		if files != 3 {
			t.Errorf("count = %d, want README.txt, src/ and src/main.go", files)
		}
	})
	w.unlocked(wc, "progress totals")
	w.unlocked(other, "progress totals")

	t.Run("update and conflict", func(t *testing.T) {
		rev, err := w.s.Update(ctx, []string{other})
		w.must(err, "update other")
		otherReadme := filepath.Join(other, "README.txt")
		if rev == 0 || !strings.Contains(w.read(otherReadme), "line two changed") {
			t.Fatalf("update brought r%d: %q", rev, w.read(otherReadme))
		}
		w.write(readme, "line one mine\nline two changed\nline three\n")
		w.commit(wc, "Mine")
		w.write(otherReadme, "line one theirs\nline two changed\nline three\n")
		_, err = w.s.Update(ctx, []string{other})
		w.must(err, "update into conflict")
		if st := w.status(other); st["README.txt"].Item != "conflicted" {
			t.Fatalf("status after clash: %+v", st)
		}
		text, err := w.s.ReadLocal(ctx, other, otherReadme)
		w.must(err, "read conflict")
		if !strings.Contains(text.Text, "<<<<<<<") || !strings.Contains(text.Text, ">>>>>>>") {
			t.Errorf("no markers: %q", text.Text)
		}
		w.must(w.s.Resolve(ctx, svn.AcceptTheirsFull, []string{otherReadme}), "resolve")
		if st := w.status(other); len(st) != 0 {
			t.Errorf("after resolve: %+v", st)
		}
		if got := w.read(otherReadme); !strings.HasPrefix(got, "line one mine") {
			t.Errorf("theirs-full kept %q", got)
		}
	})
	w.unlocked(wc, "update and conflict")
	w.unlocked(other, "update and conflict")

	t.Run("update to an old revision and back", func(t *testing.T) {
		_, err := w.s.Update(ctx, []string{wc})
		w.must(err, "update")
		rev, err := w.s.UpdateTo(ctx, wc, 3)
		w.must(err, "update to r3")
		if rev != 3 || strings.Contains(w.read(readme), "changed") {
			t.Errorf("at r%d: %q", rev, w.read(readme))
		}
		_, err = w.s.Update(ctx, []string{wc})
		w.must(err, "back to head")
		if !strings.Contains(w.read(readme), "mine") {
			t.Errorf("head readme = %q", w.read(readme))
		}
	})
	w.unlocked(wc, "update to an old revision and back")
	w.unlocked(other, "update to an old revision and back")

	t.Run("undo a commit", func(t *testing.T) {
		log, err := w.s.Log(ctx, svn.LogRequest{Target: trunk, Limit: 1})
		w.must(err, "log")
		_, err = w.s.Merge(ctx, svn.MergeRequest{Source: trunk, Target: wc, Change: -log[0].Revision})
		w.must(err, "reverse merge")
		if st := w.status(wc); st["README.txt"].Item != "modified" || !strings.HasPrefix(w.read(readme), "line one\n") {
			t.Errorf("after undo: %+v %q", st, w.read(readme))
		}
	})
	w.unlocked(wc, "undo a commit")
	w.unlocked(other, "undo a commit")

	t.Run("discard", func(t *testing.T) {
		scratch := filepath.Join(wc, "scratch.txt")
		added := filepath.Join(wc, "added.txt")
		w.write(scratch, "temp\n")
		w.write(added, "new\n")
		w.must(w.s.Add(ctx, []string{added}), "add")
		res, err := w.s.Discard(ctx, wc, []string{wc})
		w.must(err, "discard")
		if res.Removed != 2 || res.Reverted < 2 {
			t.Errorf("discard = %+v", res)
		}
		for _, p := range []string{scratch, added} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("%s still there", p)
			}
		}
		if st := w.status(wc); len(st) != 0 || !strings.Contains(w.read(readme), "mine") {
			t.Errorf("after discard: %+v", st)
		}
	})
	w.unlocked(wc, "discard")
	w.unlocked(other, "discard")

	t.Run("many paths at once", func(t *testing.T) {
		// Enough long paths to pass Windows' command line limit, as a big game project's assets do.
		var paths []string
		for i := range 600 {
			p := filepath.Join(wc, "Assets", "Environment", "Textures", "Terrain", fmt.Sprintf("tile_%04d_albedo_normal.png", i))
			w.write(p, "x")
			paths = append(paths, p)
		}
		w.must(w.s.Add(ctx, paths), "add 600")
		if st := w.status(wc); st["Assets/Environment/Textures/Terrain/tile_0599_albedo_normal.png"].Item != "added" {
			t.Fatalf("not added: %d entries", len(st))
		}
		res, err := w.s.Discard(ctx, wc, []string{wc})
		w.must(err, "discard all")
		if res.Removed == 0 {
			t.Errorf("discard = %+v", res)
		}
		if st := w.status(wc); len(st) != 0 {
			t.Errorf("after discard all: %d entries left", len(st))
		}
	})
	w.unlocked(wc, "many paths at once")
	w.unlocked(other, "many paths at once")

	t.Run("move and delete", func(t *testing.T) {
		moved := filepath.Join(wc, "src", "app.go")
		w.must(w.s.Move(ctx, code, moved), "move")
		w.must(w.s.Delete(ctx, []string{readme}, false), "delete")
		st := w.status(wc)
		if st["src/app.go"].Item != "added" || st["src/main.go"].Item != "deleted" || st["README.txt"].Item != "deleted" {
			t.Fatalf("status = %+v", st)
		}
		w.commit(wc, "Move and delete")
		w.write(readme, "back\n")
		w.must(w.s.Add(ctx, []string{readme}), "add back")
		code = moved
		w.commit(wc, "Readme back")
	})
	w.unlocked(wc, "move and delete")
	w.unlocked(other, "move and delete")

	t.Run("properties and ignore", func(t *testing.T) {
		_, err := w.s.SetProperty(ctx, svn.PropRequest{Targets: []string{readme}, Name: "svn:eol-style", Value: "native"})
		w.must(err, "propset")
		_, err = w.s.SetProperty(ctx, svn.PropRequest{Targets: []string{readme}, Name: "x:note", Value: "-starts with a dash\nand two lines"})
		w.must(err, "propset dash")
		props, err := w.s.Properties(ctx, readme)
		w.must(err, "proplist")
		got := map[string]string{}
		for _, p := range props {
			got[p.Name] = p.Value
		}
		if got["svn:eol-style"] != "native" || got["x:note"] != "-starts with a dash\nand two lines" {
			t.Errorf("properties = %q", got)
		}
		_, err = w.s.DeleteProperty(ctx, svn.PropRequest{Targets: []string{readme}, Name: "x:note"})
		w.must(err, "propdel")
		bin := filepath.Join(wc, "bin")
		w.write(filepath.Join(bin, "out.exe"), "x")
		w.must(w.s.Ignore(ctx, []string{bin}), "ignore")
		st := w.status(wc)
		if st["bin"].Item != "ignored" || st["."].Props != "modified" {
			t.Errorf("after ignore: %+v", st)
		}
		w.commit(wc, "Properties")
	})
	w.unlocked(wc, "properties and ignore")
	w.unlocked(other, "properties and ignore")

	t.Run("changelists", func(t *testing.T) {
		w.write(readme, "back, edited\n")
		w.must(w.s.SetChangelist(ctx, "docs", []string{readme}), "changelist")
		if st := w.status(wc); st["README.txt"].Changelist != "docs" {
			t.Errorf("status = %+v", st)
		}
		w.must(w.s.RemoveChangelist(ctx, []string{readme}), "remove changelist")
		if st := w.status(wc); st["README.txt"].Changelist != "" {
			t.Errorf("status = %+v", st)
		}
	})
	w.unlocked(wc, "changelists")
	w.unlocked(other, "changelists")

	t.Run("lock", func(t *testing.T) {
		w.must(w.s.Lock(ctx, []string{code}, "editing", false), "lock")
		if st := w.status(wc); !st["src/app.go"].Locked {
			t.Errorf("not locked: %+v", st)
		}
		w.must(w.s.Unlock(ctx, []string{code}, false), "unlock")
		if st := w.status(wc); st["src/app.go"].Locked {
			t.Errorf("still locked: %+v", st)
		}
	})
	w.unlocked(wc, "lock")
	w.unlocked(other, "lock")

	t.Run("shelve", func(t *testing.T) {
		w.must(w.s.Shelve(ctx, wc, "wip", "half done"), "shelve")
		if st := w.status(wc); st["README.txt"].Item == "modified" {
			t.Errorf("shelve left the change: %+v", st)
		}
		shelves, err := w.s.Shelves(ctx, wc)
		w.must(err, "shelves")
		if !slices.Contains(shelves, "wip") {
			t.Errorf("shelves = %q", shelves)
		}
		w.must(w.s.Unshelve(ctx, wc, "wip"), "unshelve")
		if !strings.Contains(w.read(readme), "edited") {
			t.Errorf("unshelve did not bring it back: %q", w.read(readme))
		}
	})
	w.unlocked(wc, "shelve")
	w.unlocked(other, "shelve")

	t.Run("patch", func(t *testing.T) {
		patch := filepath.Join(w.dir, "fix.patch")
		w.must(w.s.CreatePatch(ctx, []string{wc}, patch), "create patch")
		_, err := w.s.Discard(ctx, wc, []string{wc})
		w.must(err, "discard")
		out, err := w.s.ApplyPatch(ctx, wc, patch)
		w.must(err, "apply patch")
		if !strings.Contains(w.read(readme), "edited") {
			t.Errorf("patch did not apply (%q): %q", out, w.read(readme))
		}
		w.commit(wc, "Edited readme")
	})
	w.unlocked(wc, "patch")
	w.unlocked(other, "patch")

	t.Run("branch, switch and merge", func(t *testing.T) {
		feature := w.repo + "/branches/feature"
		_, err := w.s.Branch(ctx, svn.BranchRequest{From: trunk, To: feature, Message: "Create branch feature"})
		w.must(err, "branch")
		_, err = w.s.Switch(ctx, wc, feature)
		w.must(err, "switch")
		info, err := w.s.Info(ctx, wc)
		w.must(err, "info")
		if info.RelativeURL != "^/branches/feature" {
			t.Fatalf("on %q", info.RelativeURL)
		}
		w.write(readme, "from the branch\n")
		w.commit(wc, "Branch work")
		_, err = w.s.Switch(ctx, wc, trunk)
		w.must(err, "switch back")
		out, err := w.s.Merge(ctx, svn.MergeRequest{Source: feature, Target: wc})
		w.must(err, "merge")
		if !strings.Contains(w.read(readme), "from the branch") {
			t.Errorf("merge (%q) left %q", out, w.read(readme))
		}
		w.commit(wc, "Merge feature")
	})
	w.unlocked(wc, "branch, switch and merge")
	w.unlocked(other, "branch, switch and merge")

	t.Run("cleanup, upgrade and relocate", func(t *testing.T) {
		w.must(w.s.Cleanup(ctx, wc), "cleanup")
		w.must(w.s.Upgrade(ctx, wc), "upgrade")
		// The same repository under another address, as when a server's IP changes: a hot copy keeps its UUID.
		admin, err := exec.LookPath("svnadmin")
		w.must(err, "svnadmin")
		copyDir := filepath.Join(w.dir, "moved")
		out, err := exec.Command(admin, "hotcopy", filepath.Join(w.dir, "repo"), copyDir).CombinedOutput()
		w.must(err, "hotcopy: "+string(out))
		moved := svn.FileURL(copyDir) + "/trunk"
		w.must(w.s.Relocate(ctx, wc, moved), "relocate")
		info, err := w.s.Info(ctx, wc)
		w.must(err, "info")
		if info.URL != moved {
			t.Errorf("url = %q, want %q", info.URL, moved)
		}
		_, err = w.s.Update(ctx, []string{wc})
		w.must(err, "update after relocate")
		w.must(w.s.Relocate(ctx, wc, trunk), "relocate back")
	})
	w.unlocked(wc, "cleanup, upgrade and relocate")
	w.unlocked(other, "cleanup, upgrade and relocate")

	t.Run("export and import", func(t *testing.T) {
		out := filepath.Join(w.dir, "exported")
		w.must(w.s.ExportLocal(ctx, wc, out), "export")
		if _, err := os.Stat(filepath.Join(out, "README.txt")); err != nil {
			t.Errorf("export: %v", err)
		}
		if _, err := os.Stat(filepath.Join(out, ".svn")); !os.IsNotExist(err) {
			t.Errorf("export kept .svn")
		}
		in := filepath.Join(w.dir, "toimport")
		w.write(filepath.Join(in, "a.txt"), "a\n")
		_, err := w.s.Import(ctx, in, w.repo+"/imported", "Import a")
		w.must(err, "import")
		list, err := w.s.List(ctx, w.repo+"/imported")
		w.must(err, "list")
		if len(list) != 1 || list[0].Name != "a.txt" {
			t.Errorf("list = %+v", list)
		}
	})
	w.unlocked(wc, "export and import")
	w.unlocked(other, "export and import")

	t.Run("repository browser", func(t *testing.T) {
		_, err := w.s.MakeDir(ctx, w.repo+"/docs", "Add docs")
		w.must(err, "mkdir")
		_, err = w.s.MoveRemote(ctx, w.repo+"/imported/a.txt", w.repo+"/docs/a.txt", "Move a")
		w.must(err, "move remote")
		content, err := w.s.Cat(ctx, w.repo+"/docs/a.txt")
		w.must(err, "cat")
		if content.Text != "a\n" || content.Binary {
			t.Errorf("cat = %+v", content)
		}
		dest := filepath.Join(w.dir, "saved.txt")
		w.must(w.s.Export(ctx, w.repo+"/docs/a.txt", dest), "export file")
		if w.read(dest) != "a\n" {
			t.Errorf("saved %q", w.read(dest))
		}
		_, err = w.s.DeleteRemote(ctx, []string{w.repo + "/imported"}, "Drop imported")
		w.must(err, "delete remote")
		root, err := w.s.CheckConnection(ctx, w.repo)
		w.must(err, "check connection")
		var names []string
		for _, e := range root {
			names = append(names, e.Name)
		}
		slices.Sort(names)
		if !slices.Equal(names, []string{"branches", "docs", "trunk"}) {
			t.Errorf("root = %q", names)
		}
		_, err = w.s.SetProperty(ctx, svn.PropRequest{Targets: []string{w.repo + "/docs"}, Name: "x:owner", Value: "docs team", Message: "Owner"})
		w.must(err, "remote propset")
		props, err := w.s.Properties(ctx, w.repo+"/docs")
		w.must(err, "remote proplist")
		if len(props) != 1 || props[0].Value != "docs team" {
			t.Errorf("remote props = %+v", props)
		}
	})
	w.unlocked(wc, "repository browser")
	w.unlocked(other, "repository browser")

	t.Run("errors are matchable", func(t *testing.T) {
		_, err := w.s.Info(ctx, w.dir)
		if !errors.Is(err, svn.ErrNotWorkingCopy) {
			t.Errorf("info outside a working copy: %v", err)
		}
		if code := ToFailure(err).Code; code != "NOT_WORKING_COPY" {
			t.Errorf("failure code %q", code)
		}
		if _, err := w.s.Cat(ctx, w.repo+"/nope.txt"); err == nil {
			t.Errorf("cat of a missing file worked")
		}
	})
	w.unlocked(wc, "errors are matchable")
	w.unlocked(other, "errors are matchable")

	t.Run("help", func(t *testing.T) {
		text, err := w.s.Help(ctx, "commit")
		w.must(err, "help")
		if !strings.Contains(text, "commit") {
			t.Errorf("help = %q", text)
		}
	})
	w.unlocked(wc, "help")
	w.unlocked(other, "help")
}
