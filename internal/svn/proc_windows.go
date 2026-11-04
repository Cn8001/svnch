package svn

import (
	"context"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// command starts svn without a console window; a GUI program on Windows otherwise flashes a cmd window for every call.
func command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}
