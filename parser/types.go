package parser

import (
	"github.com/dockpit/lang/manifest"
)

var ValidHTTPMethods = []string{"GET", "POST", "PUT", "DELETE"} //@todo add more

type Parser interface {
	Parse() (*manifest.ManifestData, error)
}
