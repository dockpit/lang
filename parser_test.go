package lang_test

import (
	"path/filepath"
	"testing"

	"github.com/bmizerany/assert"

	"github.com/dockpit/lang"
)

func TestParseNotes(t *testing.T) {
	p := lang.NewParser(filepath.Join("docs", "examples", "note_service"))

	cd, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEqual(t, nil, cd)
	assert.Equal(t, 5, len(cd.Resources))
	assert.Equal(t, 3, len(cd.Resources[0].Cases))
	assert.Equal(t, 2, len(cd.Resources[1].Cases))
	assert.Equal(t, 1, len(cd.Resources[2].Cases))
	assert.Equal(t, 0, len(cd.Resources[3].Cases))
	assert.Equal(t, "/notes/note-:note_id-:author_id", cd.Resources[3].Pattern)
	assert.Equal(t, 0, len(cd.Resources[4].Cases))

	//assert when parsing
	assert.Equal(t, "GET", cd.Resources[0].Cases[1].When.Method)
	assert.Equal(t, "/notes", cd.Resources[0].Cases[1].When.Path)
	assert.Equal(t, "en", cd.Resources[0].Cases[1].When.Headers.Get("Accept-Language"))
	assert.Equal(t, "en", cd.Resources[0].Cases[1].When.Headers.Get("accept-language"))
	assert.Equal(t, `[{}, {}]`, cd.Resources[0].Cases[1].When.Body)

	//assert then parsing
	assert.Equal(t, 200, cd.Resources[0].Cases[1].Then.StatusCode)
	assert.Equal(t, "OK", cd.Resources[0].Cases[1].Then.Status)
	assert.Equal(t, "text/html", cd.Resources[0].Cases[1].Then.Headers.Get("Content-Type"))
	assert.Equal(t, `<html></html>`, cd.Resources[0].Cases[1].Then.Body)

}
