package main

import (
	"bytes"
	"github.com/arran4/g2"
	"testing"
)

type GenericPageContextOption func(*GenericPageContext)

func WithGlobalPackage(pkg *AggPackage) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.GlobalPackage = pkg
	}
}

func WithRepoPackageReverseVirtuals(v ...string) GenericPageContextOption {
	return func(c *GenericPageContext) {
		if c.RepoPackage == nil {
			c.RepoPackage = &g2.PackageData{}
		}
		c.RepoPackage.ReverseVirtuals = append(c.RepoPackage.ReverseVirtuals, v...)
	}
}

func WithRepoPackageEquivalents(eq ...string) GenericPageContextOption {
	return func(c *GenericPageContext) {
		if c.RepoPackage == nil {
			c.RepoPackage = &g2.PackageData{}
		}
		c.RepoPackage.Equivalents = append(c.RepoPackage.Equivalents, eq...)
	}
}

func WithProject(proj *AggProject) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Project = proj
	}
}

func WithRepoCategory(cat *g2.CategoryData) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.RepoCategory = cat
	}
}

func WithCategory(cat map[string]interface{}) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Category = cat
	}
}

func WithGlobalProfile(prof *g2.AggProfile) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.GlobalProfile = prof
	}
}

func WithRepoProfile(prof *g2.ProfileData) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.RepoProfile = prof
	}
}

func WithGroup(group *RepoGroup) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Group = group
	}
}

func WithGlobalUseFlag(flag *AggUseFlag) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.GlobalUseFlag = flag
	}
}

func WithLicense(lic *AggLicense) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.License = lic
	}
}

func WithArch(arch *AggArch) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Arch = arch
	}
}

func WithRepo(repo *g2.SiteData) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Repo = repo
	}
}

func WithManifest(man *g2.ManifestEntryData) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Manifest = man
	}
}

func WithVersionData(ver *g2.VersionData) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.VersionData = ver
	}
}

func WithEclass(eclass *AggEclass) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Eclass = eclass
	}
}

func WithUseExpandDesc(desc *g2.UseExpandDesc) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.UseExpandDesc = desc
	}
}

func WithBreadcrumbs(bc []g2.Breadcrumb) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.Breadcrumbs = bc
	}
}

func WithGlobalCategory(cat *AggCategory) GenericPageContextOption {
	return func(c *GenericPageContext) {
		c.GlobalCategory = cat
	}
}

func NewGenericPageContext(opts ...GenericPageContextOption) GenericPageContext {
	ctx := GenericPageContext{
		GlobalPackage: &AggPackage{},
		RepoPackage: &g2.PackageData{},
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
		GlobalCategory: &AggCategory{},
	}
	for _, opt := range opts {
		opt(&ctx)
	}
	return ctx
}

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
			data: NewGenericPageContext(
				WithRepoPackageReverseVirtuals("category/package", "invalid", "x/y"),
				WithRepoPackageEquivalents("category/package", "invalid", "x/y"),
			),
		},
		{
			name: "Edge Cases",
			data: NewGenericPageContext(
				WithGlobalPackage(&AggPackage{
					Name:     "invalid-package", // missing slash
					Category: "invalid",
				}),
				WithRepoPackageReverseVirtuals("invalid", "category/package", "foo/bar/baz"),
				WithRepoPackageEquivalents("invalid", "category/package"),
				WithGlobalUseFlag(&AggUseFlag{
					LocalDescs:    map[string]string{"invalid": "desc"},
					MetadataDescs: map[string]string{"invalid": "desc"},
				}),
				WithVersionData(&g2.VersionData{
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{
							"KEYWORDS":  "amd64 ~x86 -* invalid",
							"INHERITED": "eclass1 eclass2",
							"LICENSE":   "GPL-2",
						},
						RawText: "EAPI=8\n",
					},
				}),
				WithBreadcrumbs([]g2.Breadcrumb{
					{Name: "Home", URL: "/"},
				}),
			),
		},
		{
			name: "Extreme Edge Cases",
			data: NewGenericPageContext(
				WithCategory(map[string]interface{}{
					"ReposList": []*g2.SiteData{},
					"Name":      "invalid-no-slashes",
				}),
				WithBreadcrumbs([]g2.Breadcrumb{}),
			),
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
