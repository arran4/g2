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

	testCases := []struct {
		name string
		data GenericPageContext
	}{
		{
			name: "Basic Dummy Data",
			data: GenericPageContext{
				GlobalPackage: &AggPackage{},
				RepoPackage: &g2.PackageData{
					ReverseVirtuals: []string{"category/package", "invalid", "x/y"},
					Equivalents: []string{"category/package", "invalid", "x/y"},
				},
				Project: &AggProject{Project: &g2.Project{}},
				RepoCategory: &g2.CategoryData{},
				Category: map[string]interface{}{},
				GlobalProfile: &g2.AggProfile{},
				RepoProfile: &g2.ProfileData{},
				Group: &RepoGroup{},
				GlobalUseFlag: &AggUseFlag{},
				License: &AggLicense{},
				Arch: &AggArch{},
				Repo: &g2.SiteData{},
				Manifest: &g2.ManifestEntryData{
					Entry: &g2.ManifestEntry{},
				},
				VersionData: &g2.VersionData{
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{},
					},
				},
				Eclass: &AggEclass{},
				UseExpandDesc: &g2.UseExpandDesc{},
			},
		},
		{
			name: "Edge Cases",
			data: GenericPageContext{
				GlobalPackage: &AggPackage{
					Name: "invalid-package", // missing slash
					Category: "invalid",
				},
				RepoPackage: &g2.PackageData{
					ReverseVirtuals: []string{"invalid", "category/package", "foo/bar/baz"}, // invalid reverse virtuals
					Equivalents: []string{"invalid", "category/package"},
				},
				Project: &AggProject{Project: &g2.Project{}},
				RepoCategory: &g2.CategoryData{},
				Category: map[string]interface{}{},
				GlobalProfile: &g2.AggProfile{},
				RepoProfile: &g2.ProfileData{},
				Group: &RepoGroup{},
				GlobalUseFlag: &AggUseFlag{
					LocalDescs: map[string]string{"invalid": "desc"},
					MetadataDescs: map[string]string{"invalid": "desc"},
				},
				License: &AggLicense{},
				Arch: &AggArch{},
				Repo: &g2.SiteData{},
				Manifest: &g2.ManifestEntryData{
					Entry: &g2.ManifestEntry{},
				},
				VersionData: &g2.VersionData{
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{
							"KEYWORDS": "amd64 ~x86 -* invalid",
							"INHERITED": "eclass1 eclass2",
							"LICENSE": "GPL-2",
						},
						RawText: "EAPI=8\n",
					},
				},
				Eclass: &AggEclass{},
				UseExpandDesc: &g2.UseExpandDesc{},
				Breadcrumbs: []g2.Breadcrumb{
					{Name: "Home", URL: "/"},
				},
			},
		},
		{
			name: "Extreme Edge Cases",
			data: GenericPageContext{
				GlobalPackage: &AggPackage{},
				RepoPackage: &g2.PackageData{},
				Project: &AggProject{Project: &g2.Project{}},
				RepoCategory: &g2.CategoryData{},
				Category: map[string]interface{}{
					"ReposList": []*g2.SiteData{},
					"Name": "invalid-no-slashes",
				},
				GlobalProfile: &g2.AggProfile{},
				RepoProfile: &g2.ProfileData{},
				Group: &RepoGroup{},
				GlobalUseFlag: &AggUseFlag{},
				License: &AggLicense{},
				Arch: &AggArch{},
				Repo: &g2.SiteData{},
				Manifest: &g2.ManifestEntryData{
					Entry: &g2.ManifestEntry{},
				},
				VersionData: &g2.VersionData{
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{},
					},
				},
				Eclass: &AggEclass{},
				UseExpandDesc: &g2.UseExpandDesc{},
				Breadcrumbs: []g2.Breadcrumb{},
				GlobalCategory: &AggCategory{},
			},
		},
	}

	for _, tpl := range templates {
		name := tpl.Name()
		if name == "layout_header.html" || name == "layout_footer.html" || name == "" {
			continue
		}
		// we skip layout files, test only views
		// to find what's wrong with them

		for _, tc := range testCases {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				var buf bytes.Buffer
				err := tpl.Execute(&buf, tc.data)
				if err != nil {
					t.Errorf("template %s failed to render with %s: %v", name, tc.name, err)
				}
			})
		}
	}
}
