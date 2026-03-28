package main

import (
	"html/template"
	"strings"
)

func getTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"join":           strings.Join,
		"parseIUSEFlags": parseIUSEFlagsFunc,
		"slugify":        sanitizeFilename,
	}
}
