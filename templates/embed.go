package templates

import "embed"

//go:embed *.html admin/*.html
var FS embed.FS
