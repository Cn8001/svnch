package service

import (
	"context"
	"fmt"

	"github.com/Cn8001/svnch/internal/servers"
)

type ServerService struct {
	store *servers.Store
}

func NewServerService(store *servers.Store) *ServerService {
	return &ServerService{store: store}
}

func (s *ServerService) List(ctx context.Context) ([]servers.Server, error) {
	return s.store.List(ctx)
}

func (s *ServerService) Save(ctx context.Context, srv servers.Server) error {
	if err := s.store.Save(ctx, srv); err != nil {
		return fmt.Errorf("save server: %w", err)
	}
	return nil
}

func (s *ServerService) Remove(ctx context.Context, url string) error {
	if err := s.store.Remove(ctx, url); err != nil {
		return fmt.Errorf("remove server: %w", err)
	}
	return nil
}
