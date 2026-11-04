package svn

import (
	"errors"
	"slices"
	"testing"
)

func TestPropSetArgs_ValueAfterDoubleDash(t *testing.T) {
	// Arrange
	req := PropRequest{Targets: []string{"/wc/dir"}, Name: "svn:ignore", Value: "-weird\nbin\n", Recursive: true}

	// Act
	args, err := PropSetArgs(req)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"-R", "--", "svn:ignore", "-weird\nbin\n", "/wc/dir"}; !slices.Equal(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
}

func TestPropDelArgs_OnTheServerNeedsAMessage(t *testing.T) {
	// Arrange
	req := PropRequest{Targets: []string{"https://h/r/trunk"}, Name: "svn:externals"}

	// Act
	_, err := PropDelArgs(req)
	req.Message = "Drop externals"
	args, okErr := PropDelArgs(req)

	// Assert
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("without message: err = %v, want ErrInvalidRequest", err)
	}
	if want := []string{"-m", "Drop externals", "--", "svn:externals", "https://h/r/trunk"}; okErr != nil || !slices.Equal(args, want) {
		t.Errorf("args = %q (%v), want %q", args, okErr, want)
	}
}

func TestPropSetArgs_RejectsBadNames(t *testing.T) {
	// Arrange
	names := []string{"", "-R", "two words"}

	for _, n := range names {
		// Act
		_, err := PropSetArgs(PropRequest{Targets: []string{"/wc"}, Name: n})

		// Assert
		if !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("name %q: err = %v, want ErrInvalidRequest", n, err)
		}
	}
}

func TestParsePropList_DecodesBase64Values(t *testing.T) {
	// Arrange
	data := []byte(`<properties><target path="/wc"><property name="svn:ignore">bin
obj
</property><property name="x:bin" encoding="base64">AAE=</property></target></properties>`)

	// Act
	props, err := ParsePropList(data)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(props) != 2 || props[0].Value != "bin\nobj\n" || props[1].Value != "\x00\x01" {
		t.Errorf("props = %q", props)
	}
}

func TestAddIgnore_AppendsOnce(t *testing.T) {
	// Arrange
	current := "bin\n"

	// Act
	once := AddIgnore(current, "obj")
	twice := AddIgnore(once, "obj")
	fresh := AddIgnore("", "obj")

	// Assert
	if once != "bin\nobj\n" || twice != once || fresh != "obj\n" {
		t.Errorf("once %q, twice %q, fresh %q", once, twice, fresh)
	}
}
