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
	Action string `json:"action"` // open, update, commit, log or diff
	Path   string `json:"path"`   // empty when svnch was started plainly
	IsDir  bool   `json:"isDir"`
}

// item is one submenu entry; key orders them, since Explorer sorts static subcommands by key name.
type item struct{ key, action, label string }

func folderItems() []item {
	return []item{
		{"1open", "open", "Open in svnch"},
		{"2update", "update", "Update"},
		{"3commit", "commit", "Commit..."},
		{"4log", "log", "Show log"},
	}
}

func fileItems() []item {
	return []item{
		{"1diff", "diff", "Diff"},
		{"2commit", "commit", "Commit..."},
		{"3log", "log", "Show log"},
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
