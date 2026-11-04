package service

import (
	"context"

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
