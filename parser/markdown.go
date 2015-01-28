package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dockpit/lang/manifest"

	"github.com/russross/blackfriday"
)

var ResourceExp = regexp.MustCompile(`^(/.*)`)
var VariableExp = regexp.MustCompile(`(\(.*?\))`)
var CaseExp = regexp.MustCompile(`^'(.*)'$`)
var WhenExp = regexp.MustCompile(`^when:$`)
var ThenExp = regexp.MustCompile(`^then:$`)
var GivenStateExp = regexp.MustCompile(`^(.*).*has:.*'(.*)'.*$`)
var GivenDepExp = regexp.MustCompile(`^(.*).*responds:.*'(.*)'.*$`)

// A markdown to html rendered that stores
// a JSON structure for
// @todo add line numbers to errors
type withJSON struct {
	blackfriday.Renderer
	Manifest *manifest.ManifestData
	Errors   chan (error)

	openResource *manifest.ResourceData
	openCase     *manifest.CaseData

	recorder             *bytes.Buffer
	lastParagraph        []byte
	lastTextBeforeMarker int
	lastTextAfterMarker  int
}

func renderer(md *manifest.ManifestData) *withJSON {
	return &withJSON{
		Renderer: blackfriday.HtmlRenderer(0, "", ""),
		Manifest: md,
		Errors:   make(chan error),
	}
}

func (r *withJSON) record() {
	r.recorder = bytes.NewBuffer(nil)
}

func (r *withJSON) rewind() []byte {
	b := r.recorder.Bytes()
	r.recorder = nil
	return b
}

func (p *withJSON) ParseHTTPMessage(r io.ReadCloser, fpath string) (string, http.Header, string, error) {

	inbody := false
	rline := ""
	hlines := []string{}
	body := ""
	headers := make(http.Header)

	s := bufio.NewScanner(r)
	for s.Scan() {
		if !inbody {
			//first line should be the request line
			if rline == "" {
				rline = s.Text()
				continue
			}

			//empty line indicates that we will be parsing the body
			if s.Text() == "" {
				inbody = true
				continue
			}

			//add as headers
			hlines = append(hlines, s.Text())
		} else {
			//res is assumed to be body
			body += s.Text()
		}
	}

	//check/parse header format
	for _, h := range hlines {
		hp := strings.SplitN(h, ":", 2)
		if len(hp) != 2 {
			return rline, headers, body, UnexpectedHeaderLineError(fpath, h)
		}

		headers.Add(http.CanonicalHeaderKey(hp[0]), strings.TrimSpace(hp[1]))
	}

	return rline, headers, body, nil
}

//check method format
func (p *withJSON) parseMethod(input string) (string, error) {
	method := ""
	for _, m := range ValidHTTPMethods {
		if m == input {
			method = m
			break
		}
	}

	if method == "" {
		return "", fmt.Errorf("unexpected method %s", input)
	}

	return method, nil
}

func (p *withJSON) parsePath(input string) (string, error) {
	if !path.IsAbs(input) {
		return "", fmt.Errorf("Not an absolute path provided: %s", input)
	}

	return input, nil
}

// parses a when file loosely based on the http request spec
func (p *withJSON) parseWhen(r io.ReadCloser, fpath string) (*manifest.When, error) {
	w := &manifest.When{}

	rline, headers, body, err := p.ParseHTTPMessage(r, fpath)
	if err != nil {
		return nil, err
	}

	//request line should be 2 parts seperated by a space
	rlinep := strings.SplitN(rline, " ", 2)
	if len(rlinep) != 2 {
		return nil, UnexpectedRequestLineError(fpath, rline)
	}

	//get method
	w.Method, err = p.parseMethod(rlinep[0])
	if err != nil {
		return nil, UnexpectedRequestLineMethodError(fpath, rlinep[0])
	}

	//check path
	w.Path, err = p.parsePath(rlinep[1])
	if err != nil {
		return nil, UnexpectedRequestLinePathError(fpath, rlinep[1])
	}

	w.Headers = headers
	w.Body = body

	return w, nil
}

// parses a then file loosely based on the format of a standard http message
func (p *withJSON) parseThen(r io.ReadCloser, fpath string) (*manifest.Then, error) {
	t := &manifest.Then{}

	//parse as a standard http message
	rline, headers, body, err := p.ParseHTTPMessage(r, fpath)
	if err != nil {
		return nil, err
	}

	//response line should be 2 parts seperated by a space
	rlinep := strings.SplitN(rline, " ", 2)
	if len(rlinep) != 2 {
		return nil, UnexpectedResponseLineError(fpath, rline)
	}

	//first one should be parseable as int
	code, err := strconv.Atoi(rlinep[0])
	if err != nil {
		return nil, UnexpectedResponseLineCodeError(fpath, rlinep[0], err)
	}

	t.StatusCode = code
	t.Status = rlinep[1]
	t.Headers = headers
	t.Body = body
	return t, nil
}

func (r *withJSON) ParseGiven(in []byte) (map[string]manifest.Given, []manifest.While, error) {
	gs := make(map[string]manifest.Given)
	ws := []manifest.While{}

	s := bufio.NewScanner(bytes.NewBuffer(in))
	for s.Scan() {
		trimmed := strings.TrimSpace(s.Text())

		//dont mind empty lines
		if trimmed == "" {
			continue
		}

		if m := GivenDepExp.FindStringSubmatch(trimmed); m != nil {
			if len(m) != 3 {
				r.Errors <- fmt.Errorf("Unexpected dependency given line: %s", trimmed)
			}

			//'dependency' given
			ws = append(ws, manifest.While{strings.TrimSpace(m[1]), strings.TrimSpace(m[2])})
		} else if m := GivenStateExp.FindStringSubmatch(trimmed); m != nil {
			if len(m) != 3 {
				r.Errors <- fmt.Errorf("Unexpected state given line: %s", trimmed)
			}

			//'state' given
			gs[strings.TrimSpace(m[1])] = manifest.Given{strings.TrimSpace(m[2])}
		} else {
			r.Errors <- fmt.Errorf("Unexpected line in given: %s", trimmed)
		}
	}

	return gs, ws, nil
}

func (r *withJSON) ParseWhen(in []byte) (*manifest.When, error) {
	return r.parseWhen(ioutil.NopCloser(bytes.NewBuffer(in)), "")
}

func (r *withJSON) ParseThen(in []byte) (*manifest.Then, error) {
	return r.parseThen(ioutil.NopCloser(bytes.NewBuffer(in)), "")
}

func (r *withJSON) BlockCode(out *bytes.Buffer, text []byte, lang string) {
	if r.openCase != nil {

		//parse code block as when
		if r.openCase.When.Path == "-" {
			when, err := r.ParseWhen(text)
			if err != nil {
				r.Errors <- err
			} else {
				r.openCase.When = *when
			}

		}

		//parse code block as then
		if r.openCase.Then.Status == "-" {
			then, err := r.ParseThen(text)
			if err != nil {
				r.Errors <- err
			} else {
				r.openCase.Then = *then
			}

		}
	}

	r.Renderer.BlockCode(out, text, lang)
}

func (r *withJSON) Paragraph(out *bytes.Buffer, text func() bool) {
	r.record()
	r.Renderer.Paragraph(out, text)
	r.lastParagraph = r.rewind()
}

func (w *withJSON) AugmentGivenWithLinks(html []byte) []byte {
	str := string(html)

	//match a dependency given line that may include html
	rp := regexp.MustCompile(`([a-zA-Z_-]+)(\s+responds:\s+'[\S]*')`)

	//and wrap it in a link
	return []byte(rp.ReplaceAllString(str, `<a href="{{ .CurrentService }}">$1</a>$2`))
}

func (r *withJSON) BlockQuote(out *bytes.Buffer, text []byte) {
	var err error

	//the first quote on an open case is used for state definition
	c := r.openCase
	if c != nil {

		// first bquote
		if len(c.Given) == 0 && len(c.While) == 0 {

			//get plain text given from last paragraph
			c.Given, c.While, err = r.ParseGiven(r.lastParagraph)
			if err != nil {
				r.Errors <- err
			}

			text = r.AugmentGivenWithLinks(text)
		}
	}

	r.Renderer.BlockQuote(out, text)
}

func (r *withJSON) ToResourcePatternPart(str string) string {
	m := ResourceExp.FindStringSubmatch(str)
	if m == nil {
		return ""
	}

	return string(m[1])
}

func (r *withJSON) ToCaseName(str string) string {
	m := CaseExp.FindStringSubmatch(str)
	if m == nil {
		return ""
	}

	return m[1]
}

func (r *withJSON) IsWhen(str string) bool {
	return WhenExp.MatchString(str)
}

func (r *withJSON) IsThen(str string) bool {
	return ThenExp.MatchString(str)
}

func (r *withJSON) injectA(out *bytes.Buffer, text string) {
	insertBufferAt(out, r.lastTextAfterMarker, []byte(text))
}

func insertBufferAt(out *bytes.Buffer, at int, in []byte) {
	appendix := string(out.Bytes()[at:])
	out.Truncate(at)
	out.Write(in)
	out.Write([]byte(appendix))
}

func (r *withJSON) Header(out *bytes.Buffer, text func() bool, level int, id string) {

	//we care about level 1,2 and 3
	switch level {
	case 1, 2, 3:
		r.record()
	}

	r.Renderer.Header(out, text, level, id)

	switch level {
	case 1, 2, 3:
		title := r.rewind()
		switch level {

		// H1
		case 1:

			//if we have an open resource, close it and append to manifest
			if r.openResource != nil {
				r.openResource = nil
			}

			// h1 is a resource, open a new one
			pattern := r.ToResourcePatternPart(string(title))
			if pattern != "" {
				r.openResource = &manifest.ResourceData{
					Pattern: pattern,
					Cases:   []*manifest.CaseData{},
				}

				r.Manifest.Resources = append(r.Manifest.Resources, r.openResource)
			}

		// H2
		case 2:

			//if we have an open case, close it and add to open resource
			if r.openCase != nil {
				r.openCase = nil
			}

			// h2 is indeed a casename
			cname := r.ToCaseName(string(title))
			if cname != "" {
				if r.openResource == nil {
					r.Errors <- fmt.Errorf("Case outside resource")
				} else {
					r.openCase = &manifest.CaseData{
						Name: cname,
						When: manifest.When{},
						Then: manifest.Then{},
					}

					r.injectA(out, fmt.Sprintf(`&nbsp<a href="">test</a>`))

					r.openResource.Cases = append(r.openResource.Cases, r.openCase)
				}
			}

		//H3
		case 3:

			//is when or then
			if r.IsWhen(string(title)) {
				if r.openCase == nil {
					r.Errors <- fmt.Errorf("Encountered 'when' outside case")
				}

				if r.openCase.When.Path != "" {
					r.Errors <- fmt.Errorf("Encountered multiple 'when' statements in example")
				}

				//set a path that indicates to the code block
				//parser that it should capture and store a when/then
				r.openCase.When.Path = "-"
			} else if r.IsThen(string(title)) {
				if r.openCase == nil {
					r.Errors <- fmt.Errorf("Encountered 'then' outside case")
				}

				if r.openCase.Then.Status != "" {
					r.Errors <- fmt.Errorf("Encountered multiple 'then' statements in example")
				}

				//set a status that indicates to the code block
				//parser that it should capture and store a when/then
				r.openCase.Then.Status = "-"
			}

		}

	}

}

func (r *withJSON) NormalText(out *bytes.Buffer, text []byte) {
	if r.recorder != nil {
		r.recorder.Write(text)
	}

	r.lastTextBeforeMarker = out.Len()
	r.Renderer.NormalText(out, text)
	r.lastTextAfterMarker = out.Len()
}

// A parser implementation that takes
// markdown files and returns manifest data
type Markdown struct {
	Dir   string
	Pages map[string][]byte

	data *manifest.ManifestData
}

func NewMarkdown(dir string) *Markdown {
	p := &Markdown{Dir: dir, Pages: make(map[string][]byte)}

	p.reset()
	return p
}

func (p *Markdown) visit(fpath string, fi os.FileInfo, err error) error {

	//cancel walk if something went wrong
	if err != nil {
		return err
	}

	//care only about relastive path
	rel, err := filepath.Rel(p.Dir, fpath)
	if err != nil {
		return err
	}

	//create rendere nad handle errors
	renderer := renderer(p.data)
	go func() {
		for err := range renderer.Errors {
			fmt.Println("ERROR", err)
		}
	}()

	if !fi.IsDir() {
		if filepath.Ext(fpath) == ".md" {

			md, err := ioutil.ReadFile(fpath)
			if err != nil {
				return err
			}

			//store html for page
			p.Pages[rel] = blackfriday.Markdown(md, renderer, 0)
		}
	}

	return nil
}

func (p *Markdown) reset() {
	p.data = &manifest.ManifestData{}
}

func (p *Markdown) Parse() (*manifest.ManifestData, error) {
	defer p.reset()

	//walk nodes
	err := filepath.Walk(p.Dir, p.visit)
	if err != nil {
		return nil, err
	}

	//return result
	return p.data, nil
}
