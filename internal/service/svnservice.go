package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Cn8001/svnch/internal/svn"
)

type SVNService struct {
	runner svn.Runner
}

func New(runner svn.Runner) *SVNService {
	return &SVNService{runner: runner}
}

func (s *SVNService) Info(ctx context.Context, target string) (svn.Info, error) {

	// Give 30 seconds to info command
	runctx, stop := context.WithTimeout(ctx, time.Second*30)
	defer stop()

	out, err := s.runner.Run(runctx, "info", svn.InfoArgs(target)...)

	if err != nil {
		return svn.Info{}, fmt.Errorf("info %s: %w", target, err)
	}
	return svn.ParseInfo(out)
}

func (s *SVNService) Commit(repo string) {

}

func (s *SVNService) Checkout(repo string) {

}
