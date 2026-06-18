package main

import (
	"fmt"
	"html/template"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arran4/g2"
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
		"hasPrefix":           func(s, prefix any) bool {
			sStr, ok1 := s.(string)
			pStr, ok2 := prefix.(string)
			if ok1 && ok2 {
				return strings.HasPrefix(sStr, pStr)
			}
			return false
		},
		"groupIUSEFlags":      groupIUSEFlagsFunc,
		"isLikelyMasked":      isLikelyMaskedFunc,
		"isPkgLikelyMasked":   isPkgLikelyMaskedFunc,
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
		"formatDependency":   formatDependencyFunc,
		"packageLink":        packageLinkFunc,
		"formatPkgLinkBody":  formatPkgLinkBodyFunc,
		"resolveBreadcrumbs": resolveBreadcrumbsFunc,
	}
}

func pathRel(basePath, targetPath string) string {
	if basePath == targetPath {
		return "."
	}

	basePath = strings.TrimPrefix(basePath, "/")
	targetPath = strings.TrimPrefix(targetPath, "/")

	baseParts := strings.Split(basePath, "/")
	targetParts := strings.Split(targetPath, "/")

	if len(baseParts) == 1 && baseParts[0] == "" {
		baseParts = nil
	}
	if len(targetParts) == 1 && targetParts[0] == "" {
		targetParts = nil
	}

	i := 0
	for i < len(baseParts) && i < len(targetParts) && baseParts[i] == targetParts[i] {
		i++
	}

	var res []string
	for j := i; j < len(baseParts); j++ {
		res = append(res, "..")
	}
	for j := i; j < len(targetParts); j++ {
		res = append(res, targetParts[j])
	}

	if len(res) == 0 {
		return "."
	}

	return strings.Join(res, "/")
}

func resolveBreadcrumbsFunc(currentPath string, crumbs []g2.Breadcrumb) []g2.Breadcrumb {
	var resolved []g2.Breadcrumb
	for _, crumb := range crumbs {
		urlStr := crumb.URL
		if crumb.Path != "" {
			rel := pathRel(currentPath, crumb.Path)
			if rel == "." {
				urlStr = "" // current page doesn't usually need a link
			} else {
				urlStr = rel + "/"
			}
		}
		resolved = append(resolved, g2.Breadcrumb{Name: crumb.Name, URL: urlStr, Path: crumb.Path})
	}
	return resolved
}

var tmplPkgRegex = regexp.MustCompile(`&lt;pkg&gt;(.*?)&lt;/pkg&gt;`)

func formatPkgLinkBodyFunc(body string, baseURL string, currentRepo string, repoPkgs interface{}, globalPkgs []*AggPackage) template.HTML {
	safeBody := template.HTMLEscapeString(body)
	return template.HTML(tmplPkgRegex.ReplaceAllStringFunc(safeBody, func(m string) string {
		match := tmplPkgRegex.FindStringSubmatch(m)
		pkg := match[1]
		parts := strings.Split(pkg, "/")
		if len(parts) == 2 {
			repoHasPkg := false
			if repoPkgs != nil {
				if rp, ok := repoPkgs.([]*AggPackage); ok {
					for _, p := range rp {
						if p.Category == parts[0] && p.Name == parts[1] {
							repoHasPkg = true
							break
						}
					}
				} else if rp, ok := repoPkgs.([]g2.PackageData); ok {
					for _, p := range rp {
						if p.Category == parts[0] && p.Name == parts[1] {
							repoHasPkg = true
							break
						}
					}
				}
			}
			if repoHasPkg && currentRepo != "" {
				return fmt.Sprintf("<a href=\"%srepos/%s/categories/%s/packages/%s/\">%s</a>", template.HTMLEscapeString(baseURL), template.HTMLEscapeString(currentRepo), url.PathEscape(parts[0]), url.PathEscape(parts[1]), pkg)
			}

			return fmt.Sprintf("<a href=\"%spackages/%s/%s/\">%s</a>", template.HTMLEscapeString(baseURL), url.PathEscape(parts[0]), url.PathEscape(parts[1]), pkg)
		}
		return m
	}))
}

func packageLinkFunc(baseURL string, pkg string) template.HTML {
	parts := strings.Split(pkg, "/")
	if len(parts) == 2 {
		return template.HTML(fmt.Sprintf("<a href=\"%spackages/%s/%s/\">%s</a>", template.HTMLEscapeString(baseURL), url.PathEscape(parts[0]), url.PathEscape(parts[1]), template.HTMLEscapeString(pkg)))
	}
	return template.HTML(template.HTMLEscapeString(pkg))
}

func formatDependencyFunc(baseURL string, dep string) template.HTML {
	if dep == "" {
		return template.HTML("")
	}

	var b strings.Builder
	b.Grow(len(dep) * 2)

	isSpace := func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}

	var currentToken strings.Builder
	flushToken := func() {
		token := currentToken.String()
		if token == "" {
			return
		}
		currentToken.Reset()

		if token == "||" || token == "(" || token == ")" || strings.HasSuffix(token, "?") || !strings.Contains(token, "/") {
			b.WriteString(template.HTMLEscapeString(token))
			return
		}

		pkgName := g2.ExtractPackageNameFromDep(token)
		if pkgName != "" {
			parts := strings.Split(pkgName, "/")
			if len(parts) == 2 {
				link := fmt.Sprintf("<a href=\"%spackages/%s/%s/\">%s</a>", template.HTMLEscapeString(baseURL), url.PathEscape(parts[0]), url.PathEscape(parts[1]), template.HTMLEscapeString(token))
				b.WriteString(link)
				return
			}
		}
		b.WriteString(template.HTMLEscapeString(token))
	}

	for _, r := range dep {
		if isSpace(r) {
			flushToken()
			b.WriteRune(r)
		} else {
			currentToken.WriteRune(r)
		}
	}
	flushToken()

	return template.HTML(b.String())
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

func isLikelyMaskedFunc(keywords any, explicitlyMasked any) bool {
	if val := reflect.ValueOf(explicitlyMasked); val.IsValid() && !val.IsZero() {
		return true
	}

	keywordsStr, _ := keywords.(string)
	parts := strings.Fields(keywordsStr)
	if len(parts) == 0 {
		return true
	}
	for _, p := range parts {
		if !strings.HasPrefix(p, "-") && !strings.HasPrefix(p, "~") {
			return false
		}
	}
	return true
}

func isPkgLikelyMaskedFunc(pkg any) bool {
	val := reflect.ValueOf(pkg)
	if !val.IsValid() || val.IsZero() || val.Kind() != reflect.Struct {
		return false
	}

	// Check explicitlyMasked (.Masked)
	explicitlyMasked := val.FieldByName("Masked")
	if explicitlyMasked.IsValid() && !explicitlyMasked.IsZero() {
		return true
	}

	versions := val.FieldByName("Versions")
	if versions.IsValid() && versions.Kind() == reflect.Slice {
		if versions.Len() == 0 {
			return true // No versions, likely masked/empty
		}

		allVersionsMasked := true
		for i := 0; i < versions.Len(); i++ {
			versionVal := versions.Index(i)
			if !versionVal.IsValid() || versionVal.Kind() != reflect.Struct {
				continue
			}

			ebuildVal := versionVal.FieldByName("Ebuild")
			if !ebuildVal.IsValid() || ebuildVal.IsZero() {
				continue
			}

			if ebuildVal.Kind() == reflect.Pointer {
				ebuildVal = ebuildVal.Elem()
			}

			varsVal := ebuildVal.FieldByName("Vars")
			if !varsVal.IsValid() || varsVal.IsZero() {
				continue
			}

			keywordsVal := varsVal.MapIndex(reflect.ValueOf("KEYWORDS"))
			var keywords string
			if keywordsVal.IsValid() {
				keywords = keywordsVal.String()
			}

			if !isLikelyMaskedFunc(keywords, nil) {
				allVersionsMasked = false
				break
			}
		}

		if allVersionsMasked {
			return true
		}
	}

	return false
}
