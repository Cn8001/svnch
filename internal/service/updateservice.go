package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type Updater interface {
	CheckAndInstall(ctx context.Context) error
}

type UpdateService struct {
	ctx     context.Context
	mu      sync.Mutex
	updater Updater
}

var (
	ErrUpdatesOff = fmt.Errorf("updates are off")
)

func NewUpdateService(ctx context.Context) (s *UpdateService, enable func(Updater)) {
	s = &UpdateService{ctx: ctx}
	return s, func(u Updater) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.updater = u
	}
}

func (s *UpdateService) Check() error {
	s.mu.Lock()
	u := s.updater
	s.mu.Unlock()

	if u == nil {
		return ErrUpdatesOff
	}

	go func() {
		if err := u.CheckAndInstall(s.ctx); err != nil {
			slog.Warn("update", "error", err)
		}
	}()
	return nil
}
