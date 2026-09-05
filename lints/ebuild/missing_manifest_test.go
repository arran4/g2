package ebuild

import (
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
	"testing/fstest"
)

func TestMissingManifestLintRule(t *testing.T) {
	mockFS := fstest.MapFS{
		filepath.Join("app-test", "testpkg", "testpkg-1.0.ebuild"): &fstest.MapFile{
			Data: []byte(`SRC_URI="https://example.com/file1.tar.gz"`),
		},
		filepath.Join("app-test", "testpkg", "testpkg-2.0.ebuild"): &fstest.MapFile{
			Data: []byte(`SRC_URI="https://example.com/file2.tar.gz"`),
		},
	}

	category := "app-test"
	name := "testpkg"

	pkgData := &g2.PackageData{
		Category: category,
		Name:     name,
		Manifest: &g2.Manifest{
			Entries: []*g2.ManifestEntry{
				{
					Type:     "DIST",
					Filename: "file1.tar.gz",
				},
			},
		},
	}

	rule := &MissingManifestLintRule{}
	results := rule.lintWithFS(mockFS, filepath.Join(category, name), pkgData, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}

	expectedMsg := "[Error] version 2.0: distfile missing from Manifest: [ file2.tar.gz ]"
	if results[0].Message != expectedMsg {
		t.Errorf("expected message '%s', got '%s'", expectedMsg, results[0].Message)
	}
	if results[0].File != "testpkg-2.0.ebuild" {
		t.Errorf("expected file 'testpkg-2.0.ebuild', got '%s'", results[0].File)
	}
}

func TestMissingManifestLintRule_Beeper_P_vs_PF(t *testing.T) {
	mockFS := fstest.MapFS{
		filepath.Join("app-test", "beeper", "beeper-1.2.3-r1.ebuild"): &fstest.MapFile{
			// ${P} evaluates to beeper-1.2.3, ${PF} evaluates to beeper-1.2.3-r1
			Data: []byte(`
SRC_URI="
	https://example.com/${P}.tar.gz
	https://example.com/${PF}.tar.gz
"`),
		},
	}

	category := "app-test"
	name := "beeper"

	pkgData := &g2.PackageData{
		Category: category,
		Name:     name,
		Manifest: &g2.Manifest{
			Entries: []*g2.ManifestEntry{
				{
					Type:     "DIST",
					Filename: "beeper-1.2.3.tar.gz", // Only P is present
				},
			},
		},
	}

	rule := &MissingManifestLintRule{}
	results := rule.lintWithFS(mockFS, filepath.Join(category, name), pkgData, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}

	expectedMsg := "[Error] version 1.2.3-r1: distfile missing from Manifest: [ beeper-1.2.3-r1.tar.gz ]"
	if results[0].Message != expectedMsg {
		t.Errorf("expected message '%s', got '%s'", expectedMsg, results[0].Message)
	}
}
func TestMissingManifestLintRule_WhichBrowser_Complex(t *testing.T) {
	mockFS := fstest.MapFS{
		filepath.Join("www-client", "which_browser", "which_browser-0.2.6_p44-r1.ebuild"): &fstest.MapFile{
			Data: []byte(`
MY_PV_NO_REV="${PV%%-r*}"
MY_BASE_PV="${MY_PV_NO_REV%_p*}"
MY_BUILD_SUFFIX="${MY_PV_NO_REV##*_p}"
MY_DEB_ARCHIVE="which_browser-${MY_BASE_PV}+${MY_BUILD_SUFFIX}-linux.deb"
SRC_URI="https://which-browser-site.pages.dev/downloads/v${MY_BASE_PV}/${MY_DEB_ARCHIVE}"
`),
		},
	}

	category := "www-client"
	name := "which_browser"

	pkgData := &g2.PackageData{
		Category: category,
		Name:     name,
		Versions: []g2.VersionData{
			{
				Version: "0.2.6_p44-r1",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{
						"SRC_URI": "https://which-browser-site.pages.dev/downloads/v0.2.6/which_browser-0.2.6_p44+0.2.6_p44-linux.deb",
						"PVR":     "0.2.6_p44-r1",
						"PF":      "which_browser-0.2.6_p44-r1",
						"PN":      "which_browser",
						"PV":      "0.2.6_p44",
						"PR":      "r1",
						"P":       "which_browser-0.2.6_p44",
					},
					SrcUri: []g2.URIEntry{
						{Filename: "which_browser-0.2.6_p44+0.2.6_p44-linux.deb"},
					},
				},
			},
		},
		Manifest: &g2.Manifest{
			Entries: []*g2.ManifestEntry{
				{
					Type:     "DIST",
					Filename: "which_browser-0.2.6_p44+0.2.6_p44-linux.deb",
				},
			},
		},
	}

	rule := &MissingManifestLintRule{}
	results := rule.lintWithFS(mockFS, filepath.Join(category, name), pkgData, nil)

	if len(results) > 0 {
		t.Fatalf("expected 0 results, got %d: %v", len(results), results)
	}
}
