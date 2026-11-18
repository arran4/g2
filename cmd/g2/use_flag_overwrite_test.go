package main

import (
	"bytes"
	"log"
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

func TestUseFlagValidationAndRequiredUseExtraction(t *testing.T) {
	if isValidUseFlagName("") {
		t.Error("empty string should not be a valid USE flag")
	}
	for _, bad := range []string{".", "..", "/", "\\", "+foo", "-bar", "!baz", "foo/bar", "a b", "${VAR}", "abi$((1"} {
		if isValidUseFlagName(bad) {
			t.Errorf("isValidUseFlagName(%q) = true, want false", bad)
		}
	}
	for _, good := range []string{"wayland", "X", "3d", "cpu_flags_x86_avx2", "c++20", "l10n_zh-CN", "linguas_sr@latin"} {
		if !isValidUseFlagName(good) {
			t.Errorf("isValidUseFlagName(%q) = false, want true", good)
		}
	}

	gotIUSE := parseIUSEFlagsFunc("wayland + - +- . .. foo/bar ${VAR} +X -debug")
	if len(gotIUSE) != 3 || gotIUSE[0].Name != "wayland" || gotIUSE[1].Name != "X" || gotIUSE[2].Name != "debug" {
		t.Fatalf("unexpected parseIUSEFlagsFunc result: %+v", gotIUSE)
	}

	gotReq := extractRequiredUseFlags("! ( wayland X ) ?? ( a b ) ${PYTHON_REQUIRED_USE} python? ( !debug )", "test-repo", "media-gfx/openvdb")
	wantReq := []string{"wayland", "X", "a", "b", "debug"}
	if strings.Join(gotReq, ",") != strings.Join(wantReq, ",") {
		t.Fatalf("extractRequiredUseFlags = %v, want %v", gotReq, wantReq)
	}
}

func TestReserveUseFlagDestRejectsCollisionsAndParentIndex(t *testing.T) {
	seen := map[string]string{
		"/tmp/site/uses/index.html": "uses index",
	}
	for _, bad := range []string{"", " ", ".", "..", "a/b", "../root"} {
		if _, err := reserveUseFlagDest(seen, "/tmp/site/uses", bad); err == nil {
			t.Errorf("expected error reserving invalid flag %q, got nil", bad)
		}
	}
	first, err := reserveUseFlagDest(seen, "/tmp/site/uses", "wayland")
	if err != nil {
		t.Fatalf("unexpected error reserving valid flag wayland: %v", err)
	}
	if filepath.ToSlash(first) != "/tmp/site/uses/wayland/index.html" {
		t.Fatalf("unexpected reserved path: %q", first)
	}
	if _, err := reserveUseFlagDest(seen, "/tmp/site/uses", "wayland"); err == nil {
		t.Fatal("expected duplicate destination error for second wayland reservation")
	}
}

func buildMalformedUseTestSite(t *testing.T, repoName string) *g2.SiteData {
	t.Helper()
	fsys := fstest.MapFS{
		"profiles/repo_name": &fstest.MapFile{Data: []byte(repoName + "\n")},
		"profiles/use.desc": &fstest.MapFile{Data: []byte(
			"wayland - Enable Wayland support\n" +
				" - Empty flag entry\n" +
				".. - Parent traversal flag\n",
		)},
		"profiles/use.local.desc": &fstest.MapFile{Data: []byte(
			"media-gfx/openvdb: - Blank local flag\n" +
				"media-gfx/openvdb:wayland - Enable Wayland viewer\n",
		)},
		"media-gfx/openvdb/openvdb-12.0.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Volumetric data library\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\n" +
				"IUSE=\"wayland + - .. . ${UNSET}\"\n" +
				"REQUIRED_USE=\"! ( wayland ) ${PYTHON_REQUIRED_USE}\"\n",
		)},
		"media-gfx/openvdb/metadata.xml": &fstest.MapFile{Data: []byte(
			`<pkgmetadata><use><flag name="">Empty flag</flag><flag name="  ">Space flag</flag><flag name="wayland">Wayland metadata desc</flag></use></pkgmetadata>`,
		)},
		"dev-qt/qt-docs/qt-docs-6.11.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"Qt docs\"\nSLOT=\"6\"\nKEYWORDS=\"amd64\"\nIUSE=\"+wayland ${!iuse[*]}\"\nREQUIRED_USE=\"! ( wayland )\"\n",
		)},
		"media-gfx/freecad/freecad-1.1.ebuild": &fstest.MapFile{Data: []byte(
			"EAPI=8\nDESCRIPTION=\"CAD app\"\nSLOT=\"0\"\nKEYWORDS=\"amd64\"\nIUSE=\"wayland + -\"\nREQUIRED_USE=\"! ( wayland )\"\n",
		)},
	}
	site, err := parseRepo(fsys, ".", repoName, false, nil)
	if err != nil {
		t.Fatalf("parseRepo(%s) failed: %v", repoName, err)
	}
	return site
}

func serveSiteAtPrefixes(t *testing.T, outDir string, handler func(prefix string, srv *httptest.Server)) {
	t.Helper()
	fsHandler := http.FileServer(http.Dir(outDir))

	rootSrv := httptest.NewServer(fsHandler)
	defer rootSrv.Close()
	handler("", rootSrv)

	projSrv := httptest.NewServer(http.StripPrefix("/example-project/", fsHandler))
	defer projSrv.Close()
	handler("/example-project", projSrv)
}

func TestMalformedUseFlagsDoNotOverwriteUsesIndex(t *testing.T) {
	var logBuf bytes.Buffer
	origOut := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(origOut)

	for _, mode := range []struct {
		name  string
		repos []string
	}{
		{name: "single-overlay", repos: []string{"overlay-a"}},
		{name: "aggregated-overlay", repos: []string{"overlay-a", "overlay-b"}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			var sites []*g2.SiteData
			for _, rn := range mode.repos {
				sites = append(sites, buildMalformedUseTestSite(t, rn))
			}

			outDir := t.TempDir()
			if err := generateSite(outDir, sites, 90*24*time.Hour, "3 months", GenerationInfo{}); err != nil {
				t.Fatalf("generateSite failed: %v", err)
			}

			if !strings.Contains(logBuf.String(), "ignoring invalid USE flag") {
				t.Errorf("expected contextual warning log for invalid USE flags, got:\n%s", logBuf.String())
			}

			for _, aggFlag := range sites[0].AggUseFlags.([]*AggUseFlag) {
				if !isValidUseFlagName(aggFlag.Name) {
					t.Errorf("invalid flag %q leaked into repo AggUseFlags", aggFlag.Name)
				}
			}

			// Verify global /uses/index.html is a USE listing, not a detail page
			globalIndexBytes, err := os.ReadFile(filepath.Join(outDir, "uses", "index.html"))
			if err != nil {
				t.Fatalf("reading global uses/index.html: %v", err)
			}
			globalIndex := string(globalIndexBytes)
			if strings.Contains(globalIndex, "<h2>USE Flag:") {
				t.Fatalf("global uses/index.html was overwritten by a USE detail template")
			}
			if !strings.Contains(globalIndex, `href="wayland/"`) {
				t.Errorf("global uses/index.html missing wayland link:\n%s", globalIndex)
			}

			// Verify every repo's /repos/<repo>/uses/index.html is a USE listing
			for _, rn := range mode.repos {
				repoIndexBytes, err := os.ReadFile(filepath.Join(outDir, "repos", rn, "uses", "index.html"))
				if err != nil {
					t.Fatalf("reading repo %s uses/index.html: %v", rn, err)
				}
				repoIndex := string(repoIndexBytes)
				if strings.Contains(repoIndex, "<h2>USE Flag:") {
					t.Fatalf("repo %s uses/index.html was overwritten by a USE detail template", rn)
				}
				if !strings.Contains(repoIndex, `href="wayland/"`) {
					t.Errorf("repo %s uses/index.html missing wayland link", rn)
				}
			}

			// Verify wayland detail page and HTTP navigation at both / and /example-project/
			serveSiteAtPrefixes(t, outDir, func(prefix string, srv *httptest.Server) {
				for _, path := range []string{"/uses/", "/uses/wayland/", "/repos/" + mode.repos[0] + "/uses/"} {
					resp, err := http.Get(srv.URL + prefix + path)
					if err != nil {
						t.Fatalf("GET %s%s failed: %v", prefix, path, err)
					}
					var body bytes.Buffer
					_, _ = body.ReadFrom(resp.Body)
					_ = resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						t.Fatalf("GET %s%s status = %d", prefix, path, resp.StatusCode)
					}
					htmlStr := body.String()
					if path == "/uses/wayland/" {
						if !strings.Contains(htmlStr, "<h2>USE Flag: wayland</h2>") {
							t.Errorf("expected wayland heading on %s%s", prefix, path)
						}
						if !strings.Contains(htmlStr, `action="../../search/"`) {
							t.Errorf("expected prefix-safe search form action ../../search/ on %s%s", prefix, path)
						}
						if !strings.Contains(htmlStr, `href="../">USE Flags</a>`) {
							t.Errorf("expected USE Flags breadcrumb ../ on %s%s", prefix, path)
						}
						if !strings.Contains(htmlStr, "media-gfx/openvdb") || !strings.Contains(htmlStr, "dev-qt/qt-docs") || !strings.Contains(htmlStr, "media-gfx/freecad") {
							t.Errorf("expected openvdb, qt-docs, and freecad on wayland detail page")
						}
					}
				}
			})
		})
	}
}
