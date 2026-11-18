package main

import (
	"encoding/json"
	"io"
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

func TestSanitizeCLIArgs(t *testing.T) {
	in := []string{
		"overlay", "site", "generate",
		"-out", "/home/alice/private/site_out",
		"-token=supersecret123",
		"-api-key", "secretval",
		"https://alice:hunter2@example.com/repo.git",
		"/tmp/local/overlay",
	}
	got := sanitizeCLIArgs(in)
	joined := strings.Join(got, " ")
	for _, forbidden := range []string{"supersecret123", "secretval", "hunter2", "/home/alice/private", "/tmp/local"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("sanitized args leaked %q: %v", forbidden, got)
		}
	}
	if !strings.Contains(joined, "https://redacted@example.com/repo.git") {
		t.Fatalf("expected redacted userinfo in URL, got: %v", got)
	}
	if !strings.Contains(joined, "<path>/overlay") {
		t.Fatalf("expected basename-only path, got: %v", got)
	}
}

func TestScanOutputStorageExcludesStats(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("index.html", "12345")              // 5 bytes HTML
	writeFile("search_index.json", "1234567890")  // 10 bytes search
	writeFile("app.css", "123")                   // 3 bytes asset
	writeFile("news/index.rss", "1234567")        // 7 bytes feed
	writeFile("robots.txt", "12")                 // 2 bytes other
	writeFile("stats/index.html", "shouldignore") // excluded
	writeFile("stats/stats.json", "shouldignore") // excluded

	st, err := scanOutputStorage(dir)
	if err != nil {
		t.Fatalf("scanOutputStorage failed: %v", err)
	}
	if st.TotalFiles != 5 || st.TotalLogicalBytes != 27 {
		t.Fatalf("expected 5 files / 27 bytes, got %d files / %d bytes", st.TotalFiles, st.TotalLogicalBytes)
	}
	if st.HTMLBytes != 5 || st.SearchBytes != 10 || st.AssetBytes != 3 || st.FeedBytes != 7 || st.OtherBytes != 2 {
		t.Fatalf("unexpected category breakdown: %+v", st)
	}
}

func TestSiteStatsMockedClockAndHTTP(t *testing.T) {
	memFS1 := fstest.MapFS{
		"profiles/repo_name":            {Data: []byte("repo-one\n")},
		"app-misc/one/one-1.0.ebuild":   {Data: []byte("EAPI=8\nDESCRIPTION=\"One\"\nSLOT=\"0\"\nKEYWORDS=\"~amd64\"\n")},
	}
	memFS2 := fstest.MapFS{
		"profiles/repo_name":            {Data: []byte("repo-two\n")},
		"app-misc/two/two-1.0.ebuild":   {Data: []byte("EAPI=8\nDESCRIPTION=\"Two\"\nSLOT=\"0\"\nKEYWORDS=\"~amd64\"\n")},
	}
	s1, err := parseRepo(memFS1, ".", "repo-one", false, nil, SourceURL("https://example.com/repo-one.git"))
	if err != nil {
		t.Fatal(err)
	}
	s1.CheckoutTime = "1.5s"
	s1.ProcessTime = "500ms"
	s2, err := parseRepo(memFS2, ".", "repo-two", false, nil, SourceURL("https://example.com/repo-two.git"))
	if err != nil {
		t.Fatal(err)
	}
	s2.CheckoutTime = "2.5s"
	s2.ProcessTime = "500ms"

	tick := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	nowFn := func() time.Time {
		tick = tick.Add(25 * time.Millisecond)
		return tick
	}

	outDir := t.TempDir()
	checkoutW, parseW := sumRepoWorkerDurations([]*g2.SiteData{s1, s2})
	genInfo := GenerationInfo{
		Args:           []string{"overlays", "site", "generate", "-token=secret123", "/home/alice/overlays.xml"},
		FastGit:        true,
		RecentDuration: "30d",
		FetchParseWall: 3 * time.Second,
		CheckoutWorker: checkoutWorkerOrZero(checkoutW),
		ParseWorker:    parseW,
		NowFn:          nowFn,
	}

	if err := generateSite(outDir, []*g2.SiteData{s1, s2}, 30*24*time.Hour, "30d", genInfo); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

	// Serve at both / and /test-project/
	for _, prefix := range []string{"/", "/test-project/"} {
		var srv *httptest.Server
		if prefix == "/" {
			srv = httptest.NewServer(http.FileServer(http.Dir(outDir)))
		} else {
			srv = httptest.NewServer(http.StripPrefix(prefix, http.FileServer(http.Dir(outDir))))
		}

		resp, err := http.Get(srv.URL + prefix + "stats/")
		if err != nil {
			srv.Close()
			t.Fatalf("GET %sstats/: %v", prefix, err)
		}
		htmlBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			srv.Close()
			t.Fatalf("GET %sstats/ status=%d", prefix, resp.StatusCode)
		}
		htmlStr := string(htmlBytes)
		for _, want := range []string{"Phase Timing Breakdown", "Output Storage Breakdown", "Go Runtime Memory Snapshot", "Acquisition &amp; Parse (Repos)", "repo-one", "repo-two"} {
			if !strings.Contains(htmlStr, want) {
				srv.Close()
				t.Fatalf("stats page at %s missing %q", prefix, want)
			}
		}
		if strings.Contains(htmlStr, "secret123") || strings.Contains(htmlStr, "/home/alice") {
			srv.Close()
			t.Fatalf("stats page leaked sensitive args")
		}

		jsonResp, err := http.Get(srv.URL + prefix + "stats/stats.json")
		if err != nil {
			srv.Close()
			t.Fatalf("GET %sstats/stats.json: %v", prefix, err)
		}
		jsonBytes, _ := io.ReadAll(jsonResp.Body)
		_ = jsonResp.Body.Close()
		srv.Close()
		if jsonResp.StatusCode != http.StatusOK {
			t.Fatalf("GET %sstats/stats.json status=%d", prefix, jsonResp.StatusCode)
		}
		var report SiteStatsReport
		if err := json.Unmarshal(jsonBytes, &report); err != nil {
			t.Fatalf("unmarshal stats.json: %v", err)
		}
		if report.RepoCount != 2 || report.Storage.TotalFiles == 0 || report.Storage.TotalLogicalBytes == 0 {
			t.Fatalf("unexpected stats.json report: %+v", report)
		}
		if len(report.Phases) < 8 || !report.Phases[0].IsParallel || report.Phases[0].WallMS != 3000 || report.Phases[0].WorkerMS != 5000 {
			t.Fatalf("unexpected parallel phase timing in stats.json: %+v", report.Phases)
		}
	}
}

func checkoutWorkerOrZero(d time.Duration) time.Duration {
	return d
}

func TestProfilerJSONOutput(t *testing.T) {
	pFile := filepath.Join(t.TempDir(), "profile.json")
	p := NewProfiler(true, pFile)
	p.Record("Test Phase", 42*time.Millisecond)
	if err := p.Write(); err != nil {
		t.Fatalf("Profiler.Write failed: %v", err)
	}
	raw, err := os.ReadFile(pFile)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		TotalMS int64          `json:"total_ms"`
		Events  []ProfileEvent `json:"events"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid profile JSON: %v", err)
	}
	if len(parsed.Events) != 1 || parsed.Events[0].Name != "Test Phase" {
		t.Fatalf("unexpected profile JSON events: %+v", parsed.Events)
	}
}
