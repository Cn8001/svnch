package svn

import (
	"errors"
	"slices"
	"testing"
)

func has(names ...string) func(string) bool {
	return func(n string) bool { return slices.Contains(names, n) }
}

func TestInstallCommand_LinuxPrefersApt(t *testing.T) {
	// Act
	args, err := installCommand("linux", has("pkexec", "apt-get", "dnf"))

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"pkexec", "apt-get", "install", "-y", "subversion"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
}

func TestInstallCommand_LinuxWithoutPkexec(t *testing.T) {
	// Act
	_, err := installCommand("linux", has("apt-get"))

	// Assert
	if !errors.Is(err, ErrNoInstaller) {
		t.Errorf("err = %v, want ErrNoInstaller", err)
	}
}

func TestInstallCommand_MacUsesBrew(t *testing.T) {
	// Act
	args, err := installCommand("darwin", has("brew"))

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(args, []string{"brew", "install", "subversion"}) {
		t.Errorf("args = %q", args)
	}
}

func TestInstallCommand_NothingAvailable(t *testing.T) {
	// Act
	_, err := installCommand("windows", has())

	// Assert
	if !errors.Is(err, ErrNoInstaller) {
		t.Errorf("err = %v, want ErrNoInstaller", err)
	}
}
