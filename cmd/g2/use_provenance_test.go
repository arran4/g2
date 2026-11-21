package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
)

func buildProvenanceFixtures(t *testing.T) (*g2.SiteData, *g2.SiteData) {
	t.Helper()
	fsAlpha := fstest.MapFS{
		"profiles/repo_name":               {Data: []byte("repo-alpha\n")},
		"profiles/use.desc":                {Data: []byte("shared-global - Alpha definition\nunused-global - Unused global flag\n")},
		"profiles/use.local.desc":          {Data: []byte("app-misc/same:local-only - Local desc in alpha\n")},
		"profiles/desc/cpu_flags_x86.desc": {Data: []byte("avx2 - Enable AVX2 instructions\n")},
		"app-misc/same/same-1.0.ebuild":    {Data: []byte("EAPI=8\nSLOT=\"0\"\nIUSE=\"shared-global local-only multi-cat-local cpu_flags_x86_avx2 only-iuse\"\nREQUIRED_USE=\"only-req? ( shared-global )\"\n")},
		"dev-util/other/other-1.0.ebuild":  {Data: []byte("EAPI=8\nSLOT=\"0\"\nIUSE=\"multi-cat-local\"\n")},
		"app-misc/same/metadata.xml": {
			Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<pkgmetadata><use><flag name="multi-cat-local">Multi category package flag</flag></use></pkgmetadata>`),
		},
	}
	fsBeta := fstest.MapFS{
		"profiles/repo_name":            {Data: []byte("repo-beta\n")},
		"profiles/use.desc":             {Data: []byte("shared-global - Beta conflicting definition\n")},
		"app-misc/same/same-2.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\nIUSE=\"shared-global +bad/ignored\"\n")},
		"app-misc/same/metadata.xml": {
			Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<pkgmetadata><use><flag name="local-only">Beta metadata desc</flag></use></pkgmetadata>`),
		},
	}

	sA, err := parseRepo(fsAlpha, ".", "repo-alpha", false, nil, SourceURL("https://example.com/alpha.git"))
	if err != nil {
		t.Fatal(err)
	}
	sB, err := parseRepo(fsBeta, ".", "repo-beta", false, nil, SourceURL("https://example.com/beta.git"))
	if err != nil {
		t.Fatal(err)
	}
	return sA, sB
}

func TestUseProvenanceSemanticsAndDeterminism(t *testing.T) {
	sA, sB := buildProvenanceFixtures(t)

	dAB := prepareAggregatedData([]*g2.SiteData{sA, sB})
	dBA := prepareAggregatedData([]*g2.SiteData{sB, sA})

	if !reflect.DeepEqual(dAB.UseFlags, dBA.UseFlags) {
		t.Fatalf("AggregateUseFlags must be deterministic regardless of site slice iteration order")
	}

	byName := make(map[string]*AggUseFlag)
	for _, f := range dAB.UseFlags {
		byName[f.Name] = f
	}

	if _, bad := byName["bad/ignored"]; bad {
		t.Fatalf("invalid flag name bad/ignored must be rejected")
	}

	// 1. Conflicting use.desc across 2 repos preserved without overwriting
	shared := byName["shared-global"]
	if shared == nil || shared.Scope != ScopeGlobalDefined {
		t.Fatalf("expected shared-global to have ScopeGlobalDefined, got %+v", shared)
	}
	if !shared.HasConflictingGlobalDescs || len(shared.GlobalDefs) != 2 {
		t.Fatalf("expected 2 conflicting GlobalDefs preserved for shared-global, got %+v", shared.GlobalDefs)
	}
	if shared.GlobalDefs[0].RepoName != "repo-alpha" || shared.GlobalDefs[1].RepoName != "repo-beta" {
		t.Fatalf("expected deterministic repo order in GlobalDefs: %+v", shared.GlobalDefs)
	}
	if shared.DistinctPackageCount != 1 || shared.PackageInstanceCount != 2 {
		t.Fatalf("expected distinct=1 instances=2 for app-misc/same across 2 repos, got distinct=%d instances=%d", shared.DistinctPackageCount, shared.PackageInstanceCount)
	}

	// 2. Flag used in 2 categories WITHOUT use.desc MUST NOT be ScopeGlobalDefined
	multiCat := byName["multi-cat-local"]
	if multiCat == nil || multiCat.Scope != ScopePackageScoped {
		t.Fatalf("expected multi-cat-local without use.desc to be ScopePackageScoped (not global!), got %+v", multiCat)
	}

	// 3. USE_EXPAND flag
	avx := byName["cpu_flags_x86_avx2"]
	if avx == nil || avx.Scope != ScopeUseExpand {
		t.Fatalf("expected cpu_flags_x86_avx2 to have ScopeUseExpand, got %+v", avx)
	}

	// 4. Undocumented IUSE / REQUIRED_USE flags
	for _, uname := range []string{"only-iuse", "only-req"} {
		uf := byName[uname]
		if uf == nil || uf.Scope != ScopeUndocumented {
			t.Fatalf("expected %s to have ScopeUndocumented, got %+v", uname, uf)
		}
	}

	// 5. Unused use.desc flag (0 packages)
	unused := byName["unused-global"]
	if unused == nil || unused.Scope != ScopeGlobalDefined || unused.DistinctPackageCount != 0 {
		t.Fatalf("expected unused-global to be ScopeGlobalDefined with 0 packages, got %+v", unused)
	}

	// 6. Provenance grouping check
	groups := groupUseFlagsByProvenanceFunc(dAB.UseFlags)
	if len(groups.GlobalDefined) != 2 || len(groups.UseExpandDefined) != 1 || len(groups.PackageScoped) != 2 || len(groups.Undocumented) != 2 {
		t.Fatalf("unexpected provenance grouping counts: global=%d expand=%d pkg=%d undoc=%d",
			len(groups.GlobalDefined), len(groups.UseExpandDefined), len(groups.PackageScoped), len(groups.Undocumented))
	}
}

func TestUseProvenanceHTTPPagesAndLinks(t *testing.T) {
	sA, sB := buildProvenanceFixtures(t)
	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{sA, sB}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	pkgLinkRe := regexp.MustCompile(`href="([^"]*repos/[^"]+/categories/[^"]+/packages/[^"]+/)"`)

	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		for _, pagePath := range []string{
			"uses/",
			"uses/shared-global/",
			"uses/multi-cat-local/",
			"repos/repo-alpha/uses/",
			"repos/repo-alpha/uses/shared-global/",
		} {
			pageURLStr := srv.URL + prefix + pagePath
			resp, err := http.Get(pageURLStr)
			if err != nil {
				srv.Close()
				t.Fatalf("GET %s: %v", pageURLStr, err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				srv.Close()
				t.Fatalf("GET %s status=%d", pageURLStr, resp.StatusCode)
			}
			s := string(body)
			pageURL, _ := url.Parse(pageURLStr)
			for _, m := range pkgLinkRe.FindAllStringSubmatch(s, -1) {
				rel, err := url.Parse(m[1])
				if err != nil {
					srv.Close()
					t.Fatal(err)
				}
				target := pageURL.ResolveReference(rel)
				if prefix != "/" && !strings.HasPrefix(target.Path, prefix) {
					srv.Close()
					t.Fatalf("link %q on %s escaped mount %s -> %s", m[1], pagePath, prefix, target.String())
				}
				tr, err := http.Get(target.String())
				if err != nil {
					srv.Close()
					t.Fatal(err)
				}
				_ = tr.Body.Close()
				if tr.StatusCode != http.StatusOK {
					srv.Close()
					t.Fatalf("package link %s from %s returned %d", target.String(), pagePath, tr.StatusCode)
				}
			}
		}

		// Verify conflicting definitions appear on shared-global detail page
		resp, err := http.Get(srv.URL + prefix + "uses/shared-global/")
		if err != nil {
			srv.Close()
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		s := string(b)
		if !strings.Contains(s, "Alpha definition") || !strings.Contains(s, "Beta conflicting definition") {
			srv.Close()
			t.Fatalf("expected both conflicting descriptions preserved on uses/shared-global/")
		}
		srv.Close()
	}
}
