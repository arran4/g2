package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
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
	"golang.org/x/net/html"
)

type crawlIssue struct {
	SourcePage string
	Element    string
	RawURL     string
	Resolved   string
	StatusCode int
	Reason     string
}

type crawlReport struct {
	PagesVisited  int
	AssetsChecked int
	FeedsChecked  int
	BrokenLinks   []crawlIssue
	RootEscapes   []crawlIssue
	MalformedDocs []crawlIssue
}

func (r *crawlReport) totalIssues() int {
	return len(r.BrokenLinks) + len(r.RootEscapes) + len(r.MalformedDocs)
}

// newMountTestServer serves outDir at mountPrefix (e.g. "/" or "/test-project/").
// Requests outside mountPrefix return 404 so root-escaping relative links fail loudly.
func newMountTestServer(outDir, mountPrefix string) *httptest.Server {
	if mountPrefix == "" || mountPrefix == "/" {
		return httptest.NewServer(http.FileServer(http.Dir(outDir)))
	}
	prefix := "/" + strings.Trim(mountPrefix, "/") + "/"
	fsHandler := http.StripPrefix(prefix, http.FileServer(http.Dir(outDir)))
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == strings.TrimSuffix(prefix, "/") {
			http.Redirect(w, r, prefix, http.StatusMovedPermanently)
			return
		}
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		fsHandler.ServeHTTP(w, r)
	}))
}

func crawlGeneratedSite(t *testing.T, baseURL, mountPrefix string) *crawlReport {
	t.Helper()
	report := &crawlReport{}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse baseURL: %v", err)
	}
	prefix := "/"
	if mountPrefix != "" && mountPrefix != "/" {
		prefix = "/" + strings.Trim(mountPrefix, "/") + "/"
	}

	startURL := strings.TrimSuffix(baseURL, "/") + prefix
	type queueItem struct {
		urlStr string
		source string
		elem   string
		raw    string
	}

	queue := []queueItem{{urlStr: startURL, source: "<root>", elem: "root", raw: startURL}}
	visited := make(map[string]bool)
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		u, err := url.Parse(cur.urlStr)
		if err != nil {
			report.BrokenLinks = append(report.BrokenLinks, crawlIssue{
				SourcePage: cur.source,
				Element:    cur.elem,
				RawURL:     cur.raw,
				Resolved:   cur.urlStr,
				Reason:     fmt.Sprintf("invalid URL: %v", err),
			})
			continue
		}
		u.Fragment = ""
		canon := u.String()
		if visited[canon] {
			continue
		}
		visited[canon] = true

		if u.Host != parsedBase.Host {
			continue
		}
		if prefix != "/" && !strings.HasPrefix(u.Path, prefix) {
			report.RootEscapes = append(report.RootEscapes, crawlIssue{
				SourcePage: cur.source,
				Element:    cur.elem,
				RawURL:     cur.raw,
				Resolved:   canon,
				Reason:     fmt.Sprintf("path %q escapes mount prefix %q", u.Path, prefix),
			})
			continue
		}

		resp, err := client.Get(canon)
		if err != nil {
			report.BrokenLinks = append(report.BrokenLinks, crawlIssue{
				SourcePage: cur.source,
				Element:    cur.elem,
				RawURL:     cur.raw,
				Resolved:   canon,
				Reason:     fmt.Sprintf("HTTP GET failed: %v", err),
			})
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			report.BrokenLinks = append(report.BrokenLinks, crawlIssue{
				SourcePage: cur.source,
				Element:    cur.elem,
				RawURL:     cur.raw,
				Resolved:   canon,
				StatusCode: resp.StatusCode,
				Reason:     fmt.Sprintf("read body failed: %v", readErr),
			})
			continue
		}
		if resp.StatusCode != http.StatusOK {
			report.BrokenLinks = append(report.BrokenLinks, crawlIssue{
				SourcePage: cur.source,
				Element:    cur.elem,
				RawURL:     cur.raw,
				Resolved:   canon,
				StatusCode: resp.StatusCode,
				Reason:     fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode),
			})
			continue
		}

		ct := strings.ToLower(resp.Header.Get("Content-Type"))
		lowerPath := strings.ToLower(u.Path)
		switch {
		case strings.Contains(ct, "text/html") || strings.HasSuffix(lowerPath, ".html") || strings.HasSuffix(lowerPath, "/"):
			report.PagesVisited++
			validateHTMLStructure(report, canon, body)
			refs := extractHTMLRefs(u, body)
			for _, ref := range refs {
				queue = append(queue, queueItem{
					urlStr: ref.resolved,
					source: canon,
					elem:   ref.elem,
					raw:    ref.raw,
				})
			}
		case strings.HasSuffix(lowerPath, ".rss") || strings.HasSuffix(lowerPath, ".atom") || strings.Contains(ct, "xml"):
			report.FeedsChecked++
			validateXMLFeed(report, canon, body)
		case strings.HasSuffix(lowerPath, "search_index.json"):
			report.AssetsChecked++
			validateSearchIndexJSON(t, report, u, body, func(resolved, raw string) {
				queue = append(queue, queueItem{
					urlStr: resolved,
					source: canon,
					elem:   "search_index.json",
					raw:    raw,
				})
			})
		default:
			report.AssetsChecked++
		}
	}

	return report
}

func validateHTMLStructure(report *crawlReport, pageURL string, body []byte) {
	lower := strings.ToLower(string(body))
	checks := []struct {
		token string
		exact int
	}{
		{"<!doctype", 1},
		{"<html", 1},
		{"</html", 1},
		{"<head", 1},
		{"</head", 1},
		{"<body", 1},
		{"</body", 1},
	}
	for _, c := range checks {
		cnt := strings.Count(lower, c.token)
		if cnt != c.exact {
			report.MalformedDocs = append(report.MalformedDocs, crawlIssue{
				SourcePage: pageURL,
				Element:    "html",
				Resolved:   pageURL,
				Reason:     fmt.Sprintf("expected %d occurrence(s) of %q, found %d", c.exact, c.token, cnt),
			})
			return
		}
	}
}

type extractedRef struct {
	elem     string
	raw      string
	resolved string
}

func extractHTMLRefs(pageURL *url.URL, body []byte) []extractedRef {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var refs []extractedRef
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a", "link", "area":
				if v, ok := getAttr(n, "href"); ok {
					if r, ok2 := resolveInternalRef(pageURL, v); ok2 {
						refs = append(refs, extractedRef{elem: n.Data, raw: v, resolved: r})
					}
				}
			case "script", "img", "iframe", "source":
				if v, ok := getAttr(n, "src"); ok {
					if r, ok2 := resolveInternalRef(pageURL, v); ok2 {
						refs = append(refs, extractedRef{elem: n.Data, raw: v, resolved: r})
					}
				}
			case "form":
				if v, ok := getAttr(n, "action"); ok {
					if r, ok2 := resolveInternalRef(pageURL, v); ok2 {
						refs = append(refs, extractedRef{elem: n.Data, raw: v, resolved: r})
					}
				}
			case "meta":
				if eq, ok := getAttr(n, "http-equiv"); ok && strings.EqualFold(strings.TrimSpace(eq), "refresh") {
					if content, ok2 := getAttr(n, "content"); ok2 {
						if target := parseMetaRefreshTarget(content); target != "" {
							if r, ok3 := resolveInternalRef(pageURL, target); ok3 {
								refs = append(refs, extractedRef{elem: "meta-refresh", raw: target, resolved: r})
							}
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return refs
}

func getAttr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}

func parseMetaRefreshTarget(content string) string {
	parts := strings.Split(content, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 4 && strings.EqualFold(p[:4], "url=") {
			return strings.Trim(strings.TrimSpace(p[4:]), `"'`)
		}
	}
	return ""
}

func resolveInternalRef(pageURL *url.URL, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return "", false
	}
	lower := strings.ToLower(raw)
	for _, scheme := range []string{"http://", "https://", "mailto:", "javascript:", "data:", "tel:"} {
		if strings.HasPrefix(lower, scheme) {
			return "", false
		}
	}
	rel, err := url.Parse(raw)
	if err != nil {
		return raw, true
	}
	resolved := pageURL.ResolveReference(rel)
	resolved.Fragment = ""
	return resolved.String(), true
}

func validateXMLFeed(report *crawlReport, feedURL string, body []byte) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			report.MalformedDocs = append(report.MalformedDocs, crawlIssue{
				SourcePage: feedURL,
				Element:    "xml",
				Resolved:   feedURL,
				Reason:     fmt.Sprintf("invalid XML feed: %v", err),
			})
			return
		}
	}
}

func validateSearchIndexJSON(t *testing.T, report *crawlReport, indexURL *url.URL, body []byte, enqueue func(resolved, raw string)) {
	t.Helper()
	var entries []map[string]any
	if err := json.Unmarshal(body, &entries); err != nil {
		report.MalformedDocs = append(report.MalformedDocs, crawlIssue{
			SourcePage: indexURL.String(),
			Element:    "search_index.json",
			Resolved:   indexURL.String(),
			Reason:     fmt.Sprintf("invalid search_index.json: %v", err),
		})
		return
	}
	for _, entry := range entries {
		if u, ok := entry["url"].(string); ok && u != "" {
			if r, ok2 := resolveInternalRef(indexURL, u); ok2 {
				enqueue(r, u)
			}
		}
	}
}

func TestSiteCrawlerFaultInjection(t *testing.T) {
	outDir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		full := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("index.html", `<!DOCTYPE html><html><head><title>Root</title><script src="missing.js"></script></head><body>
		<a href="missing-page/">Missing</a>
		<a href="nested-doc/">Nested</a>
		<a href="deep/sub/escape.html">Escape</a>
		<a href="redirect-bad/">Bad Redirect</a>
		<a href="search_index.json">Search Index</a>
	</body></html>`)
	mustWrite("nested-doc/index.html", `<!DOCTYPE html><html><head></head><body><!DOCTYPE html><html><head></head><body>nested</body></html></body></html>`)
	mustWrite("deep/sub/escape.html", `<!DOCTYPE html><html><head></head><body><a href="../../../search/">Escaped</a></body></html>`)
	mustWrite("redirect-bad/index.html", `<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0; url=../does-not-exist/"></head><body></body></html>`)
	mustWrite("search_index.json", `[{"title":"broken","url":"broken-search-target/"}]`)

	srv := newMountTestServer(outDir, "/test-project/")
	defer srv.Close()

	report := crawlGeneratedSite(t, srv.URL, "/test-project/")
	if len(report.BrokenLinks) < 4 {
		t.Fatalf("expected at least 4 broken links (missing.js, missing-page/, redirect target, search_index target), got %d: %+v", len(report.BrokenLinks), report.BrokenLinks)
	}
	if len(report.RootEscapes) < 1 {
		t.Fatalf("expected at least 1 root escape for ../../../search/, got %d", len(report.RootEscapes))
	}
	if len(report.MalformedDocs) < 1 {
		t.Fatalf("expected at least 1 malformed HTML document for nested-doc, got %d", len(report.MalformedDocs))
	}
}

func buildCrawlerFixture(t *testing.T) []*g2.SiteData {
	t.Helper()
	memFS := fstest.MapFS{
		"profiles/repo_name":        {Data: []byte("test-overlay\n")},
		"profiles/categories":       {Data: []byte("app-misc\n")},
		"profiles/package.mask":     {Data: []byte("=app-misc/demo-2.0\n")},
		"profiles/package.deprecated": {Data: []byte("app-misc/demo\n")},
		"profiles/use.desc":         {Data: []byte("foo - Enable foo feature\n")},
		"profiles/use.local.desc":   {Data: []byte("app-misc/demo:bar - Enable local bar\n")},
		"profiles/desc/cpu_flags.desc": {Data: []byte("avx2 - Enable AVX2\n")},
		"profiles/updates/1Q-2026":  {Data: []byte("move app-misc/old-demo app-misc/demo\n")},
		"metadata/layout.conf":      {Data: []byte("masters = gentoo\n")},
		"metadata/news/2026-01-01-welcome/2026-01-01-welcome.en.txt": {
			Data: []byte("Title: Welcome\nAuthor: Jane Doe <jane@example.com>\nPosted: 2026-01-01\nRevision: 1\nNews-Item-Format: 2.0\n\nWelcome to test-overlay.\n"),
		},
		"app-misc/demo/Manifest": {
			Data: []byte("DIST demo-1.0.tar.gz 100 SHA256 0000000000000000000000000000000000000000000000000000000000000000 SHA512 00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\n"),
		},
		"app-misc/demo/metadata.xml": {
			Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<pkgmetadata>
  <maintainer type="person">
    <email>jane@example.com</email>
    <name>Jane Doe</name>
  </maintainer>
  <use>
    <flag name="foo">Enable foo</flag>
    <flag name="bar">Enable bar</flag>
  </use>
</pkgmetadata>`),
		},
		"app-misc/demo/demo-1.0.ebuild": {
			Data: []byte("EAPI=8\nDESCRIPTION=\"Demo package\"\nHOMEPAGE=\"https://example.com\"\nSLOT=\"0\"\nKEYWORDS=\"~amd64\"\nIUSE=\"foo bar cpu_flags_avx2\"\n"),
		},
		"app-misc/demo/demo-2.0.ebuild": {
			Data: []byte("EAPI=8\nDESCRIPTION=\"Demo package v2\"\nHOMEPAGE=\"https://example.com\"\nSLOT=\"0\"\nKEYWORDS=\"~amd64\"\nIUSE=\"foo\"\n"),
		},
	}

	site, err := parseRepo(memFS, ".", "test-overlay", false, nil, SourceURL("https://example.com/test-overlay.git"))
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}
	return []*g2.SiteData{site}
}

func TestGeneratedSiteHTTPNavigationRootAndSubpath(t *testing.T) {
	sites := buildCrawlerFixture(t)
	outDir := t.TempDir()
	genInfo := GenerationInfo{
		RecentDuration: "30d",
	}
	if err := generateSite(outDir, sites, 30*24*time.Hour, "30d", genInfo); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	for _, mount := range []string{"/", "/test-project/"} {
		t.Run("mount="+mount, func(t *testing.T) {
			srv := newMountTestServer(outDir, mount)
			defer srv.Close()

			report := crawlGeneratedSite(t, srv.URL, mount)
			if report.PagesVisited == 0 {
				t.Fatalf("expected crawler to visit generated pages at mount %q", mount)
			}
			if report.totalIssues() > 0 {
				t.Errorf("navigation audit at mount %q found %d issues (pages=%d assets=%d feeds=%d):\nbroken=%+v\nescapes=%+v\nmalformed=%+v",
					mount, report.totalIssues(), report.PagesVisited, report.AssetsChecked, report.FeedsChecked,
					report.BrokenLinks, report.RootEscapes, report.MalformedDocs)
			}
		})
	}
}
