package main

import (
	"fmt"
	"sort"

	"github.com/arran4/g2"
)

const defaultLeaderboardLimit = 10

// LeaderboardEntry represents a single ranked item in a top-N leaderboard.
type LeaderboardEntry struct {
	Rank   int
	Name   string
	URL    string
	Count  int
	Detail string
}

// RepoInventorySummary holds concise per-repository inventory and timing metrics.
type RepoInventorySummary struct {
	RepoName       string
	Packages       int
	Ebuilds        int
	Categories     int
	NewsItems      int
	NewsVariants   int
	LatestNewsDate string
	CheckoutTime   string
	ParseTime      string
	GitSize        string
}

// SiteInventoryStats holds aggregated site inventory, averages, and bounded top-N leaderboards.
type SiteInventoryStats struct {
	TotalRepos              int
	UniquePackages          int
	TotalRepoPackages       int
	TotalEbuilds            int
	TotalNewsItems          int
	TotalNewsVariants       int
	AvgPackagesPerRepoStr   string
	AvgNewsPerRepoStr       string
	AvgEbuildsPerPackageStr string
	RepoSummaries           []RepoInventorySummary
	TopReposByPackages      []LeaderboardEntry
	TopReposByEbuilds       []LeaderboardEntry
	TopReposByNews          []LeaderboardEntry
	TopPackagesByVersions   []LeaderboardEntry
	TopPackagesByRepos      []LeaderboardEntry
}

func computeRepoInventorySummary(site *g2.SiteData) RepoInventorySummary {
	if site == nil {
		return RepoInventorySummary{}
	}
	ebuilds := 0
	for _, cat := range site.Categories {
		for _, pkg := range cat.Packages {
			ebuilds += len(pkg.Versions)
		}
	}
	newsCount, variants, latestDate := countNewsAndLatestDate(site.News)
	return RepoInventorySummary{
		RepoName:       site.RepoName,
		Packages:       site.PackageCount,
		Ebuilds:        ebuilds,
		Categories:     len(site.Categories),
		NewsItems:      newsCount,
		NewsVariants:   variants,
		LatestNewsDate: latestDate,
		CheckoutTime:   site.CheckoutTime,
		ParseTime:      site.ProcessTime,
		GitSize:        site.GitSize,
	}
}

func countNewsAndLatestDate(items []g2.NewsItem) (uniqueCount, variants int, latestDate string) {
	if len(items) == 0 {
		return 0, 0, ""
	}
	seenIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		variants++
		id := item.DirName
		if id == "" {
			id = item.Title + "|" + item.Posted.Format("2006-01-02")
		}
		seenIDs[id] = struct{}{}
		if !item.Posted.IsZero() {
			d := item.Posted.Format("2006-01-02")
			if d > latestDate {
				latestDate = d
			}
		}
	}
	return len(seenIDs), variants, latestDate
}

func computeSiteInventoryStats(sites []*g2.SiteData, data *AggregatedData) *SiteInventoryStats {
	stats := &SiteInventoryStats{
		TotalRepos: len(sites),
	}
	if data != nil {
		stats.UniquePackages = len(data.Packages)
	}

	summaries := make([]RepoInventorySummary, 0, len(sites))
	repoPkgBoard := make([]LeaderboardEntry, 0, len(sites))
	repoEbuildBoard := make([]LeaderboardEntry, 0, len(sites))
	repoNewsBoard := make([]LeaderboardEntry, 0, len(sites))

	for _, site := range sites {
		if site == nil {
			continue
		}
		sum := computeRepoInventorySummary(site)
		summaries = append(summaries, sum)
		stats.TotalRepoPackages += sum.Packages
		stats.TotalEbuilds += sum.Ebuilds
		stats.TotalNewsItems += sum.NewsItems
		stats.TotalNewsVariants += sum.NewsVariants

		repoURL := "repos/" + site.RepoName + "/"
		repoPkgBoard = append(repoPkgBoard, LeaderboardEntry{
			Name:  site.RepoName,
			URL:   repoURL,
			Count: sum.Packages,
		})
		repoEbuildBoard = append(repoEbuildBoard, LeaderboardEntry{
			Name:  site.RepoName,
			URL:   repoURL,
			Count: sum.Ebuilds,
		})
		repoNewsBoard = append(repoNewsBoard, LeaderboardEntry{
			Name:   site.RepoName,
			URL:    repoURL + "news/",
			Count:  sum.NewsItems,
			Detail: sum.LatestNewsDate,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RepoName < summaries[j].RepoName
	})
	stats.RepoSummaries = summaries
	stats.TopReposByPackages = topNLeaderboard(repoPkgBoard, defaultLeaderboardLimit)
	stats.TopReposByEbuilds = topNLeaderboard(repoEbuildBoard, defaultLeaderboardLimit)
	stats.TopReposByNews = topNLeaderboard(repoNewsBoard, defaultLeaderboardLimit)

	if data != nil && len(data.Packages) > 0 {
		pkgVerBoard := make([]LeaderboardEntry, 0, len(data.Packages))
		pkgRepoBoard := make([]LeaderboardEntry, 0, len(data.Packages))
		for _, pkg := range data.Packages {
			if pkg == nil {
				continue
			}
			fullName := pkg.Category + "/" + pkg.Name
			pkgURL := "packages/" + fullName + "/"
			verCount := 0
			for _, vers := range pkg.RepoVersions {
				verCount += len(vers)
			}
			pkgVerBoard = append(pkgVerBoard, LeaderboardEntry{
				Name:  fullName,
				URL:   pkgURL,
				Count: verCount,
			})
			pkgRepoBoard = append(pkgRepoBoard, LeaderboardEntry{
				Name:  fullName,
				URL:   pkgURL,
				Count: len(pkg.Repos),
			})
		}
		stats.TopPackagesByVersions = topNLeaderboard(pkgVerBoard, defaultLeaderboardLimit)
		stats.TopPackagesByRepos = topNLeaderboard(pkgRepoBoard, defaultLeaderboardLimit)
	}

	if stats.TotalRepos > 0 {
		stats.AvgPackagesPerRepoStr = fmt.Sprintf("%.1f", float64(stats.TotalRepoPackages)/float64(stats.TotalRepos))
		stats.AvgNewsPerRepoStr = fmt.Sprintf("%.1f", float64(stats.TotalNewsItems)/float64(stats.TotalRepos))
	} else {
		stats.AvgPackagesPerRepoStr = "0.0"
		stats.AvgNewsPerRepoStr = "0.0"
	}
	if stats.UniquePackages > 0 {
		stats.AvgEbuildsPerPackageStr = fmt.Sprintf("%.1f", float64(stats.TotalEbuilds)/float64(stats.UniquePackages))
	} else {
		stats.AvgEbuildsPerPackageStr = "0.0"
	}

	return stats
}

// topNLeaderboard sorts descending by Count with deterministic ascending Name tie-breaking.
func topNLeaderboard(entries []LeaderboardEntry, limit int) []LeaderboardEntry {
	if len(entries) == 0 || limit <= 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Name < entries[j].Name
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	for i := range entries {
		entries[i].Rank = i + 1
	}
	return entries
}
