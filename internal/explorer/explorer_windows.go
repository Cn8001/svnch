package explorer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

type entry struct {
	shell, arg string // the class's shell key, and %1 for a clicked item or %V for an open folder's background
	items      []item
}

// path is the submenu's key.
func (e entry) path() string { return e.shell + `\svnch` }

// keys covers a right-click on a folder, on the empty space inside an open folder, and on any file.
func keys() []entry {
	return []entry{
		{`Software\Classes\Directory\shell`, "%1", folderItems()},
		{`Software\Classes\Directory\Background\shell`, "%V", folderItems()},
		{`Software\Classes\*\shell`, "%1", fileItems()},
	}
}

// oldTop are the entries an earlier svnch put beside the submenu; Enable and Disable clear them away.
var oldTop = []string{"svnch1update", "svnch2commit", "svnch3checkout"}

// ecfSeparatorBefore is Explorer's CommandFlags bit for a line above a subcommand.
const ecfSeparatorBefore = 0x20

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
	if err := m.writeIcons(); err != nil {
		return err
	}
	for _, k := range keys() {
		// An empty SubCommands value makes the entry a submenu filled from its shell subkey.
		for name, value := range map[string]string{"MUIVerb": "svnch", "Icon": m.exe, "SubCommands": ""} {
			if err := set(k.path(), name, value); err != nil {
				return err
			}
		}
		for _, it := range k.items {
			p := k.path() + `\shell\` + it.key
			for name, value := range map[string]string{"MUIVerb": it.label, "Icon": m.iconPath(it.icon)} {
				if err := set(p, name, value); err != nil {
					return err
				}
			}
			if it.sep {
				if err := setFlags(p, ecfSeparatorBefore); err != nil {
					return err
				}
			}
			if err := set(p+`\command`, "", m.command(it.action, k.arg)); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeIcons copies the menu's icons out of the program, replacing older ones, since Explorer reads them from disk.
func (m *Menu) writeIcons() error {
	if err := os.MkdirAll(m.iconDir, 0o755); err != nil {
		return fmt.Errorf("explorer menu icons: %w", err)
	}
	files, err := fs.Glob(icons, "icons/*.ico")
	if err != nil {
		return fmt.Errorf("explorer menu icons: %w", err)
	}
	for _, f := range files {
		data, err := icons.ReadFile(f)
		if err != nil {
			return fmt.Errorf("explorer menu icons: %w", err)
		}
		if err := os.WriteFile(filepath.Join(m.iconDir, path.Base(f)), data, 0o644); err != nil {
			return fmt.Errorf("explorer menu icons: %w", err)
		}
	}
	return nil
}

func (m *Menu) Disable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, k := range keys() {
		paths := []string{k.path()}
		for _, top := range oldTop {
			paths = append(paths, k.shell+`\`+top)
		}
		for _, p := range paths {
			if err := deleteTree(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// Repair points the menu at this copy when the copy it named is gone, as after moving the program, and rewrites a menu
// from an earlier svnch (a single entry, or a submenu without the top entries) into this one. A menu for another copy that still exists is
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
	cmd, found, err := readDefault(folder.path() + `\shell\` + folder.items[0].key + `\command`)
	if err != nil {
		return "", false, false, err
	}
	if found {
		// A menu that still has the old top entries beside it is from the version before the single submenu.
		_, top, err := readDefault(folder.shell + `\` + oldTop[0] + `\command`)
		return exeOf(cmd), true, top, err
	}
	// Earlier menus: a submenu keyed 1open or 01open, or the single entry with its own command.
	for _, old := range []string{folder.path() + `\shell\1open\command`, folder.path() + `\shell\01open\command`, folder.path() + `\command`} {
		cmd, found, err = readDefault(old)
		if err != nil || found {
			return exeOf(cmd), found, found, err
		}
	}
	return "", false, false, nil
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

func setFlags(path string, flags uint32) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("write explorer menu %s: %w", path, err)
	}
	defer k.Close()
	if err := k.SetDWordValue("CommandFlags", flags); err != nil {
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
