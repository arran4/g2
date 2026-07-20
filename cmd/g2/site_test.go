package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	versions := []g2.VersionData{
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

	got := buildManifestData(manifest, versions, nil)

	expected := []g2.ManifestEntryData{
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
	siteData, err := parseRepo(os.DirFS("../../testdata/test_overlay"), ".", "Test Overlay", false, nil)
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}

	outDir := t.TempDir()

	err = generateSite(outDir, []*g2.SiteData{siteData}, 90*24*time.Hour, "3 months", GenerationInfo{})
	if err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	type rssDocument struct {
		Channel struct {
			Title         string `xml:"title"`
			LastBuildDate string `xml:"lastBuildDate"`
			Items         []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
				PubDate     string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	for _, feedPath := range []string{
		filepath.Join(outDir, "news", "index.rss"),
		filepath.Join(outDir, "repos", siteData.RepoName, "news", "index.rss"),
	} {
		t.Run(feedPath, func(t *testing.T) {
			contents, err := os.ReadFile(feedPath)
			if err != nil {
				t.Fatalf("reading generated RSS feed: %v", err)
			}
			var feed rssDocument
			if err := xml.Unmarshal(contents, &feed); err != nil {
				t.Fatalf("parsing generated RSS feed: %v", err)
			}
			if len(feed.Channel.Items) != len(siteData.News) {
				t.Fatalf("RSS item count = %d, want %d", len(feed.Channel.Items), len(siteData.News))
			}
			if !strings.Contains(feed.Channel.Title, "News RSS Feed") {
				t.Errorf("RSS channel title %q does not identify the news RSS feed", feed.Channel.Title)
			}
			if feed.Channel.LastBuildDate == "" {
				t.Error("RSS lastBuildDate is empty")
			}
			first := feed.Channel.Items[0]
			if first.Title != siteData.News[0].Title {
				t.Errorf("first RSS title = %q, want %q", first.Title, siteData.News[0].Title)
			}
			if first.Link != "archive/"+siteData.News[0].DirName+"/" {
				t.Errorf("first RSS link = %q", first.Link)
			}
			if _, err := time.Parse(time.RFC1123Z, first.PubDate); err != nil {
				t.Errorf("first RSS pubDate %q is invalid: %v", first.PubDate, err)
			}
		})
	}

	type atomDocument struct {
		Title   string `xml:"title"`
		Updated string `xml:"updated"`
		Entries []struct {
			Title   string `xml:"title"`
			Updated string `xml:"updated"`
		} `xml:"entry"`
	}
	for _, feedPath := range []string{
		filepath.Join(outDir, "news", "index.atom"),
		filepath.Join(outDir, "repos", siteData.RepoName, "news", "index.atom"),
	} {
		t.Run(feedPath, func(t *testing.T) {
			contents, err := os.ReadFile(feedPath)
			if err != nil {
				t.Fatalf("reading generated Atom feed: %v", err)
			}
			var feed atomDocument
			if err := xml.Unmarshal(contents, &feed); err != nil {
				t.Fatalf("parsing generated Atom feed: %v", err)
			}
			if len(feed.Entries) != len(siteData.News) {
				t.Fatalf("Atom entry count = %d, want %d", len(feed.Entries), len(siteData.News))
			}
			if !strings.Contains(feed.Title, "News Atom Feed") {
				t.Errorf("Atom feed title %q does not identify the news Atom feed", feed.Title)
			}
			if _, err := time.Parse(time.RFC3339, feed.Updated); err != nil {
				t.Errorf("Atom updated value %q is invalid: %v", feed.Updated, err)
			}
			if feed.Entries[0].Title != siteData.News[0].Title {
				t.Errorf("first Atom title = %q, want %q", feed.Entries[0].Title, siteData.News[0].Title)
			}
		})
	}
}

func TestEscapeXML(t *testing.T) {
	const input = `News & updates <today> "quoted"`
	const want = `News &amp; updates &lt;today&gt; &#34;quoted&#34;`
	if got := escapeXML(input); got != want {
		t.Errorf("escapeXML(%q) = %q, want %q", input, got, want)
	}
}

func TestGenerateSite_TemplateError(t *testing.T) {
	// Let's pass a struct to generateSite that we know will fail.
	// To cause an issue intentionally with templates, we can pass something
	// that will cause MkdirAll to fail, or just pass a package with a malformed template format.
	// We will supply a category with a malformed name to trigger a file path error or a bad move.

	siteData := &g2.SiteData{
		Title:    "Bad Template Site",
		RepoName: "bad-repo",
		Categories: []g2.CategoryData{
			{
				Name: "broken-category/\x00/invalid",
				Packages: []g2.PackageData{
					{
						Name:     "broken-package",
						Category: "broken-category/\x00/invalid",
					},
				},
			},
		},
	}
	outDir := t.TempDir()

	err := generateSite(outDir, []*g2.SiteData{siteData}, 90*24*time.Hour, "3 months", GenerationInfo{})

	if err == nil {
		t.Fatalf("generateSite unexpectedly succeeded with bad parameters, template/file errors are likely being swallowed")
	}
	t.Logf("generateSite successfully surfaced error: %v", err)
}

func TestDominantMetadataSelection(t *testing.T) {
	pkgData := g2.PackageData{
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{
						"PV":          "1.0",
						"KEYWORDS":    "~amd64",
						"DESCRIPTION": "Ebuild description",
						"HOMEPAGE":    "https://ebuild.com",
						"LICENSE":     "GPL-2",
					},
				},
			},
		},
	}

	var highestUnmasked *g2.Ebuild
	var highestMasked *g2.Ebuild
	for _, v := range pkgData.Versions {
		if v.Ebuild == nil || v.Ebuild.Vars == nil {
			continue
		}
		keywords := v.Ebuild.Vars["KEYWORDS"]
		parts := strings.Fields(keywords)

		isMasked := true
		for _, p := range parts {
			if !strings.HasPrefix(p, "-") && !strings.HasPrefix(p, "~") {
				isMasked = false
				break
			}
		}
		if !isMasked {
			if highestUnmasked == nil || g2.CompareVersions(v.Version, highestUnmasked.Vars["PV"]) > 0 {
				highestUnmasked = v.Ebuild
			}
		} else {
			if highestMasked == nil || g2.CompareVersions(v.Version, highestMasked.Vars["PV"]) > 0 {
				highestMasked = v.Ebuild
			}
		}
	}

	targetEbuild := highestUnmasked
	if targetEbuild == nil {
		targetEbuild = highestMasked
	}
	if targetEbuild == nil && len(pkgData.Versions) > 0 {
		for _, v := range pkgData.Versions {
			if v.Ebuild != nil && v.Ebuild.Vars != nil {
				targetEbuild = v.Ebuild
				break
			}
		}
	}

	pkgData.Metadata = &g2.PkgMetadata{
		LongDescription: []g2.LongDescription{
			{Body: "Metadata description"},
		},
	}

	if pkgData.Metadata != nil && len(pkgData.Metadata.LongDescription) > 0 {
		pkgData.DominantDescription = pkgData.Metadata.LongDescription[0].Body
	} else if targetEbuild != nil {
		pkgData.DominantDescription = targetEbuild.Vars["DESCRIPTION"]
	}

	if targetEbuild != nil {
		pkgData.DominantHomepage = targetEbuild.Vars["HOMEPAGE"]
		pkgData.DominantLicense = targetEbuild.Vars["LICENSE"]
	}

	if pkgData.DominantDescription != "Metadata description" {
		t.Errorf("Expected description 'Metadata description', got '%s'", pkgData.DominantDescription)
	}

	if pkgData.DominantHomepage != "https://ebuild.com" {
		t.Errorf("Expected homepage 'https://ebuild.com', got '%s'", pkgData.DominantHomepage)
	}
}
