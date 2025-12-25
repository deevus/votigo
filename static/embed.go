package static

import "embed"

//go:embed css/*.css js/*.js fonts/*.woff2
var FS embed.FS
