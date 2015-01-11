package manifest_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/bmizerany/assert"

	. "github.com/dockpit/lang/manifest"
)

// test JSON -> *ManifestData
func TestFactoryLoading(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	//gven a new factory
	f := NewFactory()

	//when we load json example
	data, err := f.Load(filepath.Join(wd, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}

	//assert manifest data
	assert.Equal(t, "auth", data.Name)

	//assert resource
	assert.Equal(t, "/users", data.Resources[0].Pattern)

	//assert cases: given
	assert.Equal(t, "some users", data.Resources[0].Cases[0].Given["mongodb"].Name)
	assert.Equal(t, "some messages", data.Resources[0].Cases[0].Given["nsq"].Name)

	//assert cases: when
	assert.Equal(t, "GET", data.Resources[0].Cases[0].When.Method)
	assert.Equal(t, "/users", data.Resources[0].Cases[0].When.Path)

	//assert cases: then
	assert.Equal(t, 200, data.Resources[0].Cases[0].Then.StatusCode)
	assert.Equal(t, `[{"id": "32"}]`, data.Resources[0].Cases[0].Then.Body)

	//assert cases: while
	assert.Equal(t, "GET", data.Resources[0].Cases[0].While[0].Method)
	assert.Equal(t, "/users", data.Resources[0].Cases[0].While[0].Path)
	assert.Equal(t, "github.com/dockpit/ex-store-customers", data.Resources[0].Cases[0].While[0].ID)
}

// test JSON -> ManifestData -> Manifest
func TestFactoryDraft(t *testing.T) {
	var m M
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	//gven a new factory
	f := NewFactory()

	//when we load json example
	m, err = f.Draft(filepath.Join(wd, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}

	//assert manifest data
	assert.Equal(t, "auth", m.Name())

	//assert dependencies
	deps, err := m.Dependencies()
	if err != nil {
		t.Fatal(err)
	}

	//should have 1 dependency with empty list of cases
	assert.Equal(t, 1, len(deps))
	assert.Equal(t, []string{}, deps["github.com/dockpit/ex-store-customers"])

	//assert states
	states, err := m.States()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(states))
	assert.Equal(t, []string{"some users"}, states["mongodb"])

	//assert resource
	resources, err := m.Resources()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "/users", resources[0].Pattern())

	//assert actions
	actions, err := resources[0].Actions()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "GET", actions[0].Method())
	assert.Equal(t, "POST", actions[1].Method())

	//assert pair data
	pairs := actions[0].Pairs()
	b, err := ioutil.ReadAll(pairs[0].Request.Body)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []byte("{}"), b)
	assert.Equal(t, "application/json", pairs[0].Request.Header.Get("Content-Type"))
	assert.Equal(t, "application/html", pairs[0].Response.Header.Get("Content-Type"))

}
