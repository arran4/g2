package main

import "embed"

//go:embed sitegen_templates/*.html sitegen_templates/*.xml
var siteTemplates embed.FS
