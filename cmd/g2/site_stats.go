package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/arran4/g2"
)

// PhaseTiming records wall-clock and optional accumulated worker duration for a build phase.
type PhaseTiming struct {
	Name          string `json:"name"`
	WallMS        int64  `json:"wall_ms"`
	WallHuman     string `json:"wall_human"`
	WorkerMS      int64  `json:"worker_ms,omitempty"`
	WorkerHuman   string `json:"worker_human,omitempty"`
	IsParallel    bool   `json:"is_parallel,omitempty"`
}

// OutputStorageStats summarizes generated output files and logical byte sizes by category.
type OutputStorageStats struct {
	TotalFiles        int    `json:"total_files"`
	TotalLogicalBytes int64  `json:"total_logical_bytes"`
	TotalHuman        string `json:"total_human"`
	HTMLFiles         int    `json:"html_files"`
	HTMLBytes         int64  `json:"html_bytes"`
	HTMLHuman         string `json:"html_human"`
	SearchFiles       int    `json:"search_files"`
	SearchBytes       int64  `json:"search_bytes"`
	SearchHuman       string `json:"search_human"`
	AssetFiles        int    `json:"asset_files"`
	AssetBytes        int64  `json:"asset_bytes"`
	AssetHuman        string `json:"asset_human"`
	FeedFiles         int    `json:"feed_files"`
	FeedBytes         int64  `json:"feed_bytes"`
	FeedHuman         string `json:"feed_human"`
	OtherFiles        int    `json:"other_files"`
	OtherBytes        int64  `json:"other_bytes"`
	OtherHuman        string `json:"other_human"`
	WalkSkippedDetail bool   `json:"walk_skipped_detail,omitempty"`
	Note              string `json:"note"`
}

// RuntimeMemoryStats captures Go runtime memory statistics at finalization.
type RuntimeMemoryStats struct {
	HeapAllocBytes  uint64 `json:"heap_alloc_bytes"`
	HeapAllocHuman  string `json:"heap_alloc_human"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	TotalAllocHuman string `json:"total_alloc_human"`
	SysBytes        uint64 `json:"sys_bytes"`
	SysHuman        string `json:"sys_human"`
	NumGC           uint32 `json:"num_gc"`
}

// RepoBuildStats summarizes per-repository acquisition/parse timings and size.
type RepoBuildStats struct {
	RepoName     string `json:"repo_name"`
	PackageCount int    `json:"package_count"`
	CheckoutTime string `json:"checkout_time,omitempty"`
	ParseTime    string `json:"parse_time,omitempty"`
	GitSize      string `json:"git_size,omitempty"`
}

// SiteStatsReport holds always-on, sanitized generation metrics published on /stats/.
type SiteStatsReport struct {
	Version           string             `json:"version"`
	StartedAt         string             `json:"started_at"`
	CompletedAt       string             `json:"completed_at"`
	TotalWallMS       int64              `json:"total_wall_ms"`
	TotalWallHuman    string             `json:"total_wall_human"`
	RepoCount         int                `json:"repo_count"`
	TotalNodes        int                `json:"total_nodes"`
	FastGit           bool               `json:"fast_git"`
	RecentDuration    string             `json:"recent_duration"`
	SanitizedArgs     []string           `json:"sanitized_args,omitempty"`
	Phases            []PhaseTiming      `json:"phases"`
	Storage           OutputStorageStats `json:"storage"`
	Memory            RuntimeMemoryStats `json:"memory"`
	Repos             []RepoBuildStats   `json:"repos,omitempty"`
	StatsJSONEnabled  bool               `json:"stats_json_enabled"`
}

// SiteStatsCollector records generation phase timings using an injectable clock.
type SiteStatsCollector struct {
	now       func() time.Time
	startedAt time.Time
	phases    []PhaseTiming
}

func newSiteStatsCollector(nowFn func() time.Time) *SiteStatsCollector {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &SiteStatsCollector{
		now:       nowFn,
		startedAt: nowFn().UTC(),
	}
}

func (c *SiteStatsCollector) trackPhase(name string) func() {
	start := c.now()
	return func() {
		d := c.now().Sub(start)
		if d < 0 {
			d = 0
		}
		c.phases = append(c.phases, PhaseTiming{
			Name:      name,
			WallMS:    d.Milliseconds(),
			WallHuman: formatDurationCompact(d),
		})
	}
}

func (c *SiteStatsCollector) addParallelPhase(name string, wall, worker time.Duration) {
	if wall < 0 {
		wall = 0
	}
	if worker < 0 {
		worker = 0
	}
	c.phases = append(c.phases, PhaseTiming{
		Name:        name,
		WallMS:      wall.Milliseconds(),
		WallHuman:   formatDurationCompact(wall),
		WorkerMS:    worker.Milliseconds(),
		WorkerHuman: formatDurationCompact(worker),
		IsParallel:  true,
	})
}

func formatDurationCompact(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Round(time.Millisecond).String()
}

func formatBytesIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// scanOutputStorage performs a single bounded walk of outDir to measure logical output sizes.
// It excludes stats/index.html and stats/stats.json to avoid rescan recursion when stats are written last.
func scanOutputStorage(outDir string) (OutputStorageStats, error) {
	stats := OutputStorageStats{
		Note: "Logical file byte sizes (excluding final /stats/index.html and /stats/stats.json).",
	}
	err := filepath.WalkDir(outDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(outDir, p)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "stats/index.html" || relSlash == "stats/stats.json" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		sz := info.Size()
		stats.TotalFiles++
		stats.TotalLogicalBytes += sz

		lower := strings.ToLower(relSlash)
		base := strings.ToLower(filepath.Base(relSlash))
		switch {
		case base == "search_index.json" || base == "search.js" || strings.HasPrefix(base, "search_"):
			stats.SearchFiles++
			stats.SearchBytes += sz
		case strings.HasSuffix(lower, ".html"):
			stats.HTMLFiles++
			stats.HTMLBytes += sz
		case strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".css"):
			stats.AssetFiles++
			stats.AssetBytes += sz
		case strings.HasSuffix(lower, ".rss") || strings.HasSuffix(lower, ".atom") || strings.HasSuffix(lower, ".xml"):
			stats.FeedFiles++
			stats.FeedBytes += sz
		default:
			stats.OtherFiles++
			stats.OtherBytes += sz
		}
		return nil
	})
	stats.TotalHuman = formatBytesIEC(stats.TotalLogicalBytes)
	stats.HTMLHuman = formatBytesIEC(stats.HTMLBytes)
	stats.SearchHuman = formatBytesIEC(stats.SearchBytes)
	stats.AssetHuman = formatBytesIEC(stats.AssetBytes)
	stats.FeedHuman = formatBytesIEC(stats.FeedBytes)
	stats.OtherHuman = formatBytesIEC(stats.OtherBytes)
	return stats, err
}

func captureRuntimeMemory() RuntimeMemoryStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return RuntimeMemoryStats{
		HeapAllocBytes:  ms.HeapAlloc,
		HeapAllocHuman:  formatBytesIEC(int64(ms.HeapAlloc)),
		TotalAllocBytes: ms.TotalAlloc,
		TotalAllocHuman: formatBytesIEC(int64(ms.TotalAlloc)),
		SysBytes:        ms.Sys,
		SysHuman:        formatBytesIEC(int64(ms.Sys)),
		NumGC:           ms.NumGC,
	}
}

// sanitizeCLIArgs removes credentials, tokens, and absolute local filesystem paths before publishing.
func sanitizeCLIArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, 0, len(args))
	redactNext := false
	for _, a := range args {
		if redactNext {
			out = append(out, "<redacted>")
			redactNext = false
			continue
		}
		lower := strings.ToLower(a)
		if strings.HasPrefix(lower, "-") {
			if idx := strings.IndexByte(a, '='); idx != -1 {
				flagName := strings.ToLower(a[:idx])
				val := a[idx+1:]
				if strings.Contains(flagName, "token") || strings.Contains(flagName, "secret") || strings.Contains(flagName, "pass") || strings.Contains(flagName, "key") {
					out = append(out, a[:idx]+"=<redacted>")
					continue
				}
				if strings.HasPrefix(val, "/") || strings.HasPrefix(val, "~") {
					out = append(out, a[:idx]+"="+filepath.Base(val))
					continue
				}
				out = append(out, a[:idx]+"="+sanitizeURLOrPath(val))
				continue
			}
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "pass") || strings.Contains(lower, "key") {
				out = append(out, a)
				redactNext = true
				continue
			}
			out = append(out, a)
			continue
		}
		out = append(out, sanitizeURLOrPath(a))
	}
	return out
}

func sanitizeURLOrPath(val string) string {
	if strings.Contains(val, "://") {
		if u, err := url.Parse(val); err == nil {
			if u.User != nil {
				u.User = url.User("redacted")
			}
			return u.String()
		}
	}
	if filepath.IsAbs(val) || strings.HasPrefix(val, "~/") {
		base := filepath.Base(val)
		if base == "" || base == "." || base == "/" {
			return "<path>"
		}
		return "<path>/" + base
	}
	return val
}

func buildRepoBuildStats(sites []*g2.SiteData) []RepoBuildStats {
	if len(sites) == 0 {
		return nil
	}
	out := make([]RepoBuildStats, 0, len(sites))
	for _, s := range sites {
		if s == nil {
			continue
		}
		out = append(out, RepoBuildStats{
			RepoName:     s.RepoName,
			PackageCount: s.PackageCount,
			CheckoutTime: s.CheckoutTime,
			ParseTime:    s.ProcessTime,
			GitSize:      s.GitSize,
		})
	}
	return out
}

func sumRepoWorkerDurations(sites []*g2.SiteData) (checkoutWorker, parseWorker time.Duration) {
	for _, s := range sites {
		if s == nil {
			continue
		}
		if s.CheckoutTime != "" {
			if d, err := time.ParseDuration(s.CheckoutTime); err == nil {
				checkoutWorker += d
			}
		}
		if s.ProcessTime != "" {
			if d, err := time.ParseDuration(s.ProcessTime); err == nil {
				parseWorker += d
			}
		}
	}
	return checkoutWorker, parseWorker
}

func writeStatsJSON(outDir string, report *SiteStatsReport) error {
	if report == nil {
		return nil
	}
	statsDir := filepath.Join(outDir, "stats")
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(statsDir, "stats.json"), data, 0o644)
}
