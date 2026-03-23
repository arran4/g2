package main

import "embed"

//go:embed sitegen_templates/*.html
var siteTemplates embed.FS
