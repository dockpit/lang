package lang

import (
	"github.com/dockpit/lang/parser"
)

func FileParser(dir string) parser.Parser {
	return parser.NewFile(dir)
}

func MarkdownParser(dir string) parser.Parser {
	return parser.NewMarkdown(dir)
}
