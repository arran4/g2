package templates

import (
	"embed"
)

//go:embed ebuild/*.ebuild ebuild/*.tmpl
var EbuildFS embed.FS

//go:embed app/*.html partials/*.html views/**/*.html views/*.html site/*.xml site/*.js
var SiteFS embed.FS
