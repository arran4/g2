package main

import (
	"bytes"
	"github.com/arran4/g2"
	"testing"
)

func TestAllTemplatesRender(t *testing.T) {
	tmpl, err := GetSiteTemplates()
	if err != nil {
		t.Fatalf("failed to get templates: %v", err)
	}

	templates := tmpl.Templates()

	for _, tpl := range templates {
		name := tpl.Name()
		if name == "layout_header.html" || name == "layout_footer.html" || name == "" {
			continue
		}
		// we skip layout files, test only views
		// to find what's wrong with them

		t.Run(name, func(t *testing.T) {
			data := GenericPageContext{}
			// populate it with some dummy data to avoid nil pointer derefs
			data.GlobalPackage = &AggPackage{}
			data.RepoPackage = &g2.PackageData{}
			data.Project = &AggProject{Project: &g2.Project{}}
			data.RepoCategory = &g2.CategoryData{}
			data.Category = map[string]interface{}{}
			data.GlobalProfile = &g2.AggProfile{}
			data.RepoProfile = &g2.ProfileData{}
			data.Group = &RepoGroup{}
			data.GlobalUseFlag = &AggUseFlag{}
			data.License = &AggLicense{}
			data.Arch = &AggArch{}
			data.Repo = &g2.SiteData{}
			data.Manifest = &g2.ManifestEntryData{
				Entry: &g2.ManifestEntry{},
			}
			data.VersionData = &g2.VersionData{
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{},
				},
			}
			data.Eclass = &AggEclass{}
			data.UseExpandDesc = &g2.UseExpandDesc{}

			var buf bytes.Buffer
			err := tpl.Execute(&buf, data)
			if err != nil {
				t.Logf("template %s failed to render: %v", name, err)
				t.Fail()
			}
		})
	}
}
