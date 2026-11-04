// Package appinfo is what the About dialog and the updater know about the app. Name, version and copyright come
// from build/config.yml, the file every build already reads, so the version is written in one place.
package appinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

type Info struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Copyright   string    `json:"copyright"`
	Go          string    `json:"go"`
	Wails       string    `json:"wails"`
	Platform    string    `json:"platform"`
	Notices     string    `json:"notices"`  // THIRD_PARTY_NOTICES.md
	Licenses    []License `json:"licenses"` // THIRD_PARTY_LICENSES.json, written by scripts/licenses
}

// License is one shipped package's licence in full, with its NOTICE where it has one: what MIT, BSD, ISC and
// Apache-2.0 ask to go with every copy.
type License struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	License string `json:"license"`
	Text    string `json:"text"`
	Notice  string `json:"notice,omitempty"`
}

// FromConfig reads the info block of build/config.yml. It is a few quoted keys, so a line match does without a YAML parser.
func FromConfig(config []byte) Info {
	var out Info
	inInfo := false
	for line := range strings.Lines(string(config)) {
		line = strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			inInfo = strings.HasPrefix(line, "info:")
			continue
		}
		key, value, ok := quotedField(trimmed)
		if !inInfo || !ok {
			continue
		}
		switch key {
		case "productName":
			out.Name = value
		case "version":
			out.Version = value
		case "description":
			out.Description = value
		case "companyName":
			out.Author = value
		case "copyright":
			out.Copyright = value
		}
	}
	return out
}

func quotedField(line string) (key, value string, ok bool) {
	key, rest, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, `"`) {
		return "", "", false
	}
	value, _, ok = strings.Cut(rest[1:], `"`)
	return key, value, ok
}

func (i Info) WithBuild() Info {
	i.Go = runtime.Version()
	i.Platform = runtime.GOOS + "/" + runtime.GOARCH
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/wailsapp/wails/v3" {
				i.Wails = d.Version
			}
		}
	}
	return i
}
