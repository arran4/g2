package ebuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestOrphanedManifestLintRule(t *testing.T) {
	rule := &ebuild.OrphanedManifestLintRule{}

	// Create a temporary directory structure for the test repo
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "app-misc", "testpkg")
	if err := os.MkdirAll(filepath.Join(pkgDir, "files"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create an ebuild that uses a DIST file with PV variable
	ebuildPath := filepath.Join(pkgDir, "testpkg-1.0-r1.ebuild")
	ebuildContent := `SRC_URI="https://example.com/file1-v${PV}.tar.gz"`
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an ESP-IDF ebuild that uses PV which should evaluate to exactly PV not PV-PR
	ebuildPathEsp := filepath.Join(pkgDir, "testpkg-2.0-r2.ebuild")
	ebuildContentEsp := `SRC_URI="https://example.com/esp-v${PV}.tar.gz"`
	if err := os.WriteFile(ebuildPathEsp, []byte(ebuildContentEsp), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an ESP-IDF ebuild that uses PV which should evaluate to exactly PV not PV-PR
	if err := os.WriteFile(filepath.Join(pkgDir, "files", "used.patch"), []byte("patch content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up the package data
	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "testpkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0-r1",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{
						"SRC_URI": "https://example.com/file1-v${PV}.tar.gz",
					},
					SrcUri: []g2.URIEntry{
						{Filename: "file1-v1.0.tar.gz"},
					},
				},
			},
			{
				Version: "2.0-r2",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{
						"SRC_URI": "https://example.com/esp-v${PV}.tar.gz",
					},
					SrcUri: []g2.URIEntry{
						{Filename: "esp-v2.0.tar.gz"}, // Evaluates exactly to PV
					},
				},
			},
		},
		Manifest: &g2.Manifest{
			Entries: []*g2.ManifestEntry{
				// Used DIST file (should not error)
				{Type: "DIST", Filename: "file1-v1.0.tar.gz"},
				// Used ESP-IDF DIST file testing PV
				{Type: "DIST", Filename: "esp-v2.0.tar.gz"},
				// Unused DIST file (should error)
				{Type: "DIST", Filename: "file2.tar.gz"},
				// Used AUX file (should not error)
				{Type: "AUX", Filename: "used.patch"},
				// Non-existent AUX file (should error)
			},
		},
	}

	warnings := rule.Lint(repoDir, pkg)

	expectedWarnings := []string{
		"Manifest entry for unused DIST file 'file2.tar.gz'",
	}

	if len(warnings) != len(expectedWarnings) {
		t.Errorf("Expected %d warnings, got %d:\n%v", len(expectedWarnings), len(warnings), warnings)
	}

	for _, expected := range expectedWarnings {
		found := false
		for _, warn := range warnings {
			if strings.Contains(warn.Message, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning containing: %s\nGot warnings:\n%v", expected, warnings)
		}
	}
}
func TestOrphanedManifestLintRule_WhichBrowser_Complex(t *testing.T) {
	pkgDir := t.TempDir()

	ebuildContent := `
MY_PV_NO_REV="${PV%%-r*}"
MY_BASE_PV="${MY_PV_NO_REV%.*}"
MY_BUILD_SUFFIX="${MY_PV_NO_REV##*.}"
MY_DEB_ARCHIVE="which_browser-${MY_BASE_PV}+${MY_BUILD_SUFFIX}-linux.deb"
SRC_URI="https://which-browser-site.pages.dev/downloads/v${MY_BASE_PV}/${MY_DEB_ARCHIVE}"
`
	if err := os.MkdirAll(filepath.Join(pkgDir, "www-client", "which_browser"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "www-client", "which_browser", "which_browser-0.2.6.44-r1.ebuild"), []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	pkgData := &g2.PackageData{
		Category: "www-client",
		Name:     "which_browser",
		Manifest: &g2.Manifest{
			Entries: []*g2.ManifestEntry{
				{Type: "DIST", Filename: "which_browser-0.2.6+44-linux.deb"},
			},
		},
	}

	rule := &ebuild.OrphanedManifestLintRule{}
	results := rule.Lint(pkgDir, pkgData)

	if len(results) > 0 {
		t.Fatalf("expected 0 results, got %d: %v", len(results), results)
	}
}

// Verify that a syntactically valid ebuild with eclass-generated SRC_URI values
// cannot produce reliable orphaned-DIST claims from static URI enumeration.
func TestOrphanedManifestLintRule_UncertainSrcURI(t *testing.T) {
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "net-im", "rustapp")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	const source = `EAPI=8
CRATES="
	example@1.2.3
"
inherit cargo
SRC_URI="${CARGO_CRATE_URIS}"
`
	if err := os.WriteFile(filepath.Join(pkgDir, "rustapp-1.0.ebuild"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	parsed, err := g2.ParseEbuild(os.DirFS(pkgDir), "rustapp-1.0.ebuild", g2.ParseFull)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.SrcUriUncertain {
		t.Fatal("expected unresolved eclass-generated source URI")
	}
	manifest := &g2.Manifest{Entries: []*g2.ManifestEntry{
		{Type: "DIST", Filename: "example-1.2.3.crate"},
		{Type: "DIST", Filename: "other-1.0.tar.gz"},
	}}
	rule := &ebuild.OrphanedManifestLintRule{}
	for _, tt := range []struct {
		name string
		pkg  *g2.PackageData
	}{
		{
			name: "parsed package versions",
			pkg: &g2.PackageData{
				Category: "net-im", Name: "rustapp",
				Versions: []g2.VersionData{{Version: "1.0", Ebuild: parsed}},
				Manifest: manifest,
			},
		},
		{
			name: "filesystem discovery",
			pkg: &g2.PackageData{
				Category: "net-im", Name: "rustapp",
				Manifest: manifest,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			results := rule.Lint(repoDir, tt.pkg)
			if len(results) != 0 {
				t.Fatalf("unresolved source URI: expected no false orphan reports; got %v", results)
			}
		})
	}
}

// The uncertainty guard must not disable ordinary orphan detection.
func TestOrphanedManifestLintRule_AuthoritativeSrcURI(t *testing.T) {
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "app-misc", "known")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "known-1.ebuild"), []byte(`SRC_URI="https://example.org/used-1.tar.gz"`), 0644); err != nil {
		t.Fatal(err)
	}
	parsed, err := g2.ParseEbuild(os.DirFS(pkgDir), "known-1.ebuild", g2.ParseFull)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SrcUriUncertain {
		t.Fatalf("unexpected unresolved source URI")
	}
	pkg := &g2.PackageData{
		Category: "app-misc", Name: "known",
		Versions: []g2.VersionData{{Version: "1", Ebuild: parsed}},
		Manifest: &g2.Manifest{Entries: []*g2.ManifestEntry{
			{Type: "DIST", Filename: "used-1.tar.gz"},
			{Type: "DIST", Filename: "orphaned-1.tar.gz"},
		}},
	}
	results := (&ebuild.OrphanedManifestLintRule{}).Lint(repoDir, pkg)
	if len(results) != 1 || !strings.Contains(results[0].Message, "orphaned-1.tar.gz") {
		t.Fatalf("expected exactly one genuine orphan, got %v", results)
	}
}
