package explorer

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// Open hands path to the program the desktop opens it with, as a double-click in the file manager would. It does not
// tie the program to ctx: a bound call's context ends when the call returns, and that would close the editor again.
func Open(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	go cmd.Wait() // reaps the launcher; explorer.exe exits 1 even when it worked, so its result means nothing
	return nil
}
