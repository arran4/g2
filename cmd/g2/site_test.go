package main

import (
	"reflect"
	"testing"

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
