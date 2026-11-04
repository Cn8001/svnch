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

// Checkout has no timeout of its own because a large repository can take many minutes; the caller cancels through ctx.
func (s *SVNService) Checkout(ctx context.Context, req svn.CheckoutRequest) (int64, error) {
	args, err := svn.CheckoutArgs(req)
	if err != nil {
		return 0, err
	}
	out, err := s.runner.Run(ctx, "checkout", args...)
	if err != nil {
		return 0, fmt.Errorf("checkout %s: %w", req.URL, err)
	}
	return svn.ParseCheckout(out)
}
