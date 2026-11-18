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
)

func TestRepoUsePagesSingleRenderAndValidLinks(t *testing.T) {
	siteA, err := parseRepo(fstest.MapFS{
		"profiles/repo_name": &fstest.MapFile{Data: []byte("repo-alpha\n")},
		"media-gfx/shared-app/shared-app-1.0.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Shared app with wayland\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"wayland\"\n",
		)},
		"app-misc/alpha-only/alpha-only-1.0.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Alpha app\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"wayland custom\"\n",
		)},
	}, ".", "repo-alpha", false, nil)
	if err != nil {
		t.Fatalf("parseRepo alpha failed: %v", err)
	}

	// repo-beta also has media-gfx/shared-app, WITHOUT wayland flag
	siteB, err := parseRepo(fstest.MapFS{
		"profiles/repo_name": &fstest.MapFile{Data: []byte("repo-beta\n")},
		"media-gfx/shared-app/shared-app-2.0.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Shared app without wayland\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"qt6\"\n",
		)},
	}, ".", "repo-beta", false, nil)
	if err != nil {
		t.Fatalf("parseRepo beta failed: %v", err)
	}

	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{siteA, siteB}, 90*24*time.Hour, "3 months", GenerationInfo{}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	// Verify repo USE pages were rendered with the consolidated repo_use.html / repo_uses.html contract
	repoUseBytes, err := os.ReadFile(filepath.Join(outDir, "repos", "repo-alpha", "uses", "wayland", "index.html"))
	if err != nil {
		t.Fatalf("reading repo-alpha wayland page: %v", err)
	}
	repoUseHTML := string(repoUseBytes)
	if strings.Contains(repoUseHTML, `href="../../repos/repo-alpha/`) {
		t.Fatalf("repo_use.html still contains duplicated repo path link:\n%s", repoUseHTML)
	}
	if !strings.Contains(repoUseHTML, `value="overlay:repo-alpha use:wayland `) {
		t.Errorf("expected repo_use.html search form with overlay:repo-alpha filter")
	}
	if !strings.Contains(repoUseHTML, `href="../../../../repos/repo-alpha/categories/media-gfx/packages/shared-app/"`) {
		t.Errorf("expected repo_use.html to link to repo-alpha shared-app package:\n%s", repoUseHTML)
	}

	// Verify global /uses/wayland/ deterministically links media-gfx/shared-app to repo-alpha (not repo-beta)
	globalUseBytes, err := os.ReadFile(filepath.Join(outDir, "uses", "wayland", "index.html"))
	if err != nil {
		t.Fatalf("reading global wayland page: %v", err)
	}
	globalUseHTML := string(globalUseBytes)
	if !strings.Contains(globalUseHTML, `href="../../repos/repo-alpha/categories/media-gfx/packages/shared-app/"`) {
		t.Errorf("expected global wayland page to link shared-app deterministically to repo-alpha:\n%s", globalUseHTML)
	}
	if strings.Contains(globalUseHTML, `repos/repo-beta/categories/media-gfx/packages/shared-app/`) {
		t.Errorf("global wayland page linked shared-app to repo-beta which does not use wayland")
	}

	// Verify over HTTP at / and /test-project/
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

			for _, path := range []string{
				"/uses/wayland/",
				"/repos/repo-alpha/uses/",
				"/repos/repo-alpha/uses/wayland/",
			} {
				baseURL, _ := url.Parse(srv.URL + tc.prefix + path)
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

				for _, link := range []string{
					"../../../../repos/repo-alpha/categories/media-gfx/packages/shared-app/",
					"../../../../repos/repo-alpha/categories/app-misc/packages/alpha-only/",
				} {
					if path == "/repos/repo-alpha/uses/wayland/" {
						resolved, _ := baseURL.Parse(link)
						if tc.prefix != "" && !strings.HasPrefix(resolved.Path, tc.prefix+"/") {
							t.Errorf("link %q escaped project prefix %q -> %q", link, tc.prefix, resolved.Path)
						}
						targetResp, err := http.Get(resolved.String())
						if err != nil {
							t.Fatalf("GET %s failed: %v", resolved, err)
						}
						_ = targetResp.Body.Close()
						if targetResp.StatusCode != http.StatusOK {
							t.Errorf("broken link %q -> %q (status %d)", link, resolved.Path, targetResp.StatusCode)
						}
					}
				}
			}
		})
	}
}
