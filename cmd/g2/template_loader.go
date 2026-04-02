package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arran4/g2/templates"
)

var (
	siteTemplates *template.Template
	siteTmplErr   error
	siteTmplOnce  sync.Once
)

// GetSiteTemplates loads templates from templates/app, templates/partials, and templates/views
// once, and returns the parsed template registry.
func GetSiteTemplates() (*template.Template, error) {
	siteTmplOnce.Do(func() {
		tmpl := template.New("").Funcs(getTemplateFuncMap())

		err := fs.WalkDir(templates.SiteFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			if strings.HasSuffix(path, ".html") {
				b, err := fs.ReadFile(templates.SiteFS, path)
				if err != nil {
					return err
				}

				name := filepath.Base(path)

				// Optional: if we want to name them by path for partials
				// For now, base name is used as before, but we can change this later if collisions happen
				// However, because we are moving files into subfolders and changing references like "repo_info_pkgs.html"
				// to "repo/info_pkgs.html", we should name the template after its path relative to "views/", "app/", or "partials/"

				// Determine template name based on path
				if strings.HasPrefix(path, "views/") {
					name = strings.TrimPrefix(path, "views/")
				} else if strings.HasPrefix(path, "app/") {
					name = strings.TrimPrefix(path, "app/")
				} else if strings.HasPrefix(path, "partials/") {
					name = strings.TrimPrefix(path, "partials/")
				}

				_, err = tmpl.New(name).Parse(string(b))
				if err != nil {
					return fmt.Errorf("parsing template %s: %w", path, err)
				}
			}

			return nil
		})

		if err != nil {
			siteTmplErr = err
		} else {
			siteTemplates = tmpl
		}
	})
	return siteTemplates, siteTmplErr
}
