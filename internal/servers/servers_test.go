package servers

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

func TestStore_SaveReplacesSameURL(t *testing.T) {
	// Arrange
	ctx := context.Background()
	s := NewStore(filepath.Join(t.TempDir(), "svnch", "servers.json"))

	// Act
	err1 := s.Save(ctx, Server{URL: "https://svn.example.org/repo/"})
	err2 := s.Save(ctx, Server{Name: "Work", URL: "https://svn.example.org/repo", Username: "me"})
	list, err := s.List(ctx)

	// Assert
	if err1 != nil || err2 != nil || err != nil {
		t.Fatalf("errors: %v %v %v", err1, err2, err)
	}
	want := []Server{{Name: "Work", URL: "https://svn.example.org/repo", Username: "me"}}
	if !slices.Equal(list, want) {
		t.Errorf("list = %+v, want %+v", list, want)
	}
}

func TestStore_DefaultNameAndRemove(t *testing.T) {
	// Arrange
	ctx := context.Background()
	s := NewStore(filepath.Join(t.TempDir(), "servers.json"))
	_ = s.Save(ctx, Server{URL: "svn://host/repo"})
	_ = s.Save(ctx, Server{URL: "https://other/svn"})

	// Act
	err := s.Remove(ctx, "svn://host/repo/")
	list, _ := s.List(ctx)

	// Assert
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(list) != 1 || list[0].Name != "other/svn" {
		t.Errorf("list = %+v", list)
	}
}

func TestStore_RejectsBadURL(t *testing.T) {
	for name, u := range map[string]string{"no scheme": "host/repo", "unknown scheme": "ftp://host/repo", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			// Arrange
			s := NewStore(filepath.Join(t.TempDir(), "servers.json"))

			// Act
			err := s.Save(context.Background(), Server{URL: u})

			// Assert
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestStore_MissingFileIsEmpty(t *testing.T) {
	// Act
	list, err := NewStore(filepath.Join(t.TempDir(), "none.json")).List(context.Background())

	// Assert
	if err != nil || list == nil || len(list) != 0 {
		t.Errorf("got (%v, %v), want an empty list", list, err)
	}
}
