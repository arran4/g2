package main

import "embed"

//go:embed sitegen_templates/*.html sitegen_templates/*.xml sitegen_templates/*.js
var siteTemplates embed.FS
