package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
)

func TestNewsAutolinksHTTPResolutionRootAndSubpath(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	newsBody := "Title: Runtime and Package Migration\n" +
		"Author: Jane Doe <jane@example.com>\n" +
		"Posted: " + today + "\n" +
		"Revision: 1\n" +
		"News-Item-Format: 2.0\n\n" +
		"See https://example.com/guide (and app-misc/shared, >=app-misc/shared-1.0::repo-a, app-misc/only-b-2.0).\n" +
		"Do not link `app-misc/shared` or unknown/pkg.\n"

	fsA := fstest.MapFS{
		"profiles/repo_name":               {Data: []byte("repo-a\n")},
		"app-misc/shared/shared-1.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
		"metadata/news/" + today + "-mig/" + today + "-mig.en.txt": {Data: []byte(newsBody)},
	}
	fsB := fstest.MapFS{
		"profiles/repo_name":               {Data: []byte("repo-b\n")},
		"app-misc/shared/shared-1.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
		"app-misc/only-b/only-b-2.0.ebuild": {Data: []byte("EAPI=8\nSLOT=\"0\"\n")},
	}

	sA, err := parseRepo(fsA, ".", "repo-a", false, nil, SourceURL("https://example.com/a.git"))
	if err != nil {
		t.Fatal(err)
	}
	sB, err := parseRepo(fsB, ".", "repo-b", false, nil, SourceURL("https://example.com/b.git"))
	if err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{sA, sB}, 30*24*time.Hour, "30d", GenerationInfo{RecentDuration: "30d"}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	hrefRe := regexp.MustCompile(`<div class="news-body"[^>]*>([\s\S]*?)</div>`)
	linkRe := regexp.MustCompile(`href="([^"]+)"`)

	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		for _, pagePath := range []string{
			"news/",
			"news/archive/repo-a/" + today + "-mig/",
			"repos/repo-a/news/",
			"repos/repo-a/news/archive/" + today + "-mig/",
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
			m := hrefRe.FindStringSubmatch(s)
			if m == nil {
				srv.Close()
				t.Fatalf("missing .news-body on %s", pageURLStr)
			}
			newsHTML := m[1]
			if !strings.Contains(newsHTML, `https://example.com/guide`) || strings.Contains(newsHTML, `unknown/pkg</a>`) || !strings.Contains(newsHTML, `<code>app-misc/shared</code>`) {
				srv.Close()
				t.Fatalf("unexpected .news-body autolink output on %s:\n%s", pageURLStr, newsHTML)
			}

			pageURL, _ := url.Parse(pageURLStr)
			internalCount := 0
			for _, lm := range linkRe.FindAllStringSubmatch(newsHTML, -1) {
				raw := lm[1]
				if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
					continue
				}
				rel, err := url.Parse(raw)
				if err != nil {
					srv.Close()
					t.Fatalf("invalid internal link %q: %v", raw, err)
				}
				resolved := pageURL.ResolveReference(rel)
				if prefix != "/" && !strings.HasPrefix(resolved.Path, prefix) {
					srv.Close()
					t.Fatalf("autolink escaped mount %q: %s -> %s", prefix, raw, resolved.String())
				}
				targetResp, err := http.Get(resolved.String())
				if err != nil {
					srv.Close()
					t.Fatalf("GET autolink target %s: %v", resolved.String(), err)
				}
				_ = targetResp.Body.Close()
				if targetResp.StatusCode != http.StatusOK {
					srv.Close()
					t.Fatalf("autolink target %s returned status %d (raw=%q from %s)", resolved.String(), targetResp.StatusCode, raw, pagePath)
				}
				internalCount++
			}
			if internalCount < 3 {
				srv.Close()
				t.Fatalf("expected >= 3 internal autolinks on %s, got %d\nbody:\n%s", pageURLStr, internalCount, newsHTML)
			}
		}

		srv.Close()
	}
}
