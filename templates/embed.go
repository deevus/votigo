package templates

import "embed"

//go:embed legacy/*.html legacy/admin/*.html modern/*.html modern/admin/*.html modern/partials/*.html
var FS embed.FS
