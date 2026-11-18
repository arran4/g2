package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arran4/g2"
)

func TestMultilangNewsIngestionAndFallbackHTTP(t *testing.T) {
	now := time.Now().UTC()
	oldDate := now.Add(-60 * 24 * time.Hour).Format("2006-01-02")

	memFS := fstest.MapFS{
		"profiles/repo_name": {Data: []byte("news-overlay\n")},
		// 1. Multilingual with English primary
		"metadata/news/" + oldDate + "-alpha/" + oldDate + "-alpha.en.txt": {
			Data: []byte("Title: Alpha English\nAuthor: Jane Doe <jane@example.com>\nPosted: " + oldDate + "\nRevision: 1\nNews-Item-Format: 2.0\n\nEnglish body.\n"),
		},
		"metadata/news/" + oldDate + "-alpha/" + oldDate + "-alpha.de.txt": {
			Data: []byte("Title: Alpha Deutsch\nAuthor: Jane Doe <jane@example.com>\nTranslator: Max Mustermann <max@example.com>\nPosted: " + oldDate + "\nRevision: 1\nNews-Item-Format: 2.0\n\nDeutscher Inhalt.\n"),
		},
		// 2. Missing English, only de and fr
		"metadata/news/" + oldDate + "-beta/" + oldDate + "-beta.de.txt": {
			Data: []byte("Title: Beta Deutsch\nAuthor: Jane Doe <jane@example.com>\nPosted: " + oldDate + "\nRevision: 1\nNews-Item-Format: 2.0\n\nNur auf Deutsch.\n"),
		},
		"metadata/news/" + oldDate + "-beta/" + oldDate + "-beta.fr.txt": {
			Data: []byte("Title: Beta Francais\nAuthor: Jane Doe <jane@example.com>\nPosted: " + oldDate + "\nRevision: 1\nNews-Item-Format: 2.0\n\nSeulement en francais.\n"),
		},
		// 3. Empty news directory -> diagnostic
		"metadata/news/2026-01-01-empty/.keep": {Data: []byte("")},
	}

	site, err := parseRepo(memFS, ".", "news-overlay", false, nil, SourceURL("https://example.com/news.git"))
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}
	if len(site.News) != 2 {
		t.Fatalf("expected 2 news items, got %d", len(site.News))
	}
	// Deterministic sort on equal Posted dates: alpha before beta
	if site.News[0].DirName != oldDate+"-alpha" || site.News[1].DirName != oldDate+"-beta" {
		t.Fatalf("unexpected deterministic order: %s, %s", site.News[0].DirName, site.News[1].DirName)
	}
	if site.News[0].MissingEnglish || len(site.News[0].Translations) != 2 {
		t.Fatalf("expected alpha to have English + 2 translations: %+v", site.News[0])
	}
	if !site.News[1].MissingEnglish || len(site.News[1].Translations) != 2 {
		t.Fatalf("expected beta to have MissingEnglish=true and 2 translations: %+v", site.News[1])
	}
	if len(site.NewsDiagnostics) < 2 {
		t.Fatalf("expected diagnostics for missing English and empty dir, got: %v", site.NewsDiagnostics)
	}

	// Generate with 7d recentDuration so 60d-old news triggers fallback label
	outDir := t.TempDir()
	if err := generateSite(outDir, []*g2.SiteData{site}, 7*24*time.Hour, "7d", GenerationInfo{RecentDuration: "7d"}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		checks := []struct {
			path string
			want []string
		}{
			{
				path: "news/",
				want: []string{"Latest News Items (no items within 7d)", "Alpha English"},
			},
			{
				path: "repos/news-overlay/news/",
				want: []string{"Latest News Items (no items within 7d)", "Alpha English"},
			},
			{
				path: "news/archive/news-overlay/" + oldDate + "-alpha/",
				want: []string{"English body.", "de/"},
			},
			{
				path: "news/archive/news-overlay/" + oldDate + "-alpha/de/",
				want: []string{"Deutscher Inhalt.", "../"},
			},
			{
				path: "news/archive/news-overlay/" + oldDate + "-beta/",
				want: []string{"English translation not available.", "de/", "fr/"},
			},
			{
				path: "news/archive/news-overlay/" + oldDate + "-beta/de/",
				want: []string{"Nur auf Deutsch.", "../fr/"},
			},
			{
				path: "repos/news-overlay/news/archive/" + oldDate + "-beta/",
				want: []string{"English translation not available.", "de/", "fr/"},
			},
			{
				path: "repos/news-overlay/news/archive/" + oldDate + "-beta/fr/",
				want: []string{"Seulement en francais."},
			},
		}

		for _, tc := range checks {
			resp, err := http.Get(srv.URL + prefix + tc.path)
			if err != nil {
				srv.Close()
				t.Fatalf("GET %s%s: %v", prefix, tc.path, err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				srv.Close()
				t.Fatalf("GET %s%s status=%d", prefix, tc.path, resp.StatusCode)
			}
			s := string(body)
			for _, w := range tc.want {
				if !strings.Contains(s, w) {
					srv.Close()
					t.Fatalf("%s%s missing %q", prefix, tc.path, w)
				}
			}
		}
		srv.Close()
	}
}
