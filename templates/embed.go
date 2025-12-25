package templates

import "embed"

//go:embed legacy/*.html legacy/admin/*.html modern/*.html
var FS embed.FS
