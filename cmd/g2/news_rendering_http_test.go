package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
	newsLint "github.com/arran4/g2/lints/news"
)

func TestNewsASTHeadingsFootnotesHTTPAndLintIndependence(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	newsContent := func(title string) string {
		return "Title: " + title + "\n" +
			"Author: Jane Doe <jane@example.com>\n" +
			"Posted: " + today + "\n" +
			"Revision: 1\n" +
			"News-Item-Format: 2.0\n\n" +
			"# Migration Heading\n\n" +
			"Read footnote[1] and ignore <script>xss</script>.\n\n" +
			"[1] https://example.com/ref\n"
	}

	fs1 := fstest.MapFS{
		"profiles/repo_name":                              {Data: []byte("repo-a\n")},
		"app-misc/pkg/pkg-1.0.ebuild":                     {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
		"metadata/news/2026-03-31-first/2026-03-31-first.en.txt": {Data: []byte(newsContent("First Article"))},
	}
	fs2 := fstest.MapFS{
		"profiles/repo_name":                               {Data: []byte("repo-b\n")},
		"app-misc/pkg/pkg-1.0.ebuild":                      {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
		"metadata/news/2026-03-31-second/2026-03-31-second.en.txt": {Data: []byte(newsContent("Second Article"))},
	}

	// Verify lints/news remains strict and unaffected (both valid GLEP 42 files).
	if res := newsLint.NewNewsValidityLintRule(newsLint.WithFS(fs1)).Lint(".", nil); len(res) != 0 {
		t.Fatalf("unexpected lint errors on fs1: %+v", res)
	}

	s1, err := parseRepo(fs1, ".", "repo-a", false, nil, SourceURL("https://example.com/a.git"))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := parseRepo(fs2, ".", "repo-b", false, nil, SourceURL("https://example.com/b.git"))
	if err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{s1, s2}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	idRe := regexp.MustCompile(`id="([^"]+)"`)
	hrefFragRe := regexp.MustCompile(`href="#([^"]+)"`)

	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		for _, p := range []string{
			"news/",
			"news/archive/repo-a/2026-03-31-first/",
			"repos/repo-a/news/",
			"repos/repo-a/news/archive/2026-03-31-first/",
		} {
			resp, err := http.Get(srv.URL + prefix + p)
			if err != nil {
				srv.Close()
				t.Fatalf("GET %s%s: %v", prefix, p, err)
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				srv.Close()
				t.Fatalf("GET %s%s status=%d", prefix, p, resp.StatusCode)
			}
			htmlStr := string(bodyBytes)
			if !strings.Contains(htmlStr, "<h3 id=") || !strings.Contains(htmlStr, `class="news-footnotes"`) {
				srv.Close()
				t.Fatalf("%s%s missing transformed heading or footnotes", prefix, p)
			}
			if strings.Contains(htmlStr, "<script>xss</script>") {
				srv.Close()
				t.Fatalf("%s%s leaked raw script tag", prefix, p)
			}

			// Verify unique IDs and that every #fragment target exists on the page.
			ids := make(map[string]bool)
			for _, m := range idRe.FindAllStringSubmatch(htmlStr, -1) {
				if ids[m[1]] {
					srv.Close()
					t.Fatalf("%s%s has duplicate id %q", prefix, p, m[1])
				}
				ids[m[1]] = true
			}
			for _, m := range hrefFragRe.FindAllStringSubmatch(htmlStr, -1) {
				if !ids[m[1]] {
					srv.Close()
					t.Fatalf("%s%s has broken fragment href #%s", prefix, p, m[1])
				}
			}
		}

		// Verify RSS/Atom feeds remain valid XML.
		for _, feed := range []string{"news/index.rss", "news/index.atom", "repos/repo-a/news/index.rss"} {
			resp, err := http.Get(srv.URL + prefix + feed)
			if err != nil {
				srv.Close()
				t.Fatalf("GET %s%s: %v", prefix, feed, err)
			}
			feedBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				srv.Close()
				t.Fatalf("GET %s%s status=%d", prefix, feed, resp.StatusCode)
			}
			dec := xml.NewDecoder(bytes.NewReader(feedBytes))
			for {
				if _, err := dec.Token(); err != nil {
					if err == io.EOF {
						break
					}
					srv.Close()
					t.Fatalf("invalid XML feed %s%s: %v", prefix, feed, err)
				}
			}
		}

		srv.Close()
	}
}
