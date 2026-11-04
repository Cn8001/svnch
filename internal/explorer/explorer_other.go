//go:build !windows

package explorer

import "context"

func (m *Menu) Status(ctx context.Context) (Status, error) { return Status{}, nil }

func (m *Menu) Enable(ctx context.Context) error { return ErrUnsupported }

func (m *Menu) Disable(ctx context.Context) error { return ErrUnsupported }

func (m *Menu) Repair(ctx context.Context) error { return nil }
