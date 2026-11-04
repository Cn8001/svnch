package svn

import (
	"errors"
	"testing"
)

func TestParseCollection_ReadsXmlIndex(t *testing.T) {
	// Arrange
	page := `<?xml version="1.0"?><?xml-stylesheet type="text/xsl" href="/svnindex.xsl"?>
<svn version="1.14.2"><index name="Collection of Repositories" path="/"><dir name="tools" href="tools/" /></index></svn>`

	// Act
	got, err := ParseCollection([]byte(page))

	// Assert
	if err != nil || len(got) != 1 || got[0].Name != "tools" {
		t.Errorf("got (%+v, %v)", got, err)
	}
}

func TestParseCollection_EmptyCollectionIsNotAnError(t *testing.T) {
	// Arrange
	page := `<html><head><title>Collection of Repositories</title></head><body><ul></ul></body></html>`

	// Act
	got, err := ParseCollection([]byte(page))

	// Assert
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got (%#v, %v), want empty", got, err)
	}
}

func TestParseCollection_RepositoryFolderPageIsRejected(t *testing.T) {
	// Arrange
	page := `<html><head><title>repo - Revision 4: /</title></head><body><h2>repo - Revision 4: /</h2><ul><li><a href="trunk/">trunk/</a></li></ul></body></html>`

	// Act
	_, err := ParseCollection([]byte(page))

	// Assert
	if !errors.Is(err, ErrNotCollection) {
		t.Errorf("err = %v, want ErrNotCollection", err)
	}
}

func TestParseCollection_XmlIndexWithoutRevisionIsCollection(t *testing.T) {
	// Arrange
	page := `<?xml version="1.0"?><svn version="1.14.5"><index path="/"><dir name="Game" href="Game/" /><dir name="Tools" href="Tools/" /></index></svn>`

	// Act
	got, err := ParseCollection([]byte(page))

	// Assert
	if err != nil || len(got) != 2 {
		t.Errorf("got (%+v, %v)", got, err)
	}
}

func TestParseCollection_XmlRepositoryIndexIsRejected(t *testing.T) {
	// Arrange
	page := `<?xml version="1.0"?><svn version="1.14.5"><index rev="42" path="/" base="Game"><dir name="trunk" href="trunk/" /></index></svn>`

	// Act
	_, err := ParseCollection([]byte(page))

	// Assert
	if !errors.Is(err, ErrNotCollection) {
		t.Errorf("err = %v, want ErrNotCollection", err)
	}
}
