package service

import (
	"context"
	"fmt"

	"github.com/Cn8001/svnch/internal/explorer"
)

type ExplorerService struct {
	menu *explorer.Menu
}

func NewExplorerService(menu *explorer.Menu) *ExplorerService {
	return &ExplorerService{menu: menu}
}

func (s *ExplorerService) Status(ctx context.Context) (explorer.Status, error) {
	return s.menu.Status(ctx)
}

func (s *ExplorerService) SetEnabled(ctx context.Context, on bool) error {
	if !on {
		if err := s.menu.Disable(ctx); err != nil {
			return fmt.Errorf("explorer menu off: %w", err)
		}
		return nil
	}
	if err := s.menu.Enable(ctx); err != nil {
		return fmt.Errorf("explorer menu on: %w", err)
	}
	return nil
}

func (s *ExplorerService) OpenFile(ctx context.Context, path string) error {
	return explorer.Open(ctx, path)
}
