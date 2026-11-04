// Package explorer adds an svnch submenu to the Windows Explorer right-click menu of folders and files, and reads
// back the command line those entries start svnch with.
package explorer

import (
	"embed"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed icons/*.ico
var icons embed.FS

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

// item is one entry of the svnch submenu; key orders them, since Explorer sorts static subcommands by key name, sep
// draws a line above it, and icon names its file under icons/.
type item struct {
	key, action, label, icon string
	sep                      bool
}

// folderItems follows TortoiseSVN's menu, its top entries first. Explorer cannot ask svn whether the folder is a
// working copy before it draws a static menu, so every entry is there, and svnch says so when one does not fit.
func folderItems() []item {
	return []item{
		{"01update", "update", "SVN Update", "update", false},
		{"02commit", "commit", "SVN Commit...", "commit", false},
		{"03checkout", "checkout", "SVN Checkout...", "checkout", false},
		{"04open", "open", "Check for modifications", "open", true},
		{"05log", "log", "Show log", "log", false},
		{"06repos", "repos", "Repo-browser", "repos", false},
		{"07updaterev", "updaterev", "Update to revision...", "history", true},
		{"08revert", "revert", "Revert...", "revert", false},
		{"09cleanup", "cleanup", "Clean up", "cleanup", false},
		{"10resolve", "resolve", "Resolve...", "resolve", false},
		{"11branch", "branch", "Branch/tag...", "branch", true},
		{"12switch", "switch", "Switch...", "switch", false},
		{"13merge", "merge", "Merge...", "merge", false},
		{"14export", "export", "Export...", "export", false},
		{"15relocate", "relocate", "Relocate...", "relocate", false},
		{"16add", "add", "Add...", "add", true},
		{"17shelve", "shelve", "Shelve...", "shelve", false},
		{"18unshelve", "unshelve", "Unshelve...", "unshelve", false},
		{"19createpatch", "createpatch", "Create patch...", "patch", false},
		{"20applypatch", "applypatch", "Apply patch...", "apply", false},
		{"21properties", "properties", "Properties", "props", false},
		{"22import", "import", "Import...", "import", true},
		{"23createrepo", "createrepo", "Create repository here", "newrepo", false},
		{"24upgrade", "upgrade", "Upgrade working copy", "upgrade", false},
		{"25settings", "settings", "Settings", "settings", true},
		{"26help", "help", "Help", "help", false},
		{"27about", "about", "About", "about", false},
	}
}

func fileItems() []item {
	return []item{
		{"01update", "update", "SVN Update", "update", false},
		{"02commit", "commit", "SVN Commit...", "commit", false},
		{"03diff", "diff", "Diff", "diff", true},
		{"04diffprev", "diffprev", "Diff with previous version", "diffprev", false},
		{"05log", "log", "Show log", "log", false},
		{"06blame", "blame", "Blame...", "blame", false},
		{"07repos", "repos", "Repo-browser", "repos", false},
		{"08updaterev", "updaterev", "Update to revision...", "history", true},
		{"09revert", "revert", "Revert...", "revert", false},
		{"10resolve", "resolve", "Resolve...", "resolve", false},
		{"11lock", "lock", "Get lock...", "lock", true},
		{"12unlock", "unlock", "Release lock", "unlock", false},
		{"13rename", "rename", "Rename...", "rename", true},
		{"14delete", "delete", "Delete", "delete", false},
		{"15add", "add", "Add", "add", false},
		{"16ignore", "ignore", "Add to ignore list", "ignore", false},
		{"17unversion", "unversion", "Unversion and add to ignore list", "unversion", false},
		{"18createpatch", "createpatch", "Create patch...", "patch", true},
		{"19properties", "properties", "Properties", "props", false},
		{"20open", "open", "Check for modifications", "open", true},
	}
}

func known(action string) bool {
	for _, it := range append(folderItems(), fileItems()...) {
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

// Menu writes under the current user's classes, so it needs no administrator rights and no installer. Explorer reads
// an icon only from a file, so the icons are copied out of the program into iconDir when the menu is added.
type Menu struct {
	exe     string
	iconDir string
}

func New(exe, iconDir string) *Menu {
	return &Menu{exe: exe, iconDir: iconDir}
}

func (m *Menu) iconPath(name string) string {
	return filepath.Join(m.iconDir, name+".ico")
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
