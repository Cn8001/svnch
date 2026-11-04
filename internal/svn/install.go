package svn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var ErrNoInstaller = errors.New("no supported package manager found")

// Setup is what the window needs to explain a missing svn instead of failing on every command.
type Setup struct {
	Binary  string `json:"binary"`  // empty while svn is missing
	Version string `json:"version"` // e.g. 1.14.3
	OS      string `json:"os"`      // runtime.GOOS, for the install instructions
	Problem string `json:"problem"` // why the automatic install did not work
}

// Detect looks again on every call, so svn installed by hand while the app runs is picked up without a restart.
func Detect(ctx context.Context, binary, problem string) Setup {
	s := Setup{OS: runtime.GOOS}
	if binary == "" || !fileExists(binary) {
		binary = findSvn()
		if binary == "" {
			s.Problem = problem
			return s
		}
	}
	s.Binary = binary
	if out, err := command(ctx, binary, "--version", "--quiet").Output(); err == nil {
		s.Version = strings.TrimSpace(string(out))
	}
	return s
}

// EnsureInstalled returns the svn binary to run, installing Subversion through the OS package manager when it is missing.
func EnsureInstalled(ctx context.Context) (string, error) {
	if path := findSvn(); path != "" {
		return path, nil
	}

	args, err := installCommand(runtime.GOOS, hasCommand)
	if err != nil {
		return "", fmt.Errorf("install svn: %w", err)
	}
	cmd := command(ctx, args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("install svn with %s: %w", args[0], err)
	}

	if path := findSvn(); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("svn still missing after install: %w", ErrSvnNotFound)
}

func installCommand(goos string, has func(string) bool) ([]string, error) {
	switch goos {
	case "windows":
		if has("winget") {
			return []string{"winget", "install", "--id", "Slik.Subversion", "-e", "--silent",
				"--accept-package-agreements", "--accept-source-agreements"}, nil
		}
	case "darwin":
		if has("brew") {
			return []string{"brew", "install", "subversion"}, nil
		}
	case "linux":
		// pkexec shows a graphical password prompt, since the app has no terminal for sudo.
		if !has("pkexec") {
			return nil, fmt.Errorf("pkexec missing: %w", ErrNoInstaller)
		}
		switch {
		case has("apt-get"):
			return []string{"pkexec", "apt-get", "install", "-y", "subversion"}, nil
		case has("dnf"):
			return []string{"pkexec", "dnf", "install", "-y", "subversion"}, nil
		case has("pacman"):
			return []string{"pkexec", "pacman", "-S", "--noconfirm", "subversion"}, nil
		case has("zypper"):
			return []string{"pkexec", "zypper", "install", "-y", "subversion"}, nil
		}
	}
	return nil, fmt.Errorf("%s: %w", goos, ErrNoInstaller)
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// findSvn looks on PATH, then on Windows where the usual installers put svn.exe: an install made after this
// session started, TortoiseSVN's included, is not on this process's PATH yet, and winget never updates it.
func findSvn() string {
	if path, err := exec.LookPath("svn"); err == nil {
		return path
	}
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, p := range windowsSvnPaths(os.Getenv) {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func windowsSvnPaths(env func(string) string) []string {
	var out []string
	for _, root := range []string{env("ProgramFiles"), env("ProgramFiles(x86)"), env("ProgramW6432")} {
		if root == "" {
			continue
		}
		for _, dir := range []string{`TortoiseSVN\bin`, `SlikSvn\bin`, `VisualSVN Server\bin`, `Subversion\bin`, `CollabNet\Subversion Client`} {
			out = append(out, root+`\`+dir+`\svn.exe`)
		}
	}
	return out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
