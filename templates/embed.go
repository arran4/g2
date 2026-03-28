package templates

import (
	"embed"
)

//go:embed ebuild/*.ebuild
var EbuildFS embed.FS

//go:embed site/*.html site/*.xml site/*.js
var SiteFS embed.FS
