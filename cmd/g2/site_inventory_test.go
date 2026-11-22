package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
)

func TestComputeSiteInventoryEdgeCasesAndTieBreaking(t *testing.T) {
	// 0 repos
	empty := computeSiteInventoryStats(nil, &AggregatedData{})
	if empty.TotalRepos != 0 || empty.UniquePackages != 0 || empty.AvgPackagesPerRepoStr != "0.0" {
		t.Fatalf("unexpected 0-repo inventory: %+v", empty)
	}

	// Multiple repos with ties, empty repo, zero-news repo, duplicate package, multi-lang news, missing dates
	memEmpty := fstest.MapFS{"profiles/repo_name": {Data: []byte("alpha-empty\n")}}
	memBeta := fstest.MapFS{
		"profiles/repo_name": {Data: []byte("beta-repo\n")},
		"metadata/news/2026-02-01-n1/2026-02-01-n1.en.txt": {
			Data: []byte("Title: N1\nAuthor: Jane Doe <jane@example.com>\nPosted: 2026-02-01\nRevision: 1\nNews-Item-Format: 2.0\n\nBody\n"),
		},
		"metadata/news/2026-02-01-n1/2026-02-01-n1.de.txt": {
			Data: []byte("Title: N1 DE\nAuthor: Jane Doe <jane@example.com>\nPosted: 2026-02-01\nRevision: 1\nNews-Item-Format: 2.0\n\nInhalt\n"),
		},
		"app-misc/shared/shared-1.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
		"app-misc/shared/shared-2.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
	}
	memGamma := fstest.MapFS{
		"profiles/repo_name": {Data: []byte("gamma-repo\n")},
		"metadata/news/2026-02-01-n2/2026-02-01-n2.en.txt": {
			Data: []byte("Title: N2\nAuthor: Jane Doe <jane@example.com>\nRevision: 1\nNews-Item-Format: 2.0\n\nNo posted date\n"),
		},
		"app-misc/shared/shared-3.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
		"app-misc/gamma-only/gamma-only-1.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
	}

	sEmpty, err := parseRepo(memEmpty, ".", "alpha-empty", false, nil, SourceURL("https://example.com/alpha.git"))
	if err != nil {
		t.Fatal(err)
	}
	sBeta, err := parseRepo(memBeta, ".", "beta-repo", false, nil, SourceURL("https://example.com/beta.git"))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an additional translated news variant in the same news directory.
	sBeta.News = append(sBeta.News, g2.NewsItem{
		Title:    "N1 DE",
		Posted:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		DirName:  "2026-02-01-n1",
		FileName: "2026-02-01-n1.de.txt",
	})
	sGamma, err := parseRepo(memGamma, ".", "gamma-repo", false, nil, SourceURL("https://example.com/gamma.git"))
	if err != nil {
		t.Fatal(err)
	}

	sites := []*g2.SiteData{sGamma, sEmpty, sBeta}
	data := prepareAggregatedData(sites)
	inv := computeSiteInventoryStats(sites, data)

	if inv.TotalRepos != 3 {
		t.Fatalf("expected 3 repos, got %d", inv.TotalRepos)
	}
	if inv.UniquePackages != 2 {
		t.Fatalf("expected 2 unique packages (shared + gamma-only), got %d", inv.UniquePackages)
	}
	if inv.TotalRepoPackages != 3 {
		t.Fatalf("expected 3 per-repo packages sum, got %d", inv.TotalRepoPackages)
	}
	if inv.TotalEbuilds != 4 {
		t.Fatalf("expected 4 ebuilds, got %d", inv.TotalEbuilds)
	}
	if inv.TotalNewsItems != 2 || inv.TotalNewsVariants != 3 {
		t.Fatalf("expected 2 news items / 3 variants, got %d / %d", inv.TotalNewsItems, inv.TotalNewsVariants)
	}
	// Tie-break on TopReposByPackages: beta-repo (1) vs gamma-repo (2) -> gamma-repo first, then beta-repo, then alpha-empty
	if len(inv.TopReposByPackages) != 3 || inv.TopReposByPackages[0].Name != "gamma-repo" || inv.TopReposByPackages[1].Name != "beta-repo" {
		t.Fatalf("unexpected TopReposByPackages order: %+v", inv.TopReposByPackages)
	}
	// Tie-break on TopReposByNews: beta-repo (1) vs gamma-repo (1) -> beta-repo before gamma-repo
	if len(inv.TopReposByNews) != 3 || inv.TopReposByNews[0].Name != "beta-repo" || inv.TopReposByNews[1].Name != "gamma-repo" {
		t.Fatalf("unexpected TopReposByNews tie-break: %+v", inv.TopReposByNews)
	}
	if inv.TopPackagesByVersions[0].Name != "app-misc/shared" || inv.TopPackagesByVersions[0].Count != 3 {
		t.Fatalf("unexpected TopPackagesByVersions: %+v", inv.TopPackagesByVersions)
	}
	if inv.TopPackagesByRepos[0].Name != "app-misc/shared" || inv.TopPackagesByRepos[0].Count != 2 {
		t.Fatalf("unexpected TopPackagesByRepos: %+v", inv.TopPackagesByRepos)
	}
}

func TestSiteInventoryHTTPStatsAndRepoDashboard(t *testing.T) {
	memFS := fstest.MapFS{
		"profiles/repo_name":                {Data: []byte("inv-overlay\n")},
		"app-misc/hello/hello-1.0.ebuild":   {Data: []byte("EAPI=8\nDESCRIPTION=\"Hello\"\nSLOT=\"0\"\nKEYWORDS=\"~amd64\"\n")},
		"app-misc/hello/hello-2.0.ebuild":   {Data: []byte("EAPI=8\nDESCRIPTION=\"Hello 2\"\nSLOT=\"0\"\nKEYWORDS=\"~amd64\"\n")},
		"metadata/news/2026-03-01-hi/2026-03-01-hi.en.txt": {
			Data: []byte("Title: Hi\nAuthor: Jane Doe <jane@example.com>\nPosted: 2026-03-01\nRevision: 1\nNews-Item-Format: 2.0\n\nHello\n"),
		},
	}
	site, err := parseRepo(memFS, ".", "inv-overlay", false, nil, SourceURL("https://example.com/inv.git"))
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{site}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		for _, tc := range []struct {
			path string
			want []string
		}{
			{
				path: "stats/",
				want: []string{"Site Inventory", "Leaderboards / Explore", "Per-Repository Breakdown", "repos/inv-overlay/stats/"},
			},
			{
				path: "repos/inv-overlay/",
				want: []string{"Repository Summary", "Full Repository Statistics", "2026-03-01"},
			},
		} {
			resp, err := http.Get(srv.URL + prefix + tc.path)
			if err != nil {
				srv.Close()
				t.Fatalf("GET %s%s: %v", prefix, tc.path, err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				srv.Close()
				t.Fatalf("GET %s%s status=%d", prefix, tc.path, resp.StatusCode)
			}
			s := string(body)
			for _, w := range tc.want {
				if !strings.Contains(s, w) {
					srv.Close()
					t.Fatalf("page %s%s missing %q", prefix, tc.path, w)
				}
			}
		}
		srv.Close()
	}
}

func BenchmarkComputeSiteInventory(b *testing.B) {
	sites := make([]*g2.SiteData, 0, 10)
	for r := 0; r < 10; r++ {
		cats := make([]g2.CategoryData, 1)
		pkgs := make([]g2.PackageData, 0, 50)
		for p := 0; p < 50; p++ {
			pkgs = append(pkgs, g2.PackageData{
				Name: fmt.Sprintf("pkg-%03d", p),
				Versions: []g2.VersionData{
					{Version: "1.0"},
					{Version: "2.0"},
				},
			})
		}
		cats[0] = g2.CategoryData{Name: "app-misc", Packages: pkgs}
		sites = append(sites, &g2.SiteData{
			RepoName:     fmt.Sprintf("repo-%02d", r),
			Categories:   cats,
			PackageCount: 50,
		})
	}
	data := prepareAggregatedData(sites)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = computeSiteInventoryStats(sites, data)
	}
}
