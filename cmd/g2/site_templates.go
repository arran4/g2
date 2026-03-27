package main

import (
	"embed"
	"html/template"
	"strings"
)

//go:embed sitegen_templates/*.html sitegen_templates/*.xml
var siteTemplates embed.FS

func getTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"join":           strings.Join,
		"parseIUSEFlags": parseIUSEFlagsFunc,
	}
}
