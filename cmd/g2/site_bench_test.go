package main

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func buildSyntheticOverlayFS(numCategories, pkgsPerCategory, versionsPerPkg int) fstest.MapFS {
	memFS := fstest.MapFS{
		"profiles/repo_name": &fstest.MapFile{
			Data: []byte("synthetic-overlay\n"),
		},
		"profiles/eapi": &fstest.MapFile{
			Data: []byte("8\n"),
		},
		"metadata/layout.conf": &fstest.MapFile{
			Data: []byte("masters = gentoo\nrepo-name = synthetic-overlay\n"),
		},
		"profiles/use.desc": &fstest.MapFile{
			Data: []byte("test - Enable dependencies and/or preparations necessary to run tests\ndoc - Add extra documentation\n"),
		},
		"profiles/desc/python_targets.desc": &fstest.MapFile{
			Data: []byte("python3_11 - Build with Python 3.11\npython3_12 - Build with Python 3.12\n"),
		},
		"eclass/distutils-r1.eclass": &fstest.MapFile{
			Data: []byte("# Copyright 1999-2026 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n"),
		},
	}

	validCats := []string{"dev-python", "dev-libs", "app-misc", "sys-apps", "net-misc", "www-apps", "media-libs", "sci-libs"}
	var categoriesBuf strings.Builder
	for c := range numCategories {
		catName := validCats[c%len(validCats)]
		if c >= len(validCats) {
			catName = fmt.Sprintf("%s-%d", catName, c)
		}
		categoriesBuf.WriteString(catName + "\n")
		for p := range pkgsPerCategory {
			pkgName := fmt.Sprintf("pkg-%d", p)
			pkgDir := catName + "/" + pkgName
			memFS[pkgDir+"/metadata.xml"] = &fstest.MapFile{
				Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE pkgmetadata SYSTEM "https://www.gentoo.org/dtd/metadata.dtd">
<pkgmetadata>
	<maintainer type="person">
		<email>jane.doe@example.com</email>
		<name>Jane Doe</name>
	</maintainer>
</pkgmetadata>
`),
			}
			var manifestBuf strings.Builder
			for v := range versionsPerPkg {
				verStr := fmt.Sprintf("1.%d.0", v)
				ebuildName := fmt.Sprintf("%s-%s.ebuild", pkgName, verStr)
				distName := fmt.Sprintf("%s-%s.tar.gz", pkgName, verStr)
				manifestBuf.WriteString(fmt.Sprintf("DIST %s 1024 BLAKE2B deadbeef SHA512 cafebabe\n", distName))
				ebuildContent := fmt.Sprintf(`# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8

inherit distutils-r1

DESCRIPTION="Synthetic package %s in %s"
HOMEPAGE="https://example.com/%s/%s"
SRC_URI="https://example.com/downloads/%s"

LICENSE="MIT GPL-2"
SLOT="0"
KEYWORDS="amd64 ~arm64"
IUSE="test doc python_targets_python3_11 python_targets_python3_12"
RESTRICT="!test? ( test )"

RDEPEND="dev-lang/python:3.12"
DEPEND="${RDEPEND}"

src_prepare() {
	default
	sed -i -e 's/[^[:alnum:]\]/_/g' Makefile || die
}
`, pkgName, catName, catName, pkgName, distName)
				memFS[pkgDir+"/"+ebuildName] = &fstest.MapFile{
					Data:    []byte(ebuildContent),
					ModTime: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
				}
			}
			memFS[pkgDir+"/Manifest"] = &fstest.MapFile{
				Data: []byte(manifestBuf.String()),
			}
		}
	}
	memFS["profiles/categories"] = &fstest.MapFile{
		Data: []byte(categoriesBuf.String()),
	}
	return memFS
}

func TestParseRepoAndAggregationSpeed(t *testing.T) {
	memFS := buildSyntheticOverlayFS(5, 40, 2) // 200 packages, 400 ebuilds

	startParse := time.Now()
	site, err := parseRepo(memFS, ".", "synthetic-overlay", false, nil, SourceURL("https://example.com/synthetic-overlay.git"))
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}
	parseDuration := time.Since(startParse)
	if site.PackageCount != 200 {
		t.Fatalf("expected 200 packages, got %d", site.PackageCount)
	}
	if parseDuration > 5*time.Second {
		t.Errorf("parseRepo took %v for 200 packages (400 ebuilds), expected < 5s", parseDuration)
	}
}

func TestPrepareAggregatedDataLargeScaleSpeed(t *testing.T) {
	const numRepos = 4
	const pkgsPerRepo = 2500 // 10,000 packages sharing identical licenses, eclasses, and USE flags

	sites := make([]*g2.SiteData, 0, numRepos)
	for r := range numRepos {
		repoName := fmt.Sprintf("overlay-%d", r)
		site := &g2.SiteData{
			Title:            repoName,
			RepoName:         repoName,
			ProvidedLicenses: []string{"MIT", "GPL-2", "Apache-2.0"},
			DefinedEclasses:  []g2.EclassData{{Name: "distutils-r1"}, {Name: "cargo"}},
			UseDesc: &g2.UseDesc{
				Flags: map[string]string{
					"test":             "Enable tests",
					"doc":              "Build documentation",
					"unused-in-repo-0": "Flag defined in UseDesc before any package uses it",
				},
			},
			ValidUseExpands: map[string]bool{"python_targets": true},
			UseExpandDescs: map[string]*g2.UseExpandDesc{
				"python_targets": {
					Name: "python_targets",
					Flags: map[string]string{
						"python3_11": "Python 3.11",
						"python3_12": "Python 3.12",
					},
				},
			},
			Projects: &g2.Projects{
				Projects: []g2.Project{
					{
						Email: "python@example.com",
						Name:  "Python Project",
					},
				},
			},
		}

		cat := g2.CategoryData{Name: "dev-python", Packages: make([]g2.PackageData, 0, pkgsPerRepo)}
		repoAggPkgs := make(map[string]*AggPackage, pkgsPerRepo)
		for p := range pkgsPerRepo {
			pkgName := fmt.Sprintf("py-pkg-%d-%d", r, p)
			if p%3 == 0 {
				// Shared package across overlays
				pkgName = fmt.Sprintf("py-shared-%d", p)
			}
			var useBlocks []g2.Use
			if r > 0 {
				useBlocks = []g2.Use{
					{
						Flags: []g2.Flag{
							{Name: "unused-in-repo-0", Text: "Used in subsequent overlay"},
						},
					},
				}
			}
			pkgData := g2.PackageData{
				Name:                pkgName,
				Category:            "dev-python",
				DominantDescription: "Synthetic Python package",
				DominantLicense:     "MIT GPL-2",
				Metadata: &g2.PkgMetadata{
					Maintainers: []g2.Maintainer{
						{Email: "python@example.com", Name: "Python Project", Type: "project"},
					},
					Use: useBlocks,
				},
				Versions: []g2.VersionData{
					{
						Version: "1.0.0",
						PVR:     "1.0.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"PN":           pkgName,
								"PV":           "1.0.0",
								"CATEGORY":     "dev-python",
								"LICENSE":      "MIT GPL-2",
								"INHERITED":    "distutils-r1 cargo",
								"IUSE":         "test doc python_targets_python3_11 python_targets_python3_12",
								"REQUIRED_USE": "|| ( python_targets_python3_11 python_targets_python3_12 )",
							},
						},
					},
				},
			}
			cat.Packages = append(cat.Packages, pkgData)
			pkgKey := "dev-python/" + pkgName
			repoAggPkgs[pkgKey] = &AggPackage{
				Name:         pkgName,
				Category:     "dev-python",
				Repos:        map[string]*g2.SiteData{repoName: site},
				RepoVersions: map[string][]g2.VersionData{repoName: pkgData.Versions},
			}
		}
		site.Categories = []g2.CategoryData{cat}
		site.PackageCount = len(cat.Packages)
		site.AggEclasses = getRepoEclasses(site, repoAggPkgs)
		sites = append(sites, site)
	}

	start := time.Now()
	aggData := prepareAggregatedData(sites)
	aggPkgsMap := make(map[string]*AggPackage, len(aggData.Packages))
	for _, p := range aggData.Packages {
		aggPkgsMap[p.Category+"/"+p.Name] = p
	}
	for _, site := range sites {
		_ = getRepoLicenses(site, aggPkgsMap)
		_ = getRepoUseFlags(site, aggPkgsMap)
	}
	elapsed := time.Since(start)

	if len(aggData.Packages) == 0 {
		t.Fatal("expected aggregated packages, got 0")
	}
	if elapsed > 2*time.Second {
		t.Errorf("prepareAggregatedData + repo license/use aggregation took %v for %d total repo packages, expected < 2s", elapsed, numRepos*pkgsPerRepo)
	}
}

func BenchmarkParseRepoSyntheticOverlay(b *testing.B) {
	memFS := buildSyntheticOverlayFS(4, 25, 2) // 100 packages, 200 ebuilds
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseRepo(memFS, ".", "synthetic-overlay", false, nil, SourceURL("https://example.com/synthetic-overlay.git"))
		if err != nil {
			b.Fatalf("parseRepo failed: %v", err)
		}
	}
}

func BenchmarkPrepareAggregatedData(b *testing.B) {
	memFS := buildSyntheticOverlayFS(4, 50, 2)
	site, err := parseRepo(memFS, ".", "synthetic-overlay", false, nil, SourceURL("https://example.com/synthetic-overlay.git"))
	if err != nil {
		b.Fatalf("parseRepo failed: %v", err)
	}
	aggPkgs := make(map[string]*AggPackage)
	for _, cat := range site.Categories {
		for _, pkg := range cat.Packages {
			aggPkgs[cat.Name+"/"+pkg.Name] = &AggPackage{
				Name:         pkg.Name,
				Category:     cat.Name,
				Repos:        map[string]*g2.SiteData{site.RepoName: site},
				RepoVersions: map[string][]g2.VersionData{site.RepoName: pkg.Versions},
			}
		}
	}
	site.AggEclasses = getRepoEclasses(site, aggPkgs)
	sites := []*g2.SiteData{site}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = prepareAggregatedData(sites)
	}
}

func BenchmarkPerformLintingCachedSyntax(b *testing.B) {
	rawText := `# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8

inherit distutils-r1

DESCRIPTION="Synthetic package for benchmarking"
HOMEPAGE="https://example.com/pkg"
SRC_URI="https://example.com/pkg-1.0.0.tar.gz"

LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64 ~arm64"
IUSE="test doc"
RESTRICT="!test? ( test )"

RDEPEND="dev-lang/python:3.12"
DEPEND="${RDEPEND}"

src_prepare() {
	default
	sed -i -e 's/[^[:alnum:]\]/_/g' Makefile || die
}
`
	eb := &g2.Ebuild{
		RawText: rawText,
		Vars: map[string]string{
			"EAPI":        "8",
			"PN":          "pkg",
			"PV":          "1.0.0",
			"P":           "pkg-1.0.0",
			"PF":          "pkg-1.0.0",
			"CATEGORY":    "dev-python",
			"DESCRIPTION": "Synthetic package for benchmarking",
			"HOMEPAGE":    "https://example.com/pkg",
			"SRC_URI":     "https://example.com/pkg-1.0.0.tar.gz",
			"LICENSE":     "MIT",
			"SLOT":        "0",
			"KEYWORDS":    "amd64 ~arm64",
			"IUSE":        "test doc",
			"RESTRICT":    "!test? ( test )",
			"RDEPEND":     "dev-lang/python:3.12",
			"DEPEND":      "dev-lang/python:3.12",
			"INHERITED":   "distutils-r1",
		},
	}
	pkg := &g2.PackageData{
		Name:     "pkg",
		Category: "dev-python",
		Versions: []g2.VersionData{
			{
				Version: "1.0.0",
				PVR:     "1.0.0",
				Ebuild:  eb,
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eb.ClearSyntaxCache()
		_ = lints.PerformLintingResults("virtual:bench", pkg)
	}
}
