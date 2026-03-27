package main

import (
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/arran4/g2"
)

//go:embed sitegen_templates/*.html sitegen_templates/*.xml sitegen_templates/*.js
var siteTemplates embed.FS

func parseDependsLinkFunc(deps []string) template.HTML {
	if len(deps) == 0 {
		return ""
	}
	var links []string
	for _, dep := range deps {
		cleanDep := dep
		for len(cleanDep) > 0 && strings.ContainsAny(cleanDep[:1], "<>=~!:") {
			cleanDep = cleanDep[1:]
		}

		if idx := strings.Index(cleanDep, ":"); idx != -1 {
			cleanDep = cleanDep[:idx]
		}
		if idx := strings.Index(cleanDep, "["); idx != -1 {
			cleanDep = cleanDep[:idx]
		}

		escapedDep := template.HTMLEscapeString(dep)

		if strings.Contains(cleanDep, "/") {
			parts := strings.SplitN(cleanDep, "/", 2)
			if len(parts) == 2 {
				pkgName := parts[1]

				// Try to extract the package name exactly using the robust ebuild parser.
				// Since ParseEbuildVariables expects a trailing .ebuild extension and handles Gentoo versions perfectly:
				vars := g2.ParseEbuildVariables(pkgName + ".ebuild")
				if vars != nil && vars["PN"] != "" {
				    pkgName = vars["PN"]
				} else {
					// Fallback for cases without a version (like simple app-arch/7zip).
					// Append a dummy version to let the parser extract the base PN.
					vars2 := g2.ParseEbuildVariables(pkgName + "-1.0.ebuild")
					if vars2 != nil && vars2["PN"] != "" {
						pkgName = vars2["PN"]
					}
				}

				link := fmt.Sprintf(`<a href="/packages/%s/%s/">%s</a>`, template.HTMLEscapeString(parts[0]), template.HTMLEscapeString(pkgName), escapedDep)
				links = append(links, link)
				continue
			}
		}

		links = append(links, escapedDep)
	}
	return template.HTML(strings.Join(links, "<br>\n"))
}

func getTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"join":             strings.Join,
		"parseIUSEFlags":   parseIUSEFlagsFunc,
		"parseDependsLink": parseDependsLinkFunc,
	}
}
