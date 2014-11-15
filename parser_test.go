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

}
