package lang_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dockpit/lang"
)

func TestParseNotes(t *testing.T) {
	p := lang.NewParser(filepath.Join(".example", "note_service"))

	md, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEqual(t, nil, md)

	assert.Equal(t, 1, len(md.Archetypes))
	assert.Equal(t, 6, len(md.Resources))
	assert.Equal(t, 1, len(md.Resources[0].Cases))
	assert.Equal(t, 3, len(md.Resources[1].Cases))
	assert.Equal(t, 2, len(md.Resources[2].Cases))
	assert.Equal(t, 1, len(md.Resources[3].Cases))
	assert.Equal(t, 0, len(md.Resources[4].Cases))
	assert.Equal(t, "/notes/note-:note_id-:author_id", md.Resources[4].Pattern)
	assert.Equal(t, 0, len(md.Resources[4].Cases))
	assert.Equal(t, "list of notes", md.Resources[1].Cases[1].Name)

	//test root node
	assert.Equal(t, "/", md.Resources[0].Pattern)
	assert.Equal(t, "show version", md.Resources[0].Cases[0].Name)

	//assert when parsing
	assert.Equal(t, "one user", md.Resources[1].Cases[1].Given["mysql"].Name)
	assert.Equal(t, "GET", md.Resources[1].Cases[1].When.Method)
	assert.Equal(t, "/notes", md.Resources[1].Cases[1].When.Path)
	assert.Equal(t, "en", md.Resources[1].Cases[1].When.Headers.Get("Accept-Language"))
	assert.Equal(t, "en", md.Resources[1].Cases[1].When.Headers.Get("accept-language"))
	assert.Equal(t, `[{}, {}]`, md.Resources[1].Cases[1].When.Body)

	//assert then parsing
	assert.Equal(t, 200, md.Resources[1].Cases[1].Then.StatusCode)
	assert.Equal(t, "OK", md.Resources[1].Cases[1].Then.Status)
	assert.Equal(t, "text/html", md.Resources[1].Cases[1].Then.Headers.Get("Content-Type"))
	assert.Equal(t, `<html></html>`, md.Resources[1].Cases[1].Then.Body)

	//assert while parsing
	assert.Equal(t, "github.com/dockpit/ex-store-orders", md.Resources[1].Cases[1].While[0].ID)
	assert.Equal(t, "list all orders", md.Resources[1].Cases[1].While[0].Case)
}
