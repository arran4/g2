package templates

import (
	"embed"
)

//go:embed ebuild/*.ebuild ebuild/*.tmpl
var EbuildFS embed.FS

//go:embed site/*.html site/*.xml site/*.js
var SiteFS embed.FS
