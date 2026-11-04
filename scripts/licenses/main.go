// Command licenses writes THIRD_PARTY_LICENSES.json: the full licence text, and NOTICE where there is one, of every
// package svnch ships. MIT, BSD and ISC ask for their copyright and licence to go with every copy, and Apache-2.0 for
// its NOTICE too, so the list is what is shipped, not only what svnch chose: the Go modules compiled in, on every
// platform, and the frontend packages vite bundles. Run it after a dependency changes:
//
//	go run ./scripts/licenses
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Cn8001/svnch/internal/appinfo"
)

type License = appinfo.License

const own = "github.com/Cn8001/svnch"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "licenses:", err)
		os.Exit(1)
	}
}

func run() error {
	goList, err := goModules()
	if err != nil {
		return err
	}
	npmList, err := npmPackages(goList)
	if err != nil {
		return err
	}
	all := append(goList, npmList...)
	var missing []string
	for _, l := range all {
		if strings.TrimSpace(l.Text) == "" {
			missing = append(missing, l.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("no licence text for %s: add it to borrowed", strings.Join(missing, ", "))
	}
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile("THIRD_PARTY_LICENSES.json", append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("THIRD_PARTY_LICENSES.json: %d Go modules, %d frontend packages\n", len(goList), len(npmList))
	return nil
}

// goModules are the modules compiled into the binary for any platform it is built for; go-ole, say, only goes into
// the Windows one.
func goModules() ([]License, error) {
	type mod struct{ path, version, dir string }
	seen := map[string]mod{}
	for _, goos := range []string{"linux", "windows", "darwin"} {
		cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}", ".")
		cmd.Env = append(os.Environ(), "GOOS="+goos)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list for %s: %w", goos, err)
		}
		for line := range strings.Lines(strings.TrimSpace(string(out))) {
			f := strings.Split(strings.TrimSpace(line), "\t")
			if len(f) != 3 || strings.HasPrefix(f[0], own) {
				continue
			}
			seen[f[0]] = mod{f[0], f[1], f[2]}
		}
	}
	// The standard library and runtime are in every Go binary, under Go's own licence, and go list names no module for them.
	root, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return nil, fmt.Errorf("go env GOROOT: %w", err)
	}
	version, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return nil, fmt.Errorf("go env GOVERSION: %w", err)
	}
	seen["Go standard library"] = mod{"Go standard library", strings.TrimSpace(string(version)), strings.TrimSpace(string(root))}

	var list []License
	for _, m := range seen {
		text, notice := licenseFiles(m.dir)
		list = append(list, License{Name: m.path, Version: m.version, License: spdx(text), Text: text, Notice: notice})
	}
	slices.SortFunc(list, func(a, b License) int { return strings.Compare(a.Name, b.Name) })
	return list, nil
}

// npmPackages are the frontend's production packages plus svelte, which sits in devDependencies because it is a
// compiler but still puts its runtime into the bundle. Dev tools such as vite do not ship.
func npmPackages(goList []License) ([]License, error) {
	// The Wails runtime is published without its licence file and ships under Wails' own MIT.
	borrowed := map[string]string{"@wailsio/runtime": "github.com/wailsapp/wails/v3"}

	var nodes []struct {
		Name    string          `json:"name"`
		Version string          `json:"version"`
		Path    string          `json:"path"`
		License json.RawMessage `json:"license"`
	}
	for _, selector := range []string{".prod", "#svelte"} {
		cmd := exec.Command("npm", "query", selector)
		cmd.Dir = "frontend"
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("npm query %s: %w", selector, err)
		}
		var found []struct {
			Name    string          `json:"name"`
			Version string          `json:"version"`
			Path    string          `json:"path"`
			License json.RawMessage `json:"license"`
		}
		if err := json.Unmarshal(out, &found); err != nil {
			return nil, fmt.Errorf("parse npm query %s: %w", selector, err)
		}
		nodes = append(nodes, found...)
	}

	seen := map[string]bool{}
	var list []License
	for _, n := range nodes {
		key := n.Name + "@" + n.Version
		if n.Name == "" || filepath.Base(n.Path) == "frontend" || seen[key] {
			continue
		}
		seen[key] = true
		text, notice := licenseFiles(n.Path)
		if text == "" {
			if from, ok := borrowed[n.Name]; ok {
				for _, g := range goList {
					if g.Name == from {
						text = g.Text
					}
				}
			}
		}
		list = append(list, License{Name: n.Name, Version: n.Version, License: declared(n.License), Text: text, Notice: notice})
	}
	slices.SortFunc(list, func(a, b License) int { return strings.Compare(a.Name, b.Name) })
	return list, nil
}

// licenseFiles reads a package's licence and NOTICE, whatever their case and extension (LICENSE, LICENSE.md,
// licence.txt, COPYING).
func licenseFiles(dir string) (text, notice string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		up := strings.ToUpper(e.Name())
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		// Some files start with a byte order mark, which would show as a glyph.
		raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
		switch {
		case text == "" && (strings.HasPrefix(up, "LICEN") || strings.HasPrefix(up, "COPYING")):
			text = string(bytes.TrimSpace(raw))
		case notice == "" && strings.HasPrefix(up, "NOTICE"):
			notice = string(bytes.TrimSpace(raw))
		}
	}
	return text, notice
}

// declared is package.json's licence, a string or an old {type} object.
func declared(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var o struct{ Type string }
	if json.Unmarshal(raw, &o) == nil {
		return o.Type
	}
	return ""
}

// spdx names a Go module's licence from its text; Go has no field for it.
func spdx(text string) string {
	switch {
	case strings.Contains(text, "MIT License"), strings.Contains(text, "Permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(text, "Apache License"):
		return "Apache-2.0"
	case strings.Contains(text, "Neither the name"), strings.Contains(text, "names of its contributors"):
		return "BSD-3-Clause"
	case strings.Contains(text, "Redistribution and use in source and binary forms"):
		return "BSD-2-Clause"
	}
	return ""
}
