package explorer

import "testing"

func TestExeOf_ReadsBackTheWrittenCommand(t *testing.T) {
	// Arrange
	m := New(`C:\Program Files\svnch\svnch.exe`)

	// Act
	got := exeOf(m.command("%1"))

	// Assert
	if got != m.exe {
		t.Errorf("exeOf = %q, want %q", got, m.exe)
	}
}

func TestExeOf_UnquotedCommandIsNotOurs(t *testing.T) {
	// Arrange
	cmd := `notepad.exe "%1"`

	// Act
	got := exeOf(cmd)

	// Assert
	if got != "" {
		t.Errorf("exeOf = %q, want empty", got)
	}
}
