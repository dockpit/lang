package lang_test

import (
	"testing"

	"github.com/dockpit/lang"
	"github.com/dockpit/lang/parser"
)

//just test interface adherinance
func TestLang(t *testing.T) {
	var p parser.Parser

	p = lang.FileParser(".")
	p = lang.MarkdownParser(".")

	_ = p
}
