package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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

func TestUseDirectoryWithRealProvenanceAndCrossOverlayCollisions(t *testing.T) {
	fsAlpha := fstest.MapFS{
		"profiles/repo_name":               {Data: []byte("overlay-alpha\n")},
		"profiles/use.desc":                {Data: []byte("shared-global - Alpha global description\n")},
		"profiles/desc/video_cards.desc":   {Data: []byte("amdgpu - AMD GPU card\n")},
		"profiles/use.local.desc":          {Data: []byte("app-misc/shared-pkg:custom - Alpha local desc\n")},
		"app-misc/shared-pkg/shared-pkg-1.0.ebuild": {
			Data: []byte("EAPI=8\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"shared-global custom video_cards_amdgpu only-iuse\"\nREQUIRED_USE=\"shared-global? ( custom )\"\n"),
		},
		"app-misc/shared-pkg/metadata.xml": {
			Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><pkgmetadata><use><flag name="custom">Alpha metadata desc</flag></use></pkgmetadata>`),
		},
		"app-misc/alpha-only/alpha-only-1.0.ebuild": {
			Data: []byte("EAPI=8\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"shared-global\"\n"),
		},
	}
	fsBeta := fstest.MapFS{
		"profiles/repo_name": {Data: []byte("overlay-beta\n")},
		"profiles/use.desc":  {Data: []byte("shared-global - Beta conflicting global description\n")},
		"app-misc/shared-pkg/shared-pkg-2.0.ebuild": {
			Data: []byte("EAPI=8\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"shared-global\"\n"),
		},
		"app-misc/shared-pkg/metadata.xml": {
			Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><pkgmetadata><use><flag name="beta-only">Beta metadata desc</flag></use></pkgmetadata>`),
		},
		"app-misc/beta-only/beta-only-1.0.ebuild": {
			Data: []byte("EAPI=8\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"beta-only\"\n"),
		},
	}

	sAlpha, err := parseRepo(fsAlpha, ".", "overlay-alpha", false, nil, SourceURL("https://example.com/alpha.git"))
	if err != nil {
		t.Fatalf("parseRepo alpha: %v", err)
	}
	sBeta, err := parseRepo(fsBeta, ".", "overlay-beta", false, nil, SourceURL("https://example.com/beta.git"))
	if err != nil {
		t.Fatalf("parseRepo beta: %v", err)
	}

	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{sAlpha, sBeta}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite: %v", err)
	}

	// Check uses_index.json entries preserve authoritative scope, distinct packages, and repo instances.
	idxBytes, err := os.ReadFile(filepath.Join(outDir, "uses", "uses_index.json"))
	if err != nil {
		t.Fatalf("read uses_index.json: %v", err)
	}
	var idx []UseIndexEntry
	if err := json.Unmarshal(idxBytes, &idx); err != nil {
		t.Fatalf("unmarshal uses_index.json: %v", err)
	}
	byName := make(map[string]UseIndexEntry, len(idx))
	for _, e := range idx {
		byName[e.N] = e
	}
	if got := byName["shared-global"]; got.S != "use.desc" || got.C != 2 || got.I != 3 {
		t.Fatalf("shared-global index entry = %+v, want S=use.desc C=2 I=3", got)
	}
	if got := byName["video_cards_amdgpu"]; got.S != "use_expand" || got.C != 1 || got.I != 1 {
		t.Fatalf("video_cards_amdgpu index entry = %+v, want S=use_expand C=1 I=1", got)
	}
	if got := byName["custom"]; got.S != "local/metadata" || got.C != 1 || got.I != 1 {
		t.Fatalf("custom index entry = %+v, want S=local/metadata C=1 I=1", got)
	}
	if got := byName["beta-only"]; got.S != "local/metadata" || got.C != 2 || got.I != 2 {
		t.Fatalf("beta-only index entry = %+v, want S=local/metadata C=2 I=2", got)
	}
	if got := byName["only-iuse"]; got.S != "undocumented" || got.C != 1 || got.I != 1 {
		t.Fatalf("only-iuse index entry = %+v, want S=undocumented C=1 I=1", got)
	}

	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		getOK := func(p string) string {
			t.Helper()
			resp, err := http.Get(srv.URL + prefix + p)
			if err != nil {
				srv.Close()
				t.Fatalf("GET %s%s: %v", prefix, p, err)
			}
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				srv.Close()
				t.Fatalf("GET %s%s status = %d", prefix, p, resp.StatusCode)
			}
			return string(b)
		}

		landing := getOK("uses/")
		for _, want := range []string{
			"<strong>Total Distinct Flags:</strong> 5",
			"<strong>Global (use.desc):</strong> 1",
			"<strong>USE_EXPAND (profiles/desc/*.desc):</strong> 1",
			"<strong>Package-Scoped (use.local.desc / metadata.xml):</strong> 2",
			"<strong>Undocumented (IUSE / REQUIRED_USE):</strong> 1",
		} {
			if !strings.Contains(landing, want) {
				srv.Close()
				t.Fatalf("%suses/ missing %q:\n%s", prefix, want, landing)
			}
		}

		sharedPage := getOK("uses/shared-global/")
		for _, want := range []string{
			"Alpha global description",
			"Beta conflicting global description",
			"overlay-alpha (2)",
			"overlay-beta (1)",
			"repos/overlay-alpha/categories/app-misc/packages/shared-pkg/",
			"repos/overlay-beta/categories/app-misc/packages/shared-pkg/",
			"repos/overlay-alpha/categories/app-misc/packages/alpha-only/",
		} {
			if !strings.Contains(sharedPage, want) {
				srv.Close()
				t.Fatalf("%suses/shared-global/ missing %q", prefix, want)
			}
		}

		// custom is only used by app-misc/shared-pkg in overlay-alpha, never overlay-beta
		customPage := getOK("uses/custom/")
		if strings.Contains(customPage, "overlay-beta") {
			srv.Close()
			t.Fatalf("%suses/custom/ falsely attributed app-misc/shared-pkg to overlay-beta", prefix)
		}
		if !strings.Contains(customPage, "IUSE + REQUIRED_USE") || !strings.Contains(customPage, "overlay-alpha (1)") {
			srv.Close()
			t.Fatalf("%suses/custom/ missing IUSE+REQUIRED_USE provenance or overlay-alpha count", prefix)
		}

		// beta-only is used by shared-pkg (metadata only) and beta-only (IUSE) in overlay-beta only
		betaOnlyPage := getOK("uses/beta-only/")
		if strings.Contains(betaOnlyPage, "overlay-alpha") {
			srv.Close()
			t.Fatalf("%suses/beta-only/ falsely attributed app-misc/shared-pkg to overlay-alpha", prefix)
		}
		if !strings.Contains(betaOnlyPage, "overlay-beta (2)") || !strings.Contains(betaOnlyPage, "metadata only") {
			srv.Close()
			t.Fatalf("%suses/beta-only/ missing overlay-beta (2) or metadata-only provenance", prefix)
		}

		// Repo-scoped USE detail pages keep both provenance and cross-overlay link
		alphaShared := getOK("repos/overlay-alpha/uses/shared-global/")
		if !strings.Contains(alphaShared, "View across all overlays") || !strings.Contains(alphaShared, "Package Usage Occurrences (2)") {
			srv.Close()
			t.Fatalf("%srepos/overlay-alpha/uses/shared-global/ missing tiered detail header:\n%s", prefix, alphaShared)
		}
		betaShared := getOK("repos/overlay-beta/uses/shared-global/")
		if !strings.Contains(betaShared, "View across all overlays") || !strings.Contains(betaShared, "Package Usage Occurrences (1)") {
			srv.Close()
			t.Fatalf("%srepos/overlay-beta/uses/shared-global/ missing tiered detail header:\n%s", prefix, betaShared)
		}

		// Verify a relative link inside uses/shared-global/ resolves without escaping /test-project/
		pageURL, _ := url.Parse(srv.URL + prefix + "uses/shared-global/")
		rel, _ := url.Parse("../../repos/overlay-beta/categories/app-misc/packages/shared-pkg/")
		resolved := pageURL.ResolveReference(rel)
		if prefix != "/" && !strings.HasPrefix(resolved.Path, prefix) {
			srv.Close()
			t.Fatalf("relative link escaped %s: %s", prefix, resolved.String())
		}
		tr, err := http.Get(resolved.String())
		if err != nil {
			srv.Close()
			t.Fatal(err)
		}
		_ = tr.Body.Close()
		if tr.StatusCode != http.StatusOK {
			srv.Close()
			t.Fatalf("GET %s status = %d", resolved.String(), tr.StatusCode)
		}

		srv.Close()
	}
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
