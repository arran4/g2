package main

import (
	"fmt"
	"html/template"
	"net/url"
	"reflect"
	"sort"
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
		"hasPrefix":           strings.HasPrefix,
		"groupIUSEFlags":      groupIUSEFlagsFunc,
		"isLikelyMasked":      isLikelyMaskedFunc,
		"len_or_zero": func(v any) int {
			if v == nil {
				return 0
			}
			val := reflect.ValueOf(v)
			if val.Kind() == reflect.Slice || val.Kind() == reflect.Map || val.Kind() == reflect.String || val.Kind() == reflect.Array {
				return val.Len()
			}
			return 0
		},
		"packageLink": packageLinkFunc,
	}
}

func packageLinkFunc(baseURL string, pkg string) template.HTML {
	parts := strings.Split(pkg, "/")
	if len(parts) == 2 {
		return template.HTML(fmt.Sprintf("<a href=\"%spackages/%s/%s/\">%s</a>", template.HTMLEscapeString(baseURL), url.PathEscape(parts[0]), url.PathEscape(parts[1]), template.HTMLEscapeString(pkg)))
	}
	return template.HTML(template.HTMLEscapeString(pkg))
}

func formatKeywordsFunc(keywords string, baseURL string) template.HTML {
	if keywords == "" {
		return template.HTML("")
	}
	parts := strings.Fields(keywords)
	var formatted []string
	safeBaseURL := template.HTMLEscapeString(baseURL)
	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			formatted = append(formatted, template.HTMLEscapeString(p))
			continue
		}

		arch := p
		prefix := ""
		if strings.HasPrefix(p, "~") {
			arch = p[1:]
			prefix = "~"
		}

		safeArchText := template.HTMLEscapeString(arch)
		safeArchURL := url.PathEscape(arch)

		formatted = append(formatted, fmt.Sprintf("%s<a href=\"%sarches/%s/\" class=\"text-decoration-none\">%s</a>", prefix, safeBaseURL, safeArchURL, safeArchText))
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

type UseFlagGroup struct {
	Name  string
	Flags []ParsedIUSEFlag
}

func groupIUSEFlagsFunc(flags []ParsedIUSEFlag, useExpandPrefixes map[string]bool) []UseFlagGroup {
	groups := make(map[string][]ParsedIUSEFlag)
	var globalFlags []ParsedIUSEFlag

	for _, f := range flags {
		var matchedPrefix string
		for prefix := range useExpandPrefixes {
			if strings.HasPrefix(f.Name, prefix+"_") {
				if len(prefix) > len(matchedPrefix) {
					matchedPrefix = prefix
				}
			}
		}

		if matchedPrefix != "" {
			groups[matchedPrefix] = append(groups[matchedPrefix], f)
		} else {
			globalFlags = append(globalFlags, f)
		}
	}

	var result []UseFlagGroup
	if len(globalFlags) > 0 {
		result = append(result, UseFlagGroup{Name: "global", Flags: globalFlags})
	}
	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for _, name := range groupNames {
		result = append(result, UseFlagGroup{Name: name, Flags: groups[name]})
	}
	return result
}

func isLikelyMaskedFunc(keywords interface{}, explicitlyMasked interface{}) bool {
	if explicitlyMasked != nil {
		val := reflect.ValueOf(explicitlyMasked)
		if val.Kind() == reflect.Pointer && !val.IsNil() {
			return true
		} else if val.Kind() != reflect.Pointer && val.IsValid() && !val.IsZero() {
			return true
		}
	}

	keywordsStr := ""
	if keywords != nil {
		if s, ok := keywords.(string); ok {
			keywordsStr = s
		}
	}

	if strings.TrimSpace(keywordsStr) == "" {
		return true
	}
	parts := strings.Fields(keywordsStr)
	for _, p := range parts {
		if !strings.HasPrefix(p, "-") && !strings.HasPrefix(p, "~") {
			return false
		}
	}
	return true
}
