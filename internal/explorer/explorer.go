// Package explorer adds an svnch submenu to the Windows Explorer right-click menu of folders and files, and reads
// back the command line those entries start svnch with.
package explorer

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

var ErrUnsupported = errors.New("the explorer menu is only on windows")

type Status struct {
	Supported bool `json:"supported"`
	Enabled   bool `json:"enabled"`
}

// Launch is what svnch was started to do: an Explorer entry passes --action and the clicked path.
type Launch struct {
	Action string `json:"action"` // one of the menu actions, such as update, commit, log, blame or cleanup
	Path   string `json:"path"`   // empty when svnch was started plainly
	IsDir  bool   `json:"isDir"`
}

// item is one menu entry; key orders them, since Explorer sorts static subcommands by key name, and sep draws a line
// above it.
type item struct {
	key, action, label string
	sep                bool
}

// topItems sit in Explorer's own menu, outside the submenu, as TortoiseSVN's most used commands do.
func topItems(folder bool) []item {
	items := []item{
		{"svnch1update", "update", "SVN Update", false},
		{"svnch2commit", "commit", "SVN Commit...", false},
	}
	if folder {
		items = append(items, item{"svnch3checkout", "checkout", "SVN Checkout...", false})
	}
	return items
}

// folderItems follows TortoiseSVN's submenu. Explorer cannot ask svn whether the folder is a working copy before it
// draws a static menu, so every entry is there, and svnch says so when one does not fit the folder.
func folderItems() []item {
	return []item{
		{"01open", "open", "Check for modifications", false},
		{"02log", "log", "Show log", false},
		{"03repos", "repos", "Repo-browser", false},
		{"04updaterev", "updaterev", "Update to revision...", true},
		{"05revert", "revert", "Revert...", false},
		{"06cleanup", "cleanup", "Clean up", false},
		{"07resolve", "resolve", "Resolve...", false},
		{"08branch", "branch", "Branch/tag...", true},
		{"09switch", "switch", "Switch...", false},
		{"10merge", "merge", "Merge...", false},
		{"11export", "export", "Export...", false},
		{"12relocate", "relocate", "Relocate...", false},
		{"13add", "add", "Add...", true},
		{"14shelve", "shelve", "Shelve...", false},
		{"15unshelve", "unshelve", "Unshelve...", false},
		{"16createpatch", "createpatch", "Create patch...", false},
		{"17applypatch", "applypatch", "Apply patch...", false},
		{"18properties", "properties", "Properties", false},
		{"19import", "import", "Import...", true},
		{"20createrepo", "createrepo", "Create repository here", false},
		{"21upgrade", "upgrade", "Upgrade working copy", false},
		{"22settings", "settings", "Settings", true},
		{"23help", "help", "Help", false},
		{"24about", "about", "About", false},
	}
}

func fileItems() []item {
	return []item{
		{"01diff", "diff", "Diff", false},
		{"02diffprev", "diffprev", "Diff with previous version", false},
		{"03log", "log", "Show log", false},
		{"04blame", "blame", "Blame...", false},
		{"05repos", "repos", "Repo-browser", false},
		{"06updaterev", "updaterev", "Update to revision...", true},
		{"07revert", "revert", "Revert...", false},
		{"08resolve", "resolve", "Resolve...", false},
		{"09lock", "lock", "Get lock...", true},
		{"10unlock", "unlock", "Release lock", false},
		{"11rename", "rename", "Rename...", true},
		{"12delete", "delete", "Delete", false},
		{"13add", "add", "Add", false},
		{"14ignore", "ignore", "Add to ignore list", false},
		{"15unversion", "unversion", "Unversion and add to ignore list", false},
		{"16createpatch", "createpatch", "Create patch...", true},
		{"17properties", "properties", "Properties", false},
		{"18open", "open", "Check for modifications", true},
	}
}

func known(action string) bool {
	for _, it := range append(append(folderItems(), fileItems()...), topItems(true)...) {
		if it.action == action {
			return true
		}
	}
	return false
}

// ParseLaunch takes the first --action it knows and the first argument that exists on disk, so "svnch ." works as
// well as the Explorer entries, and flags a launcher adds are ignored.
func ParseLaunch(args []string, stat func(string) (fs.FileInfo, error)) Launch {
	l := Launch{Action: "open"}
	actionSet := false
	for _, a := range args {
		if name, ok := strings.CutPrefix(a, "--"); ok {
			if !actionSet && known(name) {
				l.Action, actionSet = name, true
			}
			continue
		}
		if l.Path != "" || strings.HasPrefix(a, "-") {
			continue
		}
		st, err := stat(a)
		if err != nil {
			continue
		}
		if abs, err := filepath.Abs(a); err == nil {
			a = abs
		}
		l.Path, l.IsDir = a, st.IsDir()
	}
	return l
}

// Menu writes under the current user's classes, so it needs no administrator rights and no installer.
type Menu struct {
	exe string
}

func New(exe string) *Menu {
	return &Menu{exe: exe}
}

// command is what Explorer runs; arg is %1 for a clicked folder or file and %V for the background of an open folder.
func (m *Menu) command(action, arg string) string {
	return `"` + m.exe + `" --` + action + ` "` + arg + `"`
}

// exeOf reads the program back out of a command written by command.
func exeOf(command string) string {
	if len(command) < 2 || command[0] != '"' {
		return ""
	}
	end := 1
	for end < len(command) && command[end] != '"' {
		end++
	}
	return command[1:end]
}
