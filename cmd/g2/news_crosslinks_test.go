package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
)

func buildNewsOverlay(t *testing.T, name string, withNews bool, newsTitle string) *g2.SiteData {
	t.Helper()
	files := fstest.MapFS{
		"profiles/repo_name":          {Data: []byte(name + "\n")},
		"app-misc/pkg/pkg-1.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
	}
	if withNews {
		today := time.Now().UTC().Format("2006-01-02")
		files["metadata/news/2026-01-15-shared/2026-01-15-shared.en.txt"] = &fstest.MapFile{
			Data: []byte("Title: " + newsTitle + "\nAuthor: Jane Doe <jane@example.com>\nPosted: " + today + "\nRevision: 1\nNews-Item-Format: 2.0\n\nBody for " + name + "\n"),
		}
	}
	site, err := parseRepo(files, ".", name, false, nil, SourceURL("https://example.com/"+name+".git"))
	if err != nil {
		t.Fatalf("parseRepo(%s): %v", name, err)
	}
	return site
}

func TestNewsCrossLinksSingleAndOnlyOneWithNews(t *testing.T) {
	// Case 1: Single overlay with news -> no "All news" link on repo news pages
	s1 := buildNewsOverlay(t, "solo-overlay", true, "Solo News")
	out1 := t.TempDir()
	if err := generateSite(out1, []*g2.SiteData{s1}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite(solo): %v", err)
	}
	for _, rel := range []string{
		"repos/solo-overlay/news/index.html",
		"repos/solo-overlay/news/archive/index.html",
		"repos/solo-overlay/news/archive/2026-01-15-shared/index.html",
	} {
		b, err := os.ReadFile(filepath.Join(out1, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if strings.Contains(string(b), "All news") {
			t.Fatalf("expected no 'All news' link in single-news-repo mode (%s)", rel)
		}
	}

	// Case 2: Two overlays, only one has news -> still no "All news" link, and newsless repo has no news dir
	sWith := buildNewsOverlay(t, "with-news", true, "With News")
	sWithout := buildNewsOverlay(t, "no-news", false, "")
	out2 := t.TempDir()
	if err := generateSite(out2, []*g2.SiteData{sWith, sWithout}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite(one-with-news): %v", err)
	}
	if _, err := os.Stat(filepath.Join(out2, "repos", "no-news", "news")); !os.IsNotExist(err) {
		t.Fatalf("expected no news directory for newsless repo")
	}
	b, err := os.ReadFile(filepath.Join(out2, "repos", "with-news", "news", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "All news") {
		t.Fatalf("expected no 'All news' link when only 1 of 2 repos has news")
	}
}

func TestNewsCrossLinksMultiOverlayHTTP(t *testing.T) {
	s1 := buildNewsOverlay(t, "alpha-overlay", true, "Alpha Shared")
	s2 := buildNewsOverlay(t, "beta-overlay", true, "Beta Shared")

	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{s1, s2}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite(multi): %v", err)
	}

	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		mustGet := func(path string, wants ...string) {
			t.Helper()
			resp, err := http.Get(srv.URL + prefix + path)
			if err != nil {
				srv.Close()
				t.Fatalf("GET %s%s: %v", prefix, path, err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				srv.Close()
				t.Fatalf("GET %s%s status=%d", prefix, path, resp.StatusCode)
			}
			s := string(body)
			for _, w := range wants {
				if !strings.Contains(s, w) {
					srv.Close()
					t.Fatalf("%s%s missing %q", prefix, path, w)
				}
			}
		}

		mustGet("news/", "../repos/alpha-overlay/news/archive/", "../repos/beta-overlay/news/archive/", "Alpha Shared", "Beta Shared")
		mustGet("news/archive/", "../../repos/alpha-overlay/news/archive/", "../../repos/beta-overlay/news/archive/")
		mustGet("repos/alpha-overlay/news/", "../../../news/", "Alpha Shared")
		mustGet("repos/alpha-overlay/news/archive/", "../../../../news/", "../../../../news/archive/")
		mustGet("repos/alpha-overlay/news/archive/2026-01-15-shared/", "../../../../../news/", "Body for alpha-overlay")
		mustGet("repos/beta-overlay/news/archive/2026-01-15-shared/", "../../../../../news/", "Body for beta-overlay")

		srv.Close()
	}
}
