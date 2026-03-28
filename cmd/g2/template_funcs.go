package main

import (
	"html/template"
	"net/url"
	"strings"
)

func getTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"join":                strings.Join,
		"parseIUSEFlags":      parseIUSEFlagsFunc,
		"buildOwnerEmailLink": buildOwnerEmailLinkFunc,
		"slugify":             sanitizeFilename,
	}
}

func buildOwnerEmailLinkFunc(remoteURL, email string) string {
	if strings.Contains(remoteURL, "github.com") || strings.Contains(remoteURL, "gitlab.com") {
		return remoteURL + "/search?q=" + url.QueryEscape(email)
	}
	if remoteURL != "" {
		return remoteURL
	}
	return ""
}
