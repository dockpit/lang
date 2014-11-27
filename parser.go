package lang

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dockpit/pit/contract"
)

var CaseEX = regexp.MustCompile(`^'(.*)'$`)
var ResourceEX = regexp.MustCompile(`^- (.*)`)
var VariableEX = regexp.MustCompile(`(\(.*?\))`)
var ValidHTTPMethods = []string{"GET", "POST", "PUT"} //@todo add more

func UnexpectedDirError(fi os.FileInfo) error {
	return fmt.Errorf("Parser encountered an unexpected directory: %s, expected a resource directory (starting with `- `), or a case directory formatted as `'case name'`", fi.Name())
}

func UnexpectedFileError(fi os.FileInfo) error {
	return fmt.Errorf("Parser encountered a file without an extension: '%s', only 'given', 'when', 'then' or 'while' is allowed", fi.Name())
}

func UnexpectedStateLineError(fpath, line string) error {
	return fmt.Errorf("Parser encountered a 'given' file '%s', with an invalid line: \n%s\n expected format \"<state provider name> '<state name>'\"", fpath, line)
}

func UnexpectedLinkLineError(fpath, line string) error {
	return fmt.Errorf("Parser encountered a 'while' file '%s' with an unexpected line: %s, expected format \"<service id> '<case name>'\"", fpath, line)
}

func UnexpectedLinkLineCaseNameError(fpath, line string) error {
	return fmt.Errorf("Parser encountered a 'while' file '%s' with a invalid casename: \"%s\", expected single-quoted name: e.g 'name of the case'", fpath, line)
}

func UnexpectedHeaderLineError(fpath, giv string) error {
	return fmt.Errorf("File '%s' has an unexpected header line: '%s', expected format 'Header-Key: Value'", fpath, giv)
}

func UnexpectedResponseLineError(fpath, line string) error {
	return fmt.Errorf("Parser encountered a 'then' file '%s' with an unexpected first line: %s, expected format '<HTTP Status Code> <Status Text>'", fpath, line)
}

func UnexpectedResponseLineCodeError(fpath, giv string, err error) error {
	return fmt.Errorf("Parser encountered a 'then' file '%s' with an unexpected status code: %s, expected a number. (%s)", fpath, giv, err)
}

func UnexpectedRequestLineError(fpath, line string) error {
	return fmt.Errorf("Parser encountered a 'when' file '%s' with an unexpected first line: %s, expected format '<HTTP method> <path>'", fpath, line)
}

func UnexpectedRequestLineMethodError(fpath, giv string) error {
	return fmt.Errorf("Parser encountered a 'when' file '%s' with an unexpected HTTP Method in the first line: '%s', expected one of: %s", fpath, giv, ValidHTTPMethods)
}

func UnexpectedRequestLinePathError(fpath, giv string) error {
	return fmt.Errorf("Parser encountered a 'when' file '%s' with an unexpected path in the first line: '%s', expected absolute path (starting with '/')", fpath, giv)
}

type Node struct {
	Pattern  string
	Children []*Node
}

func NewNode() *Node {
	return &Node{
		Pattern:  "/",
		Children: []*Node{},
	}
}

func (n *Node) Append(nn *Node, part string) {
	n.Children = append(n.Children, nn)

	nn.Pattern = path.Join(n.Pattern, part)
}

//
//
//
type Parser struct {
	Dir string

	data  *contract.ContractData
	nodes map[string]*Node
	cases map[string]string

	currentNode     *Node
	currentResource *contract.ResourceData
	currentCase     *contract.CaseData
}

func NewParser(dir string) *Parser {
	p := &Parser{
		Dir: dir,
	}
	p.reset()
	return p
}

func (p *Parser) reset() {
	root := NewNode()
	p.data = &contract.ContractData{}
	p.nodes = map[string]*Node{".": root}
	p.cases = map[string]string{}
	p.currentNode = root
	p.currentResource = nil
	p.currentCase = nil
}

func (p *Parser) ParseHTTPMessage(r io.ReadCloser, fpath string) (string, http.Header, string, error) {

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

// parses a when file loosely based on the http request spec
func (p *Parser) ParseWhen(r io.ReadCloser, fpath string) (*contract.When, error) {
	w := &contract.When{}

	rline, headers, body, err := p.ParseHTTPMessage(r, fpath)
	if err != nil {
		return nil, err
	}

	//request line should be 2 parts seperated by a space
	rlinep := strings.SplitN(rline, " ", 2)
	if len(rlinep) != 2 {
		return nil, UnexpectedRequestLineError(fpath, rline)
	}

	//check method format
	method := ""
	for _, m := range ValidHTTPMethods {
		if m == rlinep[0] {
			method = m
			break
		}
	}

	if method == "" {
		return nil, UnexpectedRequestLineMethodError(fpath, rlinep[0])
	}
	w.Method = method

	//check path
	if !path.IsAbs(rlinep[1]) {
		return nil, UnexpectedRequestLinePathError(fpath, rlinep[1])
	}
	w.Path = rlinep[1]
	w.Headers = headers
	w.Body = body

	return w, nil
}

// parses a then file loosely based on the format of a standard http message
func (p *Parser) ParseThen(r io.ReadCloser, fpath string) (*contract.Then, error) {
	t := &contract.Then{}

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

// parses a 'while' file
func (p *Parser) ParseWhile(r io.ReadCloser, fpath string) ([]contract.While, error) {
	ws := []contract.While{}

	s := bufio.NewScanner(r)
	for s.Scan() {

		//dont mind empty lines
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}

		//every non-empty line should have space seperated link
		wp := strings.SplitN(s.Text(), " ", 2)
		if len(wp) != 2 {
			return ws, UnexpectedLinkLineError(fpath, s.Text())
		}

		cname := p.ToCaseName(wp[1])
		if cname == "" {
			return ws, UnexpectedLinkLineCaseNameError(fpath, wp[1])
		}

		//create while for case
		ws = append(ws, contract.While{
			ID:       wp[0],
			CaseName: cname,
		})
	}

	return ws, nil
}

// parses a 'given' file
func (p *Parser) ParseGiven(r io.ReadCloser, fpath string) (map[string]contract.Given, error) {
	gs := make(map[string]contract.Given)

	s := bufio.NewScanner(r)
	for s.Scan() {

		//dont mind empty lines
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}

		//every non-empty line should have space seperated link
		gp := strings.SplitN(s.Text(), ":", 2)
		if len(gp) != 2 {
			return gs, UnexpectedStateLineError(fpath, s.Text())
		}

		//extract provider name
		pname := strings.TrimSpace(gp[0])
		if pname == "" {
			return gs, UnexpectedStateLineError(fpath, s.Text())
		}

		//extract state name as case name
		sname := p.ToCaseName(strings.TrimSpace(gp[1]))
		if sname == "" {
			return gs, UnexpectedStateLineError(fpath, s.Text())
		}

		//set given
		gs[pname] = contract.Given{
			Name: sname,
		}
	}

	return gs, nil
}

//
// Returns wether a given basename of a file path denotes a resource
func (p *Parser) ToResourcePatternPart(basename string) string {
	m := ResourceEX.FindStringSubmatch(basename)
	if m == nil {
		return ""
	}

	//replace variables by sinatra style vars
	res := VariableEX.ReplaceAllFunc([]byte(m[1]), func(in []byte) []byte {
		v := in[1 : len(in)-1]
		return append([]byte(":"), v...)
	})

	return string(res)
}

//
// Returns wether a given basename of a file path denotes a case example
func (p *Parser) ToCaseName(basename string) string {
	m := CaseEX.FindStringSubmatch(basename)
	if m == nil {
		return ""
	}

	return m[1]
}

func (p *Parser) visit(fpath string, fi os.FileInfo, err error) error {

	//cancel walk if something went wrong
	if err != nil {
		return err
	}

	//skip root
	if fpath == p.Dir {
		return nil
	}

	//care only about relative path
	rel, err := filepath.Rel(p.Dir, fpath)
	if err != nil {
		return err
	}

	//directories are expected to be either resources or cases
	if fi.IsDir() {
		if part := p.ToResourcePatternPart(filepath.Base(rel)); part != "" {
			var parent *Node
			var ok bool

			//create and add new resource node
			p.currentNode = NewNode()
			p.nodes[rel] = p.currentNode

			//get parent node
			if parent, ok = p.nodes[filepath.Dir(rel)]; !ok {
				return fmt.Errorf("No parent node found for '%s'", fpath)
			}

			//apprent to parent
			parent.Append(p.currentNode, part)

			//use node structure to create and append resource to contract
			p.currentResource = &contract.ResourceData{
				Pattern: p.currentNode.Pattern,
				Cases:   []*contract.CaseData{},
			}

			// add to contract data
			p.data.Resources = append(p.data.Resources, p.currentResource)

		} else if cname := p.ToCaseName(filepath.Base(rel)); cname != "" {

			//get current resource (if any)
			res := p.currentResource
			if res == nil {
				return fmt.Errorf("Case folder '%s' (%s) is outside a resource", cname, fpath)
			}

			//case name must be unique
			if ex, ok := p.cases[cname]; ok {
				return fmt.Errorf("Case with name '%s' (%s) already exists in '%s'", cname, fpath, filepath.Dir(ex))
			}

			//create the case from available data
			p.currentCase = &contract.CaseData{
				Name: cname,
			}

			//and append to resource
			res.Cases = append(res.Cases, p.currentCase)
			p.cases[cname] = fpath
		} else {
			return UnexpectedDirError(fi)
		}
	} else {

		//files without extension have to be either when/then/given/while
		if filepath.Ext(fpath) == "" {

			f, err := os.Open(fpath)
			if err != nil {
				return err
			}
			defer f.Close()

			//'keywords;
			if filepath.Base(fpath) == "given" {
				given, err := p.ParseGiven(f, fpath)
				if err != nil {
					return err
				}

				p.currentCase.Given = given
			} else if filepath.Base(fpath) == "when" {
				when, err := p.ParseWhen(f, fpath)
				if err != nil {
					return err
				}

				fmt.Println("When:", when)

				p.currentCase.When = *when
			} else if filepath.Base(fpath) == "then" {
				then, err := p.ParseThen(f, fpath)
				if err != nil {
					return err
				}

				fmt.Println("Then:", then)

				p.currentCase.Then = *then
			} else if filepath.Base(fpath) == "while" {
				whiles, err := p.ParseWhile(f, fpath)
				if err != nil {
					return err
				}

				p.currentCase.While = whiles
			} else {
				return UnexpectedFileError(fi)
			}

		} else {
			//@todo parse data files
		}

	}

	return nil
}

func (p *Parser) Parse() (*contract.ContractData, error) {

	//reset parser afterwards
	defer p.reset()

	//walk nodes
	err := filepath.Walk(p.Dir, p.visit)
	if err != nil {
		return nil, err
	}

	//return result
	return p.data, nil
}
