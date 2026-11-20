package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func makeSyntheticUseFlags(count int, pkgCountPerFlag int) []*AggUseFlag {
	flags := make([]*AggUseFlag, 0, count)
	for i := 0; i < count; i++ {
		prefix := "a"
		switch i % 4 {
		case 1:
			prefix = "b"
		case 2:
			prefix = "9"
		case 3:
			prefix = "_"
		}
		name := fmt.Sprintf("%s-flag-%05d", prefix, i)
		pkgs := make([]*AggPackage, 0, pkgCountPerFlag)
		for p := 0; p < pkgCountPerFlag; p++ {
			pkgs = append(pkgs, &AggPackage{
				Category: "app-misc",
				Name:     fmt.Sprintf("pkg-%03d", p),
				Repos: map[string]*g2.SiteData{
					"repo-one": {RepoName: "repo-one"},
					"repo-two": {RepoName: "repo-two"},
				},
			})
		}
		gd := ""
		if i%2 == 0 {
			gd = "Description for " + name
		}
		flags = append(flags, &AggUseFlag{
			Name:          name,
			Count:         (count - i) + pkgCountPerFlag,
			GlobalDesc:    gd,
			LocalDescs:    map[string]string{},
			MetadataDescs: map[string]string{},
			Packages:      pkgs,
		})
	}
	return flags
}

func TestUseDirectoryThreeTiersAndPaginationHTTP(t *testing.T) {
	tmpl, err := GetSiteTemplates()
	if err != nil {
		t.Fatalf("GetSiteTemplates: %v", err)
	}
	cfg := UseTierConfig{
		SmallMax:  5,
		MediumMax: 15,
		PageSize:  4,
		TopLimit:  3,
	}

	// Tier 1: Small (< 5 flags) + invalid flag guard
	t.Run("tier1_small", func(t *testing.T) {
		flags := makeSyntheticUseFlags(3, 1)
		flags = append(flags, &AggUseFlag{Name: ""}, &AggUseFlag{Name: "bad/flag"})
		outDir := t.TempDir()
		usesRoot := filepath.Join(outDir, "uses")
		err := generateUseDirectoryTree(usesRoot, 1, []g2.Breadcrumb{{Name: "Home", URL: "../"}, {Name: "USE Flags"}}, "Site", "", tmpl, flags, "test", GenerationInfo{}, cfg)
		if err != nil {
			t.Fatalf("generateUseDirectoryTree tier1: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(usesRoot, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if !strings.Contains(s, "Total Distinct Flags:</strong> 3") || !strings.Contains(s, "a-flag-00000") {
			t.Fatalf("tier1 landing page missing expected flags/summary:\n%s", s)
		}
		if strings.Contains(s, "bad/flag") {
			t.Fatalf("invalid flag leaked into tier1")
		}
		if _, err := os.Stat(filepath.Join(usesRoot, "index")); !os.IsNotExist(err) {
			t.Fatalf("tier1 should not generate bucket index pages")
		}
	})

	// Tier 2: Medium (10 flags, bucket 'a' has > PageSize -> sharded, detail flag has 6 pkgs > PageSize -> paginated) + HTTP at / and /test-project/
	t.Run("tier2_medium_and_http", func(t *testing.T) {
		flags := make([]*AggUseFlag, 0, 10)
		// 6 flags in 'a' bucket (will shard into page 1 (4) and page 2 (2))
		for i := 0; i < 6; i++ {
			pkgs := make([]*AggPackage, 0, 6)
			for p := 0; p < 6; p++ {
				pkgs = append(pkgs, &AggPackage{
					Category: "app-misc",
					Name:     fmt.Sprintf("pkg-%02d", p),
					Repos:    map[string]*g2.SiteData{"repo-one": {RepoName: "repo-one"}},
				})
			}
			flags = append(flags, &AggUseFlag{
				Name:          fmt.Sprintf("alpha-%02d", i),
				Count:         6,
				GlobalDesc:    "Alpha desc",
				LocalDescs:    map[string]string{},
				MetadataDescs: map[string]string{},
				Packages:      pkgs,
			})
		}
		// 4 flags in '9' and 'z'
		for i := 0; i < 4; i++ {
			flags = append(flags, &AggUseFlag{
				Name:          fmt.Sprintf("9flag-%d", i),
				Count:         1,
				LocalDescs:    map[string]string{},
				MetadataDescs: map[string]string{},
			})
		}

		outDir := t.TempDir()
		// Create stub package and repo USE targets so relative links resolve cleanly
		for p := 0; p < 6; p++ {
			pDir := filepath.Join(outDir, "repos", "repo-one", "categories", "app-misc", "packages", fmt.Sprintf("pkg-%02d", p))
			if err := os.MkdirAll(pDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(pDir, "index.html"), []byte("<html><body>pkg</body></html>"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		repoUsesDir := filepath.Join(outDir, "repos", "repo-one", "uses", "alpha-00")
		if err := os.MkdirAll(repoUsesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoUsesDir, "index.html"), []byte("<html><body>repo use</body></html>"), 0o644); err != nil {
			t.Fatal(err)
		}

		usesRoot := filepath.Join(outDir, "uses")
		if err := generateUseDirectoryTree(usesRoot, 1, []g2.Breadcrumb{{Name: "Home", URL: "../"}, {Name: "USE Flags"}}, "Site", "", tmpl, flags, "test", GenerationInfo{}, cfg); err != nil {
			t.Fatalf("generateUseDirectoryTree tier2: %v", err)
		}

		for _, prefix := range []string{"/", "/test-project/"} {
			var srv *httptest.Server
			if prefix == "/" {
				srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
			} else {
				srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
			}

			mustGet := func(p string, wants ...string) string {
				t.Helper()
				resp, err := http.Get(srv.URL + prefix + p)
				if err != nil {
					srv.Close()
					t.Fatalf("GET %s%s: %v", prefix, p, err)
				}
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					srv.Close()
					t.Fatalf("GET %s%s status=%d", prefix, p, resp.StatusCode)
				}
				s := string(body)
				for _, w := range wants {
					if !strings.Contains(s, w) {
						srv.Close()
						t.Fatalf("%s%s missing %q", prefix, p, w)
					}
				}
				return s
			}

			landing := mustGet("uses/", "index/a/", "index/0-9/", "uses_index.json", "Showing alphabetical bucket index (10 flags)")
			if strings.Contains(landing, `href="alpha-05/"`) {
				srv.Close()
				t.Fatalf("tier2 landing should NOT inline full flag table rows")
			}

			mustGet("uses/index/a/", "alpha-00", "alpha-03", "page-2/")
			mustGet("uses/index/a/page-2/", "alpha-04", "alpha-05", "../../")
			mustGet("uses/alpha-00/", "Page 1 of 2", "page-2/", "repos/repo-one/uses/alpha-00/")
			mustGet("uses/alpha-00/page-2/", "Page 2 of 2", "app-misc/pkg-04", "app-misc/pkg-05")

			idxBody := mustGet("uses/uses_index.json")
			var idxEntries []UseIndexEntry
			if err := json.Unmarshal([]byte(idxBody), &idxEntries); err != nil {
				srv.Close()
				t.Fatalf("invalid uses_index.json: %v", err)
			}
			if len(idxEntries) != 10 {
				srv.Close()
				t.Fatalf("expected 10 entries in uses_index.json, got %d", len(idxEntries))
			}
			srv.Close()
		}
	})

	// Tier 3: Large (> 15 flags) -> reproducible TopLimit=3 + A-Z buckets
	t.Run("tier3_large", func(t *testing.T) {
		flags := makeSyntheticUseFlags(18, 1)
		outDir := t.TempDir()
		usesRoot := filepath.Join(outDir, "uses")
		if err := generateUseDirectoryTree(usesRoot, 1, []g2.Breadcrumb{{Name: "Home", URL: "../"}, {Name: "USE Flags"}}, "Site", "", tmpl, flags, "test", GenerationInfo{}, cfg); err != nil {
			t.Fatalf("generateUseDirectoryTree tier3: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(usesRoot, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if !strings.Contains(s, "Top 3 USE Flags") || !strings.Contains(s, "a-flag-00000") || !strings.Contains(s, "index/a/") {
			t.Fatalf("tier3 landing missing top-3 or bucket links:\n%s", s)
		}
		if strings.Contains(s, "a-flag-00016") {
			t.Fatalf("tier3 landing should not list lower-ranked flags outside Top-3")
		}
	})
}

func BenchmarkBuildUseDirectoryLarge(b *testing.B) {
	for _, n := range []int{5000, 27000} {
		b.Run(fmt.Sprintf("flags=%d", n), func(b *testing.B) {
			flags := makeSyntheticUseFlags(n, 2)
			cfg := defaultUseTierConfig()
			b.ResetTimer()
			var jsonSize int
			for i := 0; i < b.N; i++ {
				_, entries, _, _ := buildUseDirectoryData(flags, "Benchmark", cfg)
				raw, _ := json.Marshal(entries)
				jsonSize = len(raw)
			}
			b.ReportMetric(float64(jsonSize), "index_bytes")
		})
	}
}
