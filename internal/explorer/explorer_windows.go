package explorer

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

type entry struct {
	path, arg string
	items     []item
}

// keys covers a right-click on a folder, on the empty space inside an open folder, and on any file.
func keys() []entry {
	return []entry{
		{`Software\Classes\Directory\shell\svnch`, "%1", folderItems()},
		{`Software\Classes\Directory\Background\shell\svnch`, "%V", folderItems()},
		{`Software\Classes\*\shell\svnch`, "%1", fileItems()},
	}
}

func (m *Menu) Status(ctx context.Context) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	_, ok, _, err := m.registered()
	if err != nil {
		return Status{}, err
	}
	return Status{Supported: true, Enabled: ok}, nil
}

// Enable rewrites the whole menu, so a menu from an older svnch never keeps entries this one dropped.
func (m *Menu) Enable(ctx context.Context) error {
	if err := m.Disable(ctx); err != nil {
		return err
	}
	for _, k := range keys() {
		// An empty SubCommands value makes the entry a submenu filled from its shell subkey.
		for name, value := range map[string]string{"MUIVerb": "svnch", "Icon": m.exe, "SubCommands": ""} {
			if err := set(k.path, name, value); err != nil {
				return err
			}
		}
		for _, it := range k.items {
			p := k.path + `\shell\` + it.key
			if err := set(p, "MUIVerb", it.label); err != nil {
				return err
			}
			if err := set(p+`\command`, "", m.command(it.action, k.arg)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Menu) Disable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, k := range keys() {
		if err := deleteTree(k.path); err != nil {
			return err
		}
	}
	return nil
}

// Repair points the menu at this copy when the copy it named is gone, as after moving the program, and turns the
// single "Open with svnch" entry of earlier versions into the submenu. A menu for another copy that still exists is
// left alone, so a test build does not take it over.
func (m *Menu) Repair(ctx context.Context) error {
	exe, ok, legacy, err := m.registered()
	if err != nil || !ok {
		return err
	}
	if legacy {
		return m.Enable(ctx)
	}
	if exe == m.exe {
		return nil
	}
	if _, err := os.Stat(exe); err == nil {
		return nil
	}
	return m.Enable(ctx)
}

// registered reads the program from the folder menu's first entry, or from the single-entry layout svnch used before.
func (m *Menu) registered() (exe string, ok, legacy bool, err error) {
	folder := keys()[0]
	cmd, found, err := readDefault(folder.path + `\shell\` + folder.items[0].key + `\command`)
	if err != nil || found {
		return exeOf(cmd), found, false, err
	}
	cmd, found, err = readDefault(folder.path + `\command`)
	return exeOf(cmd), found, found, err
}

func readDefault(path string) (string, bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read explorer menu: %w", err)
	}
	defer k.Close()
	v, _, err := k.GetStringValue("")
	if err != nil {
		return "", false, fmt.Errorf("read explorer menu: %w", err)
	}
	return v, true, nil
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

// deleteTree removes a key with everything under it; the registry only deletes keys that have no subkeys.
func deleteTree(path string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove explorer menu %s: %w", path, err)
	}
	names, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		return fmt.Errorf("remove explorer menu %s: %w", path, err)
	}
	for _, n := range names {
		if err := deleteTree(path + `\` + n); err != nil {
			return err
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("remove explorer menu %s: %w", path, err)
	}
	return nil
}
