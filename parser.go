package lang

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/dockpit/pit/contract"
)

func UnexpectedDirError(fi os.FileInfo) error {
	return fmt.Errorf("Parser encountered an unexpected directory: %s", fi.Name())
}

var CaseEX = regexp.MustCompile(`^'(.*)'$`)
var ResourceEX = regexp.MustCompile(`^- (.*)`)
var VariableEX = regexp.MustCompile(`(\(.*?\))`)

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

	data            *contract.ContractData
	nodes           map[string]*Node
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

//
//
//
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

func (p *Parser) reset() {
	root := NewNode()
	p.data = &contract.ContractData{}
	p.nodes = map[string]*Node{".": root}
	p.currentNode = root
}

func (p *Parser) visit(path string, fi os.FileInfo, err error) error {

	//skip root
	if path == p.Dir {
		return nil
	}

	//cancel walk if something went wrong
	if err != nil {
		return err
	}

	//care only about relative path
	rel, err := filepath.Rel(p.Dir, path)
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
				return fmt.Errorf("No parent node found for '%s'", path)
			}

			//apprent to parent
			parent.Append(p.currentNode, part)

			//use node structure to create and append resource to contract
			p.currentResource = &contract.ResourceData{
				Name:    part, //@todo find something better for parsing a unique name
				Pattern: p.currentNode.Pattern,
				Cases:   []*contract.CaseData{},
			}

			// add to contract data
			p.data.Resources = append(p.data.Resources, p.currentResource)

		} else if cname := p.ToCaseName(filepath.Base(rel)); cname != "" {

			//get current resource (if any)
			res := p.currentResource
			if res == nil {
				return fmt.Errorf("Case folder '%s' (%s) is outside a resource", cname, path)
			}

			//create the case from available data
			p.currentCase = &contract.CaseData{
			//@todo default case here?
			}

			//and append to resource
			res.Cases = append(res.Cases, p.currentCase)
		} else {
			return UnexpectedDirError(fi)
		}
	}

	//@todo parse files

	return nil
}
