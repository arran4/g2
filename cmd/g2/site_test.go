package main

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/arran4/g2"
)

func TestBuildManifestData(t *testing.T) {
	manifest := &g2.Manifest{
		Entries: []*g2.ManifestEntry{
			{Type: "DIST", Filename: "foo-1.0.tar.gz", Size: 100},
			{Type: "DIST", Filename: "foo-2.0-custom.tar.gz", Size: 200},
			{Type: "DIST", Filename: "common.patch", Size: 50},
			{Type: "DIST", Filename: "unused.tar.gz", Size: 300},
		},
	}

	versions := []VersionData{
		{
			Version: "1.0",
			Ebuild: &g2.Ebuild{
				Vars: map[string]string{"PV": "1.0"},
				SrcUri: []g2.URIEntry{
					{URL: "https://example.com/foo-1.0.tar.gz"}, // Implicit filename
					{URL: "https://example.com/common.patch"},
				},
			},
		},
		{
			Version: "2.0",
			Ebuild: &g2.Ebuild{
				Vars: map[string]string{"PV": "2.0"},
				SrcUri: []g2.URIEntry{
					{URL: "https://example.com/foo-2.0.tar.gz", Filename: "foo-2.0-custom.tar.gz"}, // Explicit filename
					{URL: "https://example.com/common.patch"},
				},
			},
		},
		{
			Version: "2.0-r1",
			Ebuild: &g2.Ebuild{
				// Test fallback when PV is missing
				Vars: nil,
				SrcUri: []g2.URIEntry{
					{URL: "https://example.com/foo-2.0.tar.gz", Filename: "foo-2.0-custom.tar.gz"},
					{URL: "https://example.com/common.patch"},
					{URL: "https://example.com/alt.patch"}, // Not in manifest
				},
			},
		},
	}

	got := buildManifestData(manifest, versions)

	expected := []ManifestEntryData{
		{
			Entry:    manifest.Entries[0],
			Versions: []string{"1.0"},
			URLs:     []string{"https://example.com/foo-1.0.tar.gz"},
		},
		{
			Entry:    manifest.Entries[1],
			Versions: []string{"2.0-r1", "2.0"}, // Sorted descending
			URLs:     []string{"https://example.com/foo-2.0.tar.gz"},
		},
		{
			Entry:    manifest.Entries[2],
			Versions: []string{"2.0-r1", "2.0", "1.0"}, // Sorted descending
			URLs:     []string{"https://example.com/common.patch"},
		},
		{
			Entry:    manifest.Entries[3],
			Versions: nil,
			URLs:     nil,
		},
	}

	if len(got) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(got))
	}

	for i := range expected {
		if got[i].Entry != expected[i].Entry {
			t.Errorf("entry %d: expected entry %v, got %v", i, expected[i].Entry, got[i].Entry)
		}
		if !reflect.DeepEqual(got[i].Versions, expected[i].Versions) {
			t.Errorf("entry %d: expected versions %v, got %v", i, expected[i].Versions, got[i].Versions)
		}
		if !reflect.DeepEqual(got[i].URLs, expected[i].URLs) {
			t.Errorf("entry %d: expected URLs %v, got %v", i, expected[i].URLs, got[i].URLs)
		}
	}
}

func TestGenerateSite(t *testing.T) {
	siteData, err := parseRepo(os.DirFS("../../testdata/test_overlay"), ".", "Test Overlay", false)
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}

	outDir := t.TempDir()

	err = generateSite(outDir, []*SiteData{siteData}, 90*24*time.Hour, "3 months")
	if err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}
}

func TestGenerateSite_TemplateError(t *testing.T) {
	// Let's pass a struct to generateSite that we know will fail.
	// To cause an issue intentionally with templates, we can pass something
	// that will cause MkdirAll to fail, or just pass a package with a malformed template format.
	// We will supply a category with a malformed name to trigger a file path error or a bad move.

	siteData := &SiteData{
		Title:    "Bad Template Site",
		RepoName: "bad-repo",
		Categories: []CategoryData{
			{
				Name: "broken-category/\x00/invalid",
				Packages: []PackageData{
					{
						Name:     "broken-package",
						Category: "broken-category/\x00/invalid",
					},
				},
			},
		},
	}
	outDir := t.TempDir()

	err := generateSite(outDir, []*SiteData{siteData}, 90*24*time.Hour, "3 months")

	if err == nil {
		t.Fatalf("generateSite unexpectedly succeeded with bad parameters, template/file errors are likely being swallowed")
	}
	t.Logf("generateSite successfully surfaced error: %v", err)
}
