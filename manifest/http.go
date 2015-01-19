package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/zenazn/goji/web"

	"github.com/dockpit/assert"
	"github.com/dockpit/assert/strategy"
	"github.com/dockpit/pit/config"
)

// An assert error simply denotes the failure of an assert
// during the test and the integrity of the program is fine
type AssertError struct{ err string }

func (e AssertError) Error() string { return e.err }

type Pair struct {
	Name       string
	Request    *http.Request
	Response   *http.Response
	While      []While
	Given      map[string]Given
	Archetypes []*strategy.Archetype
}

func NewPairFromData(data *CaseData, cdata *ManifestData) (*Pair, error) {

	//create request from data
	req, err := http.NewRequest(data.When.Method, data.When.Path, strings.NewReader(data.When.Body))
	if err != nil {
		return nil, err
	}

	//add headers to request
	req.Header = data.When.Headers

	//create expected response from data
	resp := &http.Response{}
	resp.StatusCode = data.Then.StatusCode
	resp.Body = ioutil.NopCloser(strings.NewReader(data.Then.Body))
	resp.Header = data.Then.Headers

	return &Pair{data.Name, req, resp, data.While, data.Given, cdata.Archetypes}, nil
}

func (p *Pair) BelongsToAction(a A) bool {

	//compare HTTP method
	if p.Request.Method == a.Method() {
		return true
	}

	return false
}

func (p *Pair) IsExpectedResponse(resp *http.Response) error {
	var err error

	//get expected content
	c1 := []byte{}
	if p.Response.Body != nil {
		buff := bytes.NewBuffer(nil)
		r := io.TeeReader(p.Response.Body, buff)

		c1, err = ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		p.Response.Body = ioutil.NopCloser(buff)
	}

	//get actualy content
	c2 := []byte{}
	if resp.Body != nil {
		buff := bytes.NewBuffer(nil)
		r := io.TeeReader(resp.Body, buff)

		c2, err = ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		resp.Body = ioutil.NopCloser(buff)
	}

	//assert response code
	if p.Response.StatusCode != resp.StatusCode {
		return AssertError{fmt.Sprintf("StatusCode not equal, expected '%d' but got '%d' with content: '%s'", p.Response.StatusCode, resp.StatusCode, string(c2))}
	}

	//determine content mime type by looking at the example body
	//but if a content-type is set specifically overwrite this
	mimet := http.DetectContentType(c1)
	if ct := p.Response.Header.Get("Content-Type"); ct != "" {
		mimet, _, err = mime.ParseMediaType(ct)
		if err != nil {
			return err
		}
	}

	//create parser using mimetype
	parser := assert.Parser(mimet, p.Archetypes)

	//assert if content follows the example
	err = assert.Follows(c1, c2, parser)
	if err != nil {
		return AssertError{fmt.Sprintf("Content Assertion: %s\n Archetypes: %s", err, p.Archetypes)}
	}

	//check if resp has _at least_ the expected headers
	//@todo switch from _at teast_ to (strict) equal for consitency with content assertion
ExpVals:
	for key, expvals := range p.Response.Header {
		val := resp.Header.Get(key)
		if val == "" {
			return AssertError{fmt.Sprintf("Expected response with '%s' header", key)}
		}

		for _, expval := range expvals {
			if expval == val {
				break ExpVals
			}
		}

		//not any of the expected values
		return AssertError{fmt.Sprintf("Expected '%s' header to have one of the following values: %s, received: %s", key, expvals, val)}
	}

	return nil
}

func (p *Pair) IsSuccessLike() bool {
	return p.Response.StatusCode >= 200 && p.Response.StatusCode < 300
}

func (p *Pair) GenerateHandler() web.Handler {
	return web.HandlerFunc(func(ctx web.C, w http.ResponseWriter, r *http.Request) {

		//add headers
		for key, vals := range p.Response.Header {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}

		//write status code and headers
		w.WriteHeader(p.Response.StatusCode)

		//copy body without consuming the original
		if p.Response.Body != nil {
			buff := bytes.NewBuffer(nil)
			r := io.TeeReader(p.Response.Body, buff)
			io.Copy(w, r)

			//'reset' original response body
			p.Response.Body = ioutil.NopCloser(buff)
		}

	})
}

func (p *Pair) GenerateTest() TestFunc {
	return func(host, dhost string, client *http.Client, conf config.C) error {

		//copy request from example pair
		req := *p.Request

		//parse overwrite host url
		h, err := url.Parse(host)
		if err != nil {
			return err
		}

		//overwrite generated with test specific host/scheme
		req.URL.Host = h.Host
		req.URL.Scheme = h.Scheme

		//do the actual request
		resp, err := client.Do(&req)
		if err != nil {
			return err
		}

		//let the pair assert itself
		if err := p.IsExpectedResponse(resp); err != nil {
			return err
		}

		//ask each mocked dependency if it was called
		for _, while := range p.While {
			ports := conf.PortsForDependency(while.ID)

			//parse host and form endpoint to get recordings from
			dhosturl, err := url.Parse(dhost)
			if err != nil {
				return err
			}

			//create rec url
			//@todo, grabbig the first (seems fundamentally flawed)
			//@see github.com/dockpit/mock/manager/manager.go
			recurl, err := url.Parse(fmt.Sprintf("http://%s:%s/_recordings?case=%s",
				strings.SplitN(dhosturl.Host, ":", 2)[0],
				ports[0].Host,
				url.QueryEscape("healthy"), //@todo get from while
			))

			if err != nil {
				return err
			}

			//request actual recording
			recresp, err := http.Get(recurl.String())
			if err != nil {

				//cant connect to mock?
				return fmt.Errorf("Error while attempt to request dependency: '%s', are the mocks running?", err.Error())
			}

			//receiving something else then 200 is probably bad
			if recresp.StatusCode > 200 {
				return AssertError{fmt.Sprintf("Mock %s recording doesn't have data case %s, returned: %d", while.ID, while.Case, recresp.StatusCode)}
			}

			//decode to get information
			rec := &struct{ Count int }{}
			dec := json.NewDecoder(recresp.Body)
			err = dec.Decode(rec)
			if err != nil {
				return err
			}

			//count mock
			if rec.Count < 1 {
				return AssertError{fmt.Sprintf("Mock %s expected case %s to have been called", while.ID, while.Case)}
			}
		}

		return nil
	}
}

//
//
//
type Action struct {
	pairs  []*Pair
	method string
}

func NewAction(p *Pair) *Action {
	return &Action{
		pairs:  []*Pair{p},
		method: p.Request.Method,
	}
}

func (a *Action) Pairs() []*Pair {
	return a.pairs
}

func (a *Action) AddPair(p *Pair) {
	a.pairs = append(a.pairs, p)
}

func (a *Action) Method() string {
	return a.method
}

func (a *Action) Tests() []TestFunc {
	tests := []TestFunc{}

	for _, p := range a.pairs {
		tests = append(tests, p.GenerateTest())
	}

	return tests
}

//@todo this is deceprated
func (a *Action) Handler(r *http.Request) (web.Handler, error) {

	if r != nil {
		//@todo use the request for more sophistication?
	}

	//pick the first example that specified a success like response
	var ex *Pair
	for _, p := range a.pairs {
		if p.IsSuccessLike() {
			ex = p
			break
		}
	}

	if ex == nil {
		return nil, MockingError(fmt.Sprintf("%s Action has no 'success-like' example", a.method))
	}

	//return a handler that simply returns the example
	return ex.GenerateHandler(), nil
}

//
//
//
type Resource struct {
	pattern string
	cases   []*Pair
}

func NewResource(pattern string, cases ...*Pair) *Resource {
	return &Resource{pattern, cases}
}

func (r *Resource) Pattern() string {
	return r.pattern
}

func (r *Resource) Actions() ([]A, error) {
	actions := []A{}

Cases:
	for _, pair := range r.cases {
		for _, a := range actions {
			if pair.BelongsToAction(a) {

				//add to action
				a.AddPair(pair)

				// and continue to next pair
				continue Cases
			}
		}

		//no existing action was matched, create new one from pair
		actions = append(actions, NewAction(pair))
	}

	return actions, nil
}
