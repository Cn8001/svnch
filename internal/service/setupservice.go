package service

import (
	"context"
	"fmt"

	"github.com/Cn8001/svnch/internal/svn"
)

// SetupService reports whether svn is there, so the window can show install help instead of failing commands.
type SetupService struct {
	binary  string
	problem string
}

func NewSetupService(binary string, installErr error) *SetupService {
	s := &SetupService{binary: binary}
	if installErr != nil {
		s.problem = installErr.Error()
	}
	return s
}

func (s *SetupService) Status(ctx context.Context) svn.Setup {
	return svn.Detect(ctx, s.binary, s.problem)
}

// CreateRepository makes an empty repository in folder with the svnadmin that came with svn, and returns its url.
func (s *SetupService) CreateRepository(ctx context.Context, folder string) (string, error) {
	setup := svn.Detect(ctx, s.binary, s.problem)
	if setup.Binary == "" {
		return "", fmt.Errorf("create repository: %w", svn.ErrSvnNotFound)
	}
	return svn.CreateRepository(ctx, setup.Binary, folder)
}
