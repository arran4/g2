package main

import (
	"bytes"
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

func TestRedirectPagesValidSingleHTMLDocument(t *testing.T) {
	site, err := parseRepo(fstest.MapFS{
		"profiles/repo_name": &fstest.MapFile{Data: []byte("solo-overlay\n")},
		"app-misc/widget/widget-1.0.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Single-overlay widget\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\n",
		)},
	}, ".", "solo-overlay", false, nil)
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}

	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{site}, 90*24*time.Hour, "3 months", GenerationInfo{}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	redirectPath := filepath.Join(outDir, "packages", "app-misc", "widget", "index.html")
	rawHTML, err := os.ReadFile(redirectPath)
	if err != nil {
		t.Fatalf("reading redirect page: %v", err)
	}
	htmlStr := string(rawHTML)

	if strings.Count(strings.ToLower(htmlStr), "<!doctype html>") != 1 {
		t.Errorf("expected exactly 1 DOCTYPE in redirect document:\n%s", htmlStr)
	}
	if strings.Count(strings.ToLower(htmlStr), "<html") != 1 {
		t.Errorf("expected exactly 1 <html> element in redirect document:\n%s", htmlStr)
	}
	if strings.Count(strings.ToLower(htmlStr), "<head>") != 1 {
		t.Errorf("expected exactly 1 <head> element in redirect document:\n%s", htmlStr)
	}
	if strings.Count(strings.ToLower(htmlStr), "<body>") != 1 {
		t.Errorf("expected exactly 1 <body> element in redirect document:\n%s", htmlStr)
	}

	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		t.Fatalf("html.Parse failed: %v", err)
	}

	var headMetaRefresh string
	var bodyMetaRefresh bool
	var fallbackHref string
	var inHead, inBody bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "head":
				inHead = true
				defer func() { inHead = false }()
			case "body":
				inBody = true
				defer func() { inBody = false }()
			case "meta":
				var httpEquiv, content string
				for _, a := range n.Attr {
					if strings.EqualFold(a.Key, "http-equiv") {
						httpEquiv = a.Val
					}
					if strings.EqualFold(a.Key, "content") {
						content = a.Val
					}
				}
				if strings.EqualFold(httpEquiv, "refresh") {
					if inHead {
						headMetaRefresh = content
					}
					if inBody {
						bodyMetaRefresh = true
					}
				}
			case "a":
				if inBody {
					for _, a := range n.Attr {
						if a.Key == "href" && strings.Contains(a.Val, "repos/solo-overlay/") {
							fallbackHref = a.Val
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

	wantTarget := "../../../repos/solo-overlay/categories/app-misc/packages/widget/"
	if !strings.Contains(headMetaRefresh, "url="+wantTarget) {
		t.Errorf("expected meta refresh inside <head> pointing to %q, got %q", wantTarget, headMetaRefresh)
	}
	if bodyMetaRefresh {
		t.Errorf("unexpected meta refresh inside <body>")
	}
	if fallbackHref != wantTarget {
		t.Errorf("expected fallback href %q, got %q", wantTarget, fallbackHref)
	}

	fsHandler := http.FileServer(http.Dir(outDir))
	for _, tc := range []struct {
		name   string
		prefix string
		mux    http.Handler
	}{
		{name: "root", prefix: "", mux: fsHandler},
		{name: "project-pages", prefix: "/test-project", mux: http.StripPrefix("/test-project/", fsHandler)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.mux)
			defer srv.Close()

			pageURL, _ := url.Parse(srv.URL + tc.prefix + "/packages/app-misc/widget/")
			resolvedTarget, _ := pageURL.Parse(fallbackHref)
			if tc.prefix != "" && !strings.HasPrefix(resolvedTarget.Path, tc.prefix+"/") {
				t.Fatalf("redirect target %q escaped project prefix %q -> %q", fallbackHref, tc.prefix, resolvedTarget.Path)
			}
			resp, err := http.Get(resolvedTarget.String())
			if err != nil {
				t.Fatalf("GET redirect target %s failed: %v", resolvedTarget, err)
			}
			var targetBuf bytes.Buffer
			_, _ = targetBuf.ReadFrom(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("redirect target status = %d", resp.StatusCode)
			}
			// Target must not contain another meta refresh redirect loop
			if strings.Contains(targetBuf.String(), `http-equiv="refresh"`) {
				t.Errorf("target page %s contains a redirect loop", resolvedTarget.Path)
			}
		})
	}
}
