// Package servers keeps the repository URLs a user has saved, so nobody types a URL twice. Passwords are not kept
// here; svn's own auth cache holds them.
package servers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

var ErrInvalid = errors.New("invalid server")

type Server struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"` // filled into the login dialog, may be empty
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) List(ctx context.Context) ([]Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Save adds a server, or replaces the one with the same URL, so editing a name does not leave a duplicate behind.
func (s *Store) Save(ctx context.Context, srv Server) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	srv, err := normalize(srv)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(list, func(x Server) bool { return x.URL == srv.URL }); i >= 0 {
		list[i] = srv
	} else {
		list = append(list, srv)
	}
	return s.write(list)
}

func (s *Store) Remove(ctx context.Context, rawURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return err
	}
	target := strings.TrimRight(rawURL, "/")
	return s.write(slices.DeleteFunc(list, func(x Server) bool { return x.URL == target }))
}

func (s *Store) load() ([]Server, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Server{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read servers: %w", err)
	}
	list := []Server{}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path, err)
	}
	return list, nil
}

// write goes through a temporary file, so a crash mid-write never leaves a half list behind.
func (s *Store) write(list []Server) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("encode servers: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create config folder: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write servers: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace servers: %w", err)
	}
	return nil
}

func normalize(srv Server) (Server, error) {
	srv.URL = strings.TrimRight(strings.TrimSpace(srv.URL), "/")
	srv.Name = strings.TrimSpace(srv.Name)
	srv.Username = strings.TrimSpace(srv.Username)
	u, err := url.Parse(srv.URL)
	if err != nil || u.Host == "" && u.Scheme != "file" {
		return Server{}, fmt.Errorf("url %q: %w", srv.URL, ErrInvalid)
	}
	switch u.Scheme {
	case "http", "https", "svn", "svn+ssh", "file":
	default:
		return Server{}, fmt.Errorf("url scheme %q: %w", u.Scheme, ErrInvalid)
	}
	if srv.Name == "" {
		srv.Name = u.Host + u.Path
	}
	return srv, nil
}
