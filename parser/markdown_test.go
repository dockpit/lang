package parser_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dockpit/lang/parser"
)

func TestParse(t *testing.T) {
	p := parser.NewMarkdown(filepath.Join(".example_markdown"))

	md, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}

	//assert collection lengths
	assert.Len(t, md.Resources, 2)
	assert.Len(t, md.Resources[0].Cases, 2)
	assert.Len(t, md.Resources[0].Cases[0].Given, 2)
	assert.Len(t, md.Resources[0].Cases[0].While, 1)

	//assert given content
	assert.Equal(t, "a single user", md.Resources[0].Cases[0].Given["mongo"].Name)
	assert.Equal(t, "no cached users", md.Resources[0].Cases[0].Given["redis"].Name)
	assert.Equal(t, "authorized", md.Resources[0].Cases[0].While[0].Case)
	assert.Equal(t, "pit-token", md.Resources[0].Cases[0].While[0].ID)

	//assert when parsing
	assert.Equal(t, "/users/31", md.Resources[0].Cases[0].When.Path)
	assert.Equal(t, "GET", md.Resources[0].Cases[0].When.Method)
	assert.Equal(t, `{"id": 21}`, md.Resources[0].Cases[0].When.Body)
	assert.Equal(t, `34e5ff21`, md.Resources[0].Cases[0].When.Headers.Get("X-Parse-Application-Id"))

	//assert then parsing
	assert.Equal(t, "OK", md.Resources[0].Cases[0].Then.Status)
	assert.Equal(t, 200, md.Resources[0].Cases[0].Then.StatusCode)
	assert.Equal(t, `{"id": "21", "username": "coolgirl21"}`, md.Resources[0].Cases[0].Then.Body)

}
