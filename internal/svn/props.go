package svn

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PropRequest sets or deletes one property. On a url it is a commit of its own, so it takes one target and a message.
type PropRequest struct {
	Targets   []string `json:"targets"`
	Name      string   `json:"name"`
	Value     string   `json:"value"`
	Recursive bool     `json:"recursive"` // folders only: apply to everything below as well
	Message   string   `json:"message"`   // needed when the target is a url
}

func PropListArgs(target string) ([]string, error) {
	t, err := targets(target)
	if err != nil {
		return nil, err
	}
	return append([]string{"--xml", "-v"}, t...), nil
}

// propArgs puts the name and the value after "--", so a value that starts with a dash is not read as an option.
func propArgs(req PropRequest, withValue bool) ([]string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, " \t\n") {
		return nil, fmt.Errorf("property name %q: %w", req.Name, ErrInvalidRequest)
	}
	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("property %s needs a target: %w", name, ErrInvalidRequest)
	}
	remote := false
	for _, t := range req.Targets {
		if requireURLs(t) == nil {
			remote = true
		}
	}
	var args []string
	if remote {
		if len(req.Targets) != 1 || req.Recursive {
			return nil, fmt.Errorf("a property on the server is set on one item, not recursively: %w", ErrInvalidRequest)
		}
		if err := requireMessage(req.Message); err != nil {
			return nil, err
		}
		args = append(args, "-m", req.Message)
	}
	if req.Recursive {
		args = append(args, "-R")
	}
	args = append(args, "--", name)
	if withValue {
		args = append(args, req.Value)
	}
	for _, t := range req.Targets {
		args = append(args, pegSafe(t))
	}
	return args, nil
}

func PropSetArgs(req PropRequest) ([]string, error) { return propArgs(req, true) }

func PropDelArgs(req PropRequest) ([]string, error) { return propArgs(req, false) }

// ParsePropList reads svn proplist --xml -v; svn sends a value that is not valid XML text in base64.
func ParsePropList(data []byte) ([]Property, error) {
	var doc struct {
		Targets []struct {
			Props []struct {
				Name     string `xml:"name,attr"`
				Encoding string `xml:"encoding,attr"`
				Value    string `xml:",chardata"`
			} `xml:"property"`
		} `xml:"target"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn proplist: %w", err)
	}
	props := []Property{}
	for _, t := range doc.Targets {
		for _, p := range t.Props {
			v := p.Value
			if p.Encoding == "base64" {
				raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v))
				if err != nil {
					return nil, fmt.Errorf("parse svn proplist %s: %w", p.Name, err)
				}
				v = string(raw)
			}
			props = append(props, Property{Name: p.Name, Value: v})
		}
	}
	return props, nil
}

// AddIgnore returns svn:ignore with name appended once; the property is one pattern per line.
func AddIgnore(current, name string) string {
	lines := strings.Split(strings.TrimRight(current, "\r\n"), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == name {
			return current
		}
	}
	if strings.TrimSpace(current) == "" {
		return name + "\n"
	}
	return strings.TrimRight(current, "\r\n") + "\n" + name + "\n"
}

// Mucc is svnmucc, which changes the repository directly in one commit; svn propset does not take a url with a
// message, so a property on the server is set through it.
type Mucc interface {
	Mucc(ctx context.Context, args ...string) ([]byte, error)
}

// MuccPropArgs sets (with valueFile, read by propsetf so any value is safe) or deletes (valueFile empty) one property
// on a url, as one commit with message.
func MuccPropArgs(req PropRequest, valueFile string) ([]string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || strings.HasPrefix(name, "-") || len(req.Targets) != 1 {
		return nil, fmt.Errorf("server property %q: %w", req.Name, ErrInvalidRequest)
	}
	if err := requireURLs(req.Targets[0]); err != nil {
		return nil, err
	}
	if err := requireMessage(req.Message); err != nil {
		return nil, err
	}
	args := []string{"-m", req.Message}
	if valueFile != "" {
		return append(args, "propsetf", name, valueFile, req.Targets[0]), nil
	}
	return append(args, "propdel", name, req.Targets[0]), nil
}

// ParseMucc reads the revision from svnmucc's "r12 committed by ..." line.
func ParseMucc(out []byte) (int64, error) {
	for line := range strings.Lines(string(out)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "r"); ok {
			if n, _, ok := strings.Cut(rest, " committed"); ok {
				return strconv.ParseInt(n, 10, 64)
			}
		}
	}
	return 0, fmt.Errorf("parse svnmucc: no revision in %q", strings.TrimSpace(string(out)))
}

// IsURL tells a url target from a working copy path.
func IsURL(target string) bool { return requireURLs(target) == nil }
