// Package explorer adds "Open with svnch" to the Windows Explorer right-click menu of folders.
package explorer

import "errors"

var ErrUnsupported = errors.New("the explorer menu is only on windows")

type Status struct {
	Supported bool `json:"supported"`
	Enabled   bool `json:"enabled"`
}

// Menu writes under the current user's classes, so it needs no administrator rights and no installer.
type Menu struct {
	exe string
}

func New(exe string) *Menu {
	return &Menu{exe: exe}
}

// command is what Explorer runs; arg is %1 for a folder that was clicked and %V for the background of an open one.
func (m *Menu) command(arg string) string {
	return `"` + m.exe + `" "` + arg + `"`
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
