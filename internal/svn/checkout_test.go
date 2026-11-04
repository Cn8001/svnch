package svn

import (
	"errors"
	"slices"
	"testing"
)

func TestCheckoutArgs_HeadWithDefaultDepth(t *testing.T) {
	// Arrange
	req := CheckoutRequest{URL: "https://svn.example.org/repo/trunk", Path: "/work/trunk"}

	// Act
	args, err := CheckoutArgs(req)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"--", "https://svn.example.org/repo/trunk", "/work/trunk"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
}

func TestCheckoutArgs_RevisionAndDepth(t *testing.T) {
	// Arrange
	req := CheckoutRequest{URL: "svn://host/repo", Path: "wc", Revision: 42, Depth: DepthImmediates}

	// Act
	args, err := CheckoutArgs(req)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"-r", "42", "--depth", "immediates", "--", "svn://host/repo", "wc"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
}

func TestCheckoutArgs_EscapesAtSignInPath(t *testing.T) {
	// Arrange
	req := CheckoutRequest{URL: "svn://host/repo", Path: "/work/me@home"}

	// Act
	args, err := CheckoutArgs(req)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := args[len(args)-1]; got != "/work/me@home@" {
		t.Errorf("path arg = %q, want %q", got, "/work/me@home@")
	}
}

func TestCheckoutArgs_RejectsBadRequests(t *testing.T) {
	cases := map[string]CheckoutRequest{
		"missing url":   {Path: "wc"},
		"missing path":  {URL: "svn://host/repo"},
		"negative rev":  {URL: "svn://host/repo", Path: "wc", Revision: -1},
		"unknown depth": {URL: "svn://host/repo", Path: "wc", Depth: "deep"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := CheckoutArgs(req)

			// Assert
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestParseCheckout_ReadsClosingRevision(t *testing.T) {
	// Arrange
	out := []byte("A    wc/a.txt\nA    wc/lib\n\nFetching external item into 'wc/lib/ext':\nChecked out external at revision 7.\n\nChecked out revision 1234.\n")

	// Act
	rev, err := ParseCheckout(out)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev != 1234 {
		t.Errorf("rev = %d, want 1234", rev)
	}
}

func TestParseCheckout_MissingRevisionLine(t *testing.T) {
	// Arrange
	out := []byte("A    wc/a.txt\n")

	// Act
	_, err := ParseCheckout(out)

	// Assert
	if err == nil {
		t.Fatal("expected an error")
	}
}
