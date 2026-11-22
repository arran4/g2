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

func TestPageNodeCumulativeAndRelativePaths(t *testing.T) {
	root := &PageNode{Name: "Root", Path: ""}
	pkgs := &PageNode{Parent: root, Name: "Packages", Path: "packages"}

	// Relative segment construction
	relCat := &PageNode{Parent: pkgs, Name: "media-gfx", Path: "media-gfx"}
	relPkg := &PageNode{Parent: relCat, Name: "openvdb", Path: "openvdb"}

	if got := relPkg.CanonicalPath(); got != "packages/media-gfx/openvdb" {
		t.Fatalf("relPkg.CanonicalPath() = %q, want packages/media-gfx/openvdb", got)
	}
	if got := relPkg.BaseURL(); got != "../../../" {
		t.Fatalf("relPkg.BaseURL() = %q, want ../../../", got)
	}

	// Cumulative path construction (must not double-count parent prefixes)
	cumCat := &PageNode{Parent: pkgs, Name: "media-gfx", Path: "packages/media-gfx"}
	cumPkg := &PageNode{Parent: cumCat, Name: "openvdb", Path: "packages/media-gfx/openvdb"}

	if got := cumPkg.CanonicalPath(); got != "packages/media-gfx/openvdb" {
		t.Fatalf("cumPkg.CanonicalPath() = %q, want packages/media-gfx/openvdb", got)
	}
	if got := cumPkg.BaseURL(); got != "../../../" {
		t.Fatalf("cumPkg.BaseURL() = %q, want ../../../", got)
	}

	crumbs := cumPkg.Breadcrumbs()
	wantCrumbs := []g2.Breadcrumb{
		{Name: "Root", URL: "../../../"},
		{Name: "Packages", URL: "../../"},
		{Name: "media-gfx", URL: "../"},
		{Name: "openvdb", URL: ""},
	}
	if len(crumbs) != len(wantCrumbs) {
		t.Fatalf("breadcrumbs len = %d, want %d: %+v", len(crumbs), len(wantCrumbs), crumbs)
	}
	for i := range wantCrumbs {
		if crumbs[i] != wantCrumbs[i] {
			t.Errorf("crumb[%d] = %+v, want %+v", i, crumbs[i], wantCrumbs[i])
		}
	}

	// Verify repo moved package path depth (depth 6)
	repo := &PageNode{Parent: root, Name: "overlay-a", Path: "repos/overlay-a"}
	cats := &PageNode{Parent: repo, Name: "Categories", Path: "categories"}
	oldCat := &PageNode{Parent: cats, Name: "app-old", Path: "app-old"}
	movedPkg := &PageNode{Parent: oldCat, Name: "legacy", Path: "packages/legacy"}
	if got := movedPkg.BaseURL(); got != "../../../../../../" {
		t.Fatalf("movedPkg.BaseURL() = %q, want ../../../../../../", got)
	}
}

func extractLinksFromHTML(t *testing.T, rawHTML []byte) []string {
	t.Helper()
	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		t.Fatalf("html.Parse failed: %v", err)
	}
	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "form") {
			attrName := "href"
			if n.Data == "form" {
				attrName = "action"
			}
			for _, attr := range n.Attr {
				if attr.Key == attrName {
					links = append(links, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}

func TestPackagePickerAndMovedPackageLinksStayWithinProjectPrefix(t *testing.T) {
	makeSite := func(repoName string, withMove bool) *g2.SiteData {
		fsys := fstest.MapFS{
			"profiles/repo_name": &fstest.MapFile{Data: []byte(repoName + "\n")},
			"media-gfx/openvdb/openvdb-12.0.ebuild": &fstest.MapFile{Data: []byte(
				"EAPI=8\nDESCRIPTION=\"Volumetric data\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\n",
			)},
			"app-misc/new-name/new-name-1.0.ebuild": &fstest.MapFile{Data: []byte(
				"EAPI=8\nDESCRIPTION=\"Renamed package\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\n",
			)},
		}
		if withMove {
			fsys["profiles/updates/1Q-2026"] = &fstest.MapFile{Data: []byte(
				"move app-misc/old-name app-misc/new-name\n" +
					"move app-retired/legacy-tool app-misc/new-name\n",
			)}
		}
		site, err := parseRepo(fsys, ".", repoName, false, nil)
		if err != nil {
			t.Fatalf("parseRepo(%s) failed: %v", repoName, err)
		}
		return site
	}

	sites := []*g2.SiteData{
		makeSite("overlay-one", true),
		makeSite("overlay-two", false),
	}
	outDir := t.TempDir()
	if err := generateSite(outDir, sites, 90*24*time.Hour, "3 months", GenerationInfo{}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	// Verify package_picker and moved pages on disk
	for _, rel := range []string{
		"packages/media-gfx/index.html",
		"packages/media-gfx/openvdb/index.html",
		"packages/app-misc/old-name/index.html",
		"repos/overlay-one/categories/app-misc/packages/old-name/index.html",
	} {
		if _, err := os.Stat(filepath.Join(outDir, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}

	fsHandler := http.FileServer(http.Dir(outDir))
	for _, tc := range []struct {
		name   string
		prefix string
		mux    http.Handler
	}{
		{name: "root", prefix: "", mux: fsHandler},
		{name: "project-pages", prefix: "/gentoo-packages", mux: http.StripPrefix("/gentoo-packages/", fsHandler)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.mux)
			defer srv.Close()

			pages := []string{
				"/packages/media-gfx/openvdb/",
				"/packages/app-misc/old-name/",
				"/packages/app-retired/legacy-tool/",
				"/repos/overlay-one/categories/app-misc/packages/old-name/",
				"/repos/overlay-one/categories/app-retired/packages/legacy-tool/",
			}
			for _, page := range pages {
				baseURL, err := url.Parse(srv.URL + tc.prefix + page)
				if err != nil {
					t.Fatalf("url.Parse: %v", err)
				}
				resp, err := http.Get(baseURL.String())
				if err != nil {
					t.Fatalf("GET %s failed: %v", baseURL, err)
				}
				var buf bytes.Buffer
				_, _ = buf.ReadFrom(resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("GET %s status = %d", baseURL, resp.StatusCode)
				}

				for _, rawLink := range extractLinksFromHTML(t, buf.Bytes()) {
					if strings.HasPrefix(rawLink, "http://") || strings.HasPrefix(rawLink, "https://") {
						continue
					}
					resolved, err := baseURL.Parse(rawLink)
					if err != nil {
						t.Fatalf("invalid link %q on %s: %v", rawLink, page, err)
					}
					if tc.prefix != "" && !strings.HasPrefix(resolved.Path, tc.prefix+"/") && resolved.Path != tc.prefix {
						t.Errorf("page %s link %q escaped project prefix %q -> %q", page, rawLink, tc.prefix, resolved.Path)
					}
					// Skip /search/ dynamic query test or non-existing move target if single repo
					targetResp, err := http.Get(resolved.String())
					if err != nil {
						t.Fatalf("GET %s (from %s -> %s) failed: %v", resolved, page, rawLink, err)
					}
					_ = targetResp.Body.Close()
					if targetResp.StatusCode != http.StatusOK {
						t.Errorf("broken link on %s: raw=%q resolved=%q status=%d", page, rawLink, resolved.Path, targetResp.StatusCode)
					}
				}
			}
		})
	}
}
