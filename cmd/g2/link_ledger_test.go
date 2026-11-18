package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
)

func buildRichTwoRepoFixture(t *testing.T) []*g2.SiteData {
	t.Helper()

	alphaFS := fstest.MapFS{
		"profiles/repo_name":                       &fstest.MapFile{Data: []byte("alpha-ov\n")},
		"profiles/use.desc":                        &fstest.MapFile{Data: []byte("foo - Enable foo\nunusedglobal - Unused flag\n")},
		"profiles/use.local.desc":                  &fstest.MapFile{Data: []byte("app-misc/solo:localonly - Local desc\n")},
		"profiles/desc/video_cards.desc":           &fstest.MapFile{Data: []byte("dummy - Dummy card\nunusedcard - Unused card\n")},
		"profiles/arch.list":                       &fstest.MapFile{Data: []byte("amd64\narm64\n")},
		"profiles/profiles.desc":                   &fstest.MapFile{Data: []byte("amd64 default/linux/amd64/23.0 stable\n")},
		"profiles/default/linux/amd64/23.0/eapi":   &fstest.MapFile{Data: []byte("8\n")},
		"profiles/default/linux/amd64/23.0/parent": &fstest.MapFile{Data: []byte("..\ngentoo:default/linux/amd64/23.0\n")},
		"profiles/package.mask":                    &fstest.MapFile{Data: []byte("# Jane Doe <jane@example.com> (2026-01-01)\n# Test mask\n>=app-misc/solo-2.0\nsys-apps/external-missing\n")},
		"profiles/package.deprecated":              &fstest.MapFile{Data: []byte("# Jane Doe <jane@example.com> (2026-01-02)\n# Test deprecated\napp-misc/shared\n")},
		"profiles/updates/1Q-2026":                 &fstest.MapFile{Data: []byte("move app-misc/oldsolo app-misc/solo\n")},
		"licenses/ALPHA-LIC":                       &fstest.MapFile{Data: []byte("Alpha License Text\n")},
		"eclass/alpha-utils.eclass":                &fstest.MapFile{Data: []byte("# alpha eclass\n")},
		"metadata/news/2026-02-01-alpha-news/2026-02-01-alpha-news.en.txt": &fstest.MapFile{Data: []byte(
			"Title: Alpha News Update\nAuthor: Jane Doe <jane@example.com>\nPosted: 2026-02-01\nRevision: 1\nNews-Item-Format: 2.0\n\nAlpha news body.\n",
		)},
		"app-misc/solo/metadata.xml": &fstest.MapFile{Data: []byte(
			"<pkgmetadata><use><flag name=\"localonly\">Enable local feature</flag></use></pkgmetadata>",
		)},
		"app-misc/solo/solo-1.0-r1.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Solo widget\"\nHOMEPAGE=\"https://example.com/solo\"\nSLOT=\"0\"\nLICENSE=\"ALPHA-LIC\"\nKEYWORDS=\"amd64 ~arm64\"\nIUSE=\"+foo localonly video_cards_dummy + . ..\"\nREQUIRED_USE=\"^^ ( foo localonly )\"\nDEPEND=\"app-misc/shared sys-libs/glibc\"\n",
		)},
		"app-misc/shared/shared-1.0.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Shared pkg in alpha\"\nHOMEPAGE=\"https://example.com/shared\"\nSLOT=\"0\"\nLICENSE=\"MIT\"\nKEYWORDS=\"amd64\"\nIUSE=\"foo bar\"\n",
		)},
	}

	betaFS := fstest.MapFS{
		"profiles/repo_name": &fstest.MapFile{Data: []byte("beta-ov\n")},
		"profiles/arch.list": &fstest.MapFile{Data: []byte("amd64\n")},
		"app-misc/shared/shared-2.0.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Shared pkg in beta\"\nHOMEPAGE=\"https://example.com/shared-beta\"\nSLOT=\"0\"\nLICENSE=\"MIT\"\nKEYWORDS=\"~amd64\"\nIUSE=\"foo baz\"\n",
		)},
	}

	s1, err := parseRepo(alphaFS, ".", "alpha-ov", false, nil)
	if err != nil {
		t.Fatalf("parseRepo alpha: %v", err)
	}
	s2, err := parseRepo(betaFS, ".", "beta-ov", false, nil)
	if err != nil {
		t.Fatalf("parseRepo beta: %v", err)
	}
	return []*g2.SiteData{s1, s2}
}

func TestLinkIntegrityLedgerCleanRichSiteStrictAndHTTP(t *testing.T) {
	sites := buildRichTwoRepoFixture(t)
	outDir := t.TempDir()

	genInfo := GenerationInfo{
		StrictLinks: true,
		Args:        []string{"g2", "overlays", "site", "generate", "-strict-links"},
	}
	if err := generateSite(outDir, sites, 90*24*time.Hour, "3 months", genInfo); err != nil {
		t.Fatalf("generateSite with StrictLinks=true failed: %v", err)
	}

	statsBytes, err := os.ReadFile(filepath.Join(outDir, "stats", "index.html"))
	if err != nil {
		t.Fatalf("reading stats/index.html: %v", err)
	}
	statsHTML := string(statsBytes)
	if !strings.Contains(statsHTML, "Link Integrity Verification") || !strings.Contains(statsHTML, "Verified (0 issues)") {
		t.Errorf("stats page missing verified link integrity summary:\n%s", statsHTML)
	}
	if strings.Contains(statsHTML, outDir) {
		t.Errorf("stats page leaked local filesystem outDir %q", outDir)
	}

	mux := http.NewServeMux()
	fsHandler := http.FileServer(http.Dir(outDir))
	mux.Handle("/test-project/", http.StripPrefix("/test-project/", fsHandler))
	mux.Handle("/", fsHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, base := range []string{srv.URL + "/", srv.URL + "/test-project/"} {
		for _, rel := range []string{
			"",
			"stats/",
			"help/",
			"search/",
			"eclasses/",
			"packages/app-misc/",
			"packages/app-misc/solo/",
			"packages/app-misc/shared/",
			"packages/app-misc/oldsolo/",
			"repos/alpha-ov/categories/app-misc/",
			"repos/alpha-ov/categories/app-misc/packages/solo/",
			"repos/alpha-ov/categories/app-misc/packages/solo/ebuild/",
			"repos/alpha-ov/categories/app-misc/packages/solo/ebuild/1.0/",
			"repos/alpha-ov/deprecated/",
			"repos/alpha-ov/masked/",
			"repos/alpha-ov/uses/foo/",
			"repos/alpha-ov/uses_expand/video_cards/",
		} {
			resp, err := http.Get(base + rel)
			if err != nil {
				t.Fatalf("GET %s%s failed: %v", base, rel, err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("GET %s%s returned %d (body len %d)", base, rel, resp.StatusCode, len(body))
			}
		}
	}
}

func TestLinkLedgerFaultInjection(t *testing.T) {
	t.Run("detects missing, root-escape, malformed, duplicate, case-collision, and nested doctype", func(t *testing.T) {
		ledger := NewLinkLedger("/tmp/fake-site")

		if !ledger.ReservePage("index.html") {
			t.Fatal("first ReservePage(index.html) failed")
		}
		if ledger.ReservePage("index.html") {
			t.Fatal("expected duplicate ReservePage(index.html) to return false")
		}

		ledger.ReservePage("Uses/Foo/index.html")
		ledger.CommitPage("Uses/Foo/index.html", 10)
		ledger.ReservePage("uses/foo/index.html")
		ledger.CommitPage("uses/foo/index.html", 10)

		ledger.ReservePage("nested/index.html")
		ledger.CommitPage("nested/index.html", 200)
		ledger.ScanHTML("nested/index.html", []byte(`<!DOCTYPE html><html><head></head><body><!DOCTYPE html><html><head></head><body>nested</body></html></body></html>`))

		badHTML := []byte(`<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0; url=missing-redirect/"></head><body>
			<a href="missing/page/">missing</a>
			<a href="/absolute/path/">root slash</a>
			<a href="../../outside/">dotdot escape</a>
			<a href="javascript:alert(1)">xss scheme</a>
			<a href="http://%zz">malformed</a>
			<a href="https://example.com/ok">external ok</a>
			<a href="#top">fragment ok</a>
			<script src="search/search.js"></script>
		</body></html>`)
		ledger.CommitPage("index.html", int64(len(badHTML)))
		ledger.ScanHTML("index.html", badHTML)

		report := ledger.Verify()
		if report.Verified {
			t.Fatal("expected report.Verified == false for injected faults")
		}
		if report.DuplicateCount != 1 {
			t.Errorf("DuplicateCount = %d, want 1", report.DuplicateCount)
		}
		if report.CaseCollisionCount != 1 {
			t.Errorf("CaseCollisionCount = %d, want 1", report.CaseCollisionCount)
		}
		if report.RootEscapeCount != 2 {
			t.Errorf("RootEscapeCount = %d, want 2", report.RootEscapeCount)
		}
		if report.MalformedCount != 2 {
			t.Errorf("MalformedCount = %d, want 2", report.MalformedCount)
		}
		if report.MissingCount != 3 { // missing/page/index.html, missing-redirect/index.html, search/search.js
			t.Errorf("MissingCount = %d, want 3 (issues=%+v)", report.MissingCount, report.Issues)
		}
		if report.StructureCount != 1 {
			t.Errorf("StructureCount = %d, want 1", report.StructureCount)
		}
		if report.ExternalRefs != 1 {
			t.Errorf("ExternalRefs = %d, want 1", report.ExternalRefs)
		}
		if report.FragmentOnlyRefs != 1 {
			t.Errorf("FragmentOnlyRefs = %d, want 1", report.FragmentOnlyRefs)
		}
	})

	t.Run("default mode reports without failing while strict mode fails nonzero", func(t *testing.T) {
		sites := buildRichTwoRepoFixture(t)

		outNonStrict := t.TempDir()
		ledgerNonStrict := NewLinkLedger(outNonStrict)
		ledgerNonStrict.ReservePage("bad/index.html")
		ledgerNonStrict.CommitPage("bad/index.html", 50)
		ledgerNonStrict.ScanHTML("bad/index.html", []byte(`<!DOCTYPE html><html><head></head><body><a href="../nonexistent/">broken</a></body></html>`))
		if err := generateSite(outNonStrict, sites, 90*24*time.Hour, "3 months", GenerationInfo{
			Ledger:      ledgerNonStrict,
			StrictLinks: false,
		}); err != nil {
			t.Fatalf("expected non-strict generateSite to succeed despite broken link, got: %v", err)
		}

		outStrict := t.TempDir()
		ledgerStrict := NewLinkLedger(outStrict)
		ledgerStrict.ReservePage("bad/index.html")
		ledgerStrict.CommitPage("bad/index.html", 50)
		ledgerStrict.ScanHTML("bad/index.html", []byte(`<!DOCTYPE html><html><head></head><body><a href="../nonexistent/">broken</a></body></html>`))
		err := generateSite(outStrict, sites, 90*24*time.Hour, "3 months", GenerationInfo{
			Ledger:      ledgerStrict,
			StrictLinks: true,
		})
		if err == nil {
			t.Fatal("expected strict generateSite to return an error on missing target, got nil")
		}
	})
}

func TestLinkLedgerConcurrentOperations(t *testing.T) {
	ledger := NewLinkLedger(t.TempDir())
	ledger.ReservePage("index.html")
	ledger.CommitPage("index.html", 128)
	ledger.ScanHTML("index.html", []byte(`<!DOCTYPE html><html><head></head><body>home</body></html>`))

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				rel := fmt.Sprintf("p/%d/page-%d/index.html", id, i)
				if !ledger.ReservePage(rel) {
					t.Errorf("unexpected duplicate reservation for %s", rel)
				}
				pageHTML := []byte(`<!DOCTYPE html><html><head></head><body><a href="../../../">home</a><a href="https://example.com">ext</a></body></html>`)
				ledger.CommitPage(rel, int64(len(pageHTML)))
				ledger.ScanHTML(rel, pageHTML)
			}
		}(g)
	}
	wg.Wait()

	report := ledger.Verify()
	if !report.Verified {
		t.Fatalf("expected concurrent verified report, got issues: %+v", report.Issues)
	}
	if report.CommittedPages != 16*50+1 {
		t.Errorf("CommittedPages = %d, want %d", report.CommittedPages, 16*50+1)
	}
}

func BenchmarkLinkLedgerScanAndVerify(b *testing.B) {
	sampleHTML := []byte(`<!DOCTYPE html><html lang="en"><head><title>Pkg</title></head><body>
		<nav><a href="../../../">Home</a> <a href="../../">Cat</a> <a href="../">Back</a></nav>
		<p><a href="../../../uses/foo/">foo</a> <a href="https://example.com">ext</a> <a href="#sec">sec</a></p>
		<form action="../../../search/" method="get"><input name="q"></form>
	</body></html>`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ledger := NewLinkLedger("/tmp/bench-site")
		for _, p := range []string{"index.html", "search/index.html", "uses/foo/index.html"} {
			ledger.ReservePage(p)
			ledger.CommitPage(p, 100)
		}
		for j := 0; j < 200; j++ {
			catPath := fmt.Sprintf("repos/r/categories/c%d/index.html", j)
			pkgDirPath := fmt.Sprintf("repos/r/categories/c%d/packages/index.html", j)
			pkgPath := fmt.Sprintf("repos/r/categories/c%d/packages/p/index.html", j)
			ledger.ReservePage(catPath)
			ledger.CommitPage(catPath, 50)
			ledger.ReservePage(pkgDirPath)
			ledger.CommitPage(pkgDirPath, 50)
			ledger.ReservePage(pkgPath)
			ledger.CommitPage(pkgPath, int64(len(sampleHTML)))
			ledger.ScanHTML(pkgPath, sampleHTML)
		}
		rep := ledger.Verify()
		if rep.CommittedPages == 0 {
			b.Fatal("empty pages")
		}
	}
}
