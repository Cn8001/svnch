package explorer

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const label = "Open with svnch"

type entry struct{ path, arg string }

// keys lists a right-click on a folder first, then one on the empty space inside an open folder.
func keys() []entry {
	return []entry{
		{`Software\Classes\Directory\shell\svnch`, "%1"},
		{`Software\Classes\Directory\Background\shell\svnch`, "%V"},
	}
}

func (m *Menu) Status(ctx context.Context) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	_, ok, err := m.registered()
	if err != nil {
		return Status{}, err
	}
	return Status{Supported: true, Enabled: ok}, nil
}

func (m *Menu) Enable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, k := range keys() {
		if err := set(k.path, "", label); err != nil {
			return err
		}
		if err := set(k.path, "Icon", m.exe); err != nil {
			return err
		}
		if err := set(k.path+`\command`, "", m.command(k.arg)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Menu) Disable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, k := range keys() {
		for _, p := range []string{k.path + `\command`, k.path} {
			if err := registry.DeleteKey(registry.CURRENT_USER, p); err != nil && !errors.Is(err, registry.ErrNotExist) {
				return fmt.Errorf("remove explorer menu %s: %w", p, err)
			}
		}
	}
	return nil
}

// Repair points an existing entry at this copy when the copy it named is gone, as after moving the program; an entry
// for another copy that still exists is left alone, so a test build does not take over the menu.
func (m *Menu) Repair(ctx context.Context) error {
	exe, ok, err := m.registered()
	if err != nil || !ok || exe == m.exe {
		return err
	}
	if _, err := os.Stat(exe); err == nil {
		return nil
	}
	return m.Enable(ctx)
}

func (m *Menu) registered() (string, bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, keys()[0].path+`\command`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read explorer menu: %w", err)
	}
	defer k.Close()
	cmd, _, err := k.GetStringValue("")
	if err != nil {
		return "", false, fmt.Errorf("read explorer menu: %w", err)
	}
	return exeOf(cmd), true, nil
}

func set(path, name, value string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("write explorer menu %s: %w", path, err)
	}
	defer k.Close()
	if err := k.SetStringValue(name, value); err != nil {
		return fmt.Errorf("write explorer menu %s: %w", path, err)
	}
	return nil
}
