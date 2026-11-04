package svn

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrNotCollection means the address answered but is not an Apache SVNParentPath index, so svn's own error stands.
var ErrNotCollection = errors.New("not a collection of repositories")

const collectionTitle = "Collection of Repositories"

// ParseCollection reads the page mod_dav_svn serves at an SVNParentPath root (with SVNListParentPath on), in both its
// plain HTML form and the XML form used when the server sets SVNIndexXSLT.
func ParseCollection(page []byte) ([]ListEntry, error) {
	d := xml.NewDecoder(strings.NewReader(string(page)))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity

	var hrefs []string
	var title strings.Builder
	marked, inTitle := false, false
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse repository index: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "title", "h2":
				inTitle = true
			case "index":
				// A repository's own index always carries its revision; the list of repositories has none.
				marked = marked || attr(t, "name") == collectionTitle || (attr(t, "rev") == "" && attr(t, "base") == "")
			case "a", "dir":
				hrefs = append(hrefs, attr(t, "href"))
			}
		case xml.EndElement:
			inTitle = false
		case xml.CharData:
			if inTitle && title.Len() < 120 {
				title.WriteString(strings.TrimSpace(string(t)))
			}
			marked = marked || (inTitle && strings.TrimSpace(string(t)) == collectionTitle)
		}
	}
	if !marked {
		// The title is what the user can tell us about a server whose index svnch does not read yet.
		return nil, fmt.Errorf("its page %q is not a list of repositories: %w", title.String(), ErrNotCollection)
	}

	entries := []ListEntry{}
	for _, h := range hrefs {
		if name, ok := repoName(h); ok {
			entries = append(entries, ListEntry{Name: name, Kind: "dir"})
		}
	}
	return entries, nil
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

// repoName keeps only links to a direct child folder; the page also links to the Subversion website in its footer.
func repoName(href string) (string, bool) {
	name, err := url.PathUnescape(strings.TrimSuffix(href, "/"))
	if err != nil || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/?#:") {
		return "", false
	}
	return name, true
}

// ListCollection is the fallback for an http(s) address that svn list refuses because it is the folder holding the
// repositories rather than a repository; svn has no command for that, the web server does.
func ListCollection(ctx context.Context, client *http.Client, address, username, password string) ([]ListEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(address, "/")+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("repository index %s: %w", address, err)
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("repository index %s: %w", address, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("repository index %s: %w", address, ErrAuthFailed)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("repository index %s: %s: %w", address, resp.Status, ErrNotCollection)
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("repository index %s: %w", address, err)
	}
	return ParseCollection(page)
}
