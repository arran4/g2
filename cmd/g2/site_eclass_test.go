package main

import (
	"testing"

	"github.com/arran4/g2"
)

func TestGetRepoEclassesAndPrepareAggregatedData(t *testing.T) {
	// Set up mock SiteData and AggPackages so Repos has elements
	site1 := &SiteData{
		RepoName: "repo1",
		Eclasses: []EclassData{
			{Name: "eclass1"},
		},
		Categories: []CategoryData{
			{
				Name: "cat1",
				Packages: []PackageData{
					{
						Name:     "pkgA",
						Category: "cat1",
						Versions: []VersionData{
							{
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"INHERITED": "eclass1 eclass2",
									},
								},
							},
						},
					},
					{
						Name:     "pkgB",
						Category: "cat1",
						Versions: []VersionData{
							{
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"INHERITED": "eclass2",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	site2 := &SiteData{
		RepoName: "repo2",
		Eclasses: []EclassData{
			{Name: "eclass2"},
		},
		Categories: []CategoryData{
			{
				Name: "cat1",
				Packages: []PackageData{
					{
						Name:     "pkgA",
						Category: "cat1",
						Versions: []VersionData{
							{
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"INHERITED": "eclass1",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pkgA1 := &AggPackage{Name: "pkgA", Category: "cat1", Repos: map[string]*SiteData{"repo1": site1}}
	pkgB1 := &AggPackage{Name: "pkgB", Category: "cat1", Repos: map[string]*SiteData{"repo1": site1}}

	aggPackagesMap1 := map[string]*AggPackage{
		"cat1/pkgA": pkgA1,
		"cat1/pkgB": pkgB1,
	}

	pkgA2 := &AggPackage{Name: "pkgA", Category: "cat1", Repos: map[string]*SiteData{"repo2": site2}}

	aggPackagesMap2 := map[string]*AggPackage{
		"cat1/pkgA": pkgA2,
	}


	// 1. Test getRepoEclasses for site1
	site1.AggEclasses = getRepoEclasses(site1, aggPackagesMap1)

	if len(site1.AggEclasses) != 2 {
		t.Fatalf("expected 2 eclasses for site1, got %d", len(site1.AggEclasses))
	}

	for _, ec := range site1.AggEclasses {
		if ec.Name == "eclass1" {
			if len(ec.Packages) != 1 || ec.Packages[0].Name != "pkgA" {
				t.Fatalf("expected 1 package (pkgA) for eclass1 in site1, got %d", len(ec.Packages))
			}
		} else if ec.Name == "eclass2" {
			if len(ec.Packages) != 2 {
				t.Fatalf("expected 2 packages for eclass2 in site1, got %d", len(ec.Packages))
			}
		}
	}

	// 2. Test getRepoEclasses for site2
	site2.AggEclasses = getRepoEclasses(site2, aggPackagesMap2)

	if len(site2.AggEclasses) != 2 {
		t.Fatalf("expected 2 eclasses for site2, got %d", len(site2.AggEclasses))
	}

	for _, ec := range site2.AggEclasses {
		if ec.Name == "eclass1" {
			if len(ec.Packages) != 1 || ec.Packages[0].Name != "pkgA" {
				t.Fatalf("expected 1 package (pkgA) for eclass1 in site2, got %d", len(ec.Packages))
			}
		} else if ec.Name == "eclass2" {
			if len(ec.Packages) != 0 {
				t.Fatalf("expected 0 packages for eclass2 in site2, got %d", len(ec.Packages))
			}
		}
	}

	// 3. Test prepareAggregatedData
	data := prepareAggregatedData([]*SiteData{site1, site2})

	if len(data.Eclasses) != 2 {
		t.Fatalf("expected 2 global eclasses, got %d", len(data.Eclasses))
	}

	for _, ec := range data.Eclasses {
		if ec.Name == "eclass1" {
			if len(ec.Packages) != 1 || ec.Packages[0].Name != "pkgA" {
				t.Fatalf("expected 1 package (pkgA) for global eclass1, got %d", len(ec.Packages))
			}
			if len(ec.Packages[0].Repos) != 2 {
				t.Fatalf("expected 2 repos for pkgA in global eclass1, got %d", len(ec.Packages[0].Repos))
			}
		} else if ec.Name == "eclass2" {
			if len(ec.Packages) != 2 {
				t.Fatalf("expected 2 packages for global eclass2, got %d", len(ec.Packages))
			}
			for _, pkg := range ec.Packages {
				if pkg.Name == "pkgA" {
					if len(pkg.Repos) != 1 || pkg.Repos["repo1"] == nil {
						t.Fatalf("expected 1 repo (repo1) for pkgA in global eclass2, got %d", len(pkg.Repos))
					}
				} else if pkg.Name == "pkgB" {
					if len(pkg.Repos) != 1 || pkg.Repos["repo1"] == nil {
						t.Fatalf("expected 1 repo (repo1) for pkgB in global eclass2, got %d", len(pkg.Repos))
					}
				}
			}
		}
	}
}
