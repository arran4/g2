package main

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"
)

func getTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"join":                strings.Join,
		"parseIUSEFlags":      parseIUSEFlagsFunc,
		"buildOwnerEmailLink": buildOwnerEmailLinkFunc,
		"now":                 time.Now,
		"slugify":             sanitizeFilename,
		"split":               strings.Split,
		"formatKeywords":      formatKeywordsFunc,
	}
}

func formatKeywordsFunc(keywords string, baseURL string) template.HTML {
	if keywords == "" {
		return template.HTML("")
	}
	parts := strings.Fields(keywords)
	var formatted []string
	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			formatted = append(formatted, p)
			continue
		}

		arch := p
		if strings.HasPrefix(p, "~") {
			arch = p[1:]
			formatted = append(formatted, fmt.Sprintf("~<a href=\"%sarches/%s/\" class=\"text-decoration-none\">%s</a>", baseURL, arch, arch))
		} else {
			formatted = append(formatted, fmt.Sprintf("<a href=\"%sarches/%s/\" class=\"text-decoration-none\">%s</a>", baseURL, arch, arch))
		}
	}
	return template.HTML(strings.Join(formatted, " "))
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
