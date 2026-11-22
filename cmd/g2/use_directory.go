package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arran4/g2"
)

// UseTierConfig controls the three-tier USE-flag directory thresholds and pagination sizes.
type UseTierConfig struct {
	SmallMax  int // Tier 1: < SmallMax (default 1000) -> single alphabetical page
	MediumMax int // Tier 2: <= MediumMax (default 26000) -> A-Z buckets + uses_index.json
	PageSize  int // Max flags per bucket shard and max packages per detail page (default 1000)
	TopLimit  int // Tier 3: > MediumMax -> Top-N list on landing page (default 250)
}

func defaultUseTierConfig() UseTierConfig {
	return UseTierConfig{
		SmallMax:  1000,
		MediumMax: 26000,
		PageSize:  1000,
		TopLimit:  250,
	}
}

// UseDirectorySummary reports authoritative metadata-provenance counts.
type UseDirectorySummary struct {
	TotalFlags        int
	GlobalDescCount   int
	UseExpandCount    int
	LocalDescCount    int
	UndocumentedCount int
}

// UseIndexEntry is the compact entry stored in uses_index.json and rendered in listings.
type UseIndexEntry struct {
	N string `json:"n"`           // Flag name
	D string `json:"d"`           // Representative description
	C int    `json:"c"`           // Distinct package count
	I int    `json:"i,omitempty"` // Repository package instance count
	S string `json:"s"`           // Authoritative scope label: use.desc | use_expand | local/metadata | undocumented
}

// UseBucketLink links to a static A-Z / 0-9 / other bucket page.
type UseBucketLink struct {
	Bucket string
	Label  string
	Count  int
	URL    string
}

// UsePageNavLink represents a single page link in paginated bucket or detail listings.
type UsePageNavLink struct {
	Page    int
	URL     string
	Current bool
}

// UseDirectoryView provides data for uses.html / repo_uses.html across all three tiers.
type UseDirectoryView struct {
	Tier         string // "small" | "medium" | "large"
	ScopeTitle   string
	Summary      UseDirectorySummary
	Buckets      []UseBucketLink
	AllFlags     []UseIndexEntry // populated for Tier 1 ("small")
	TopFlags     []UseIndexEntry // populated for Tier 3 ("large")
	IndexJSONURL string
}

// UseBucketPage provides data for static A-Z / sharded bucket HTML pages (uses_bucket.html).
type UseBucketPage struct {
	Bucket         string
	Label          string
	Page           int
	TotalPages     int
	TotalFlags     int
	Flags          []UseIndexEntry
	FlagLinkPrefix string
	LandingURL     string
	IndexJSONURL   string
	PrevURL        string
	NextURL        string
	PageLinks      []UsePageNavLink
}

// UseDetailPackageRow is a deterministically ordered package usage entry on a flag detail page.
type UseDetailPackageRow struct {
	Category     string
	Name         string
	PkgKey       string
	RepoName     string
	DeclaredIUSE bool
	RequiredUse  bool
	Versions     []string
	LocalDesc    string
	MetadataDesc string
}

// UseOverlayLink links to the per-overlay filtered view for a USE flag.
type UseOverlayLink struct {
	RepoName string
	Count    int
	URL      string
}

// UseDetailPageView provides paginated package rows and overlay links for use.html / repo_use.html.
type UseDetailPageView struct {
	Flag          *AggUseFlag
	RepoScope     string
	TotalPackages int
	Page          int
	TotalPages    int
	IsPaginated   bool
	Packages      []UseDetailPackageRow
	OverlayLinks  []UseOverlayLink
	PrevURL       string
	NextURL       string
	PageLinks     []UsePageNavLink
}

func isValidUseDirFlagName(name string) bool {
	return isValidSiteUseFlagName(name)
}

func classifyUseMetadataScope(f *AggUseFlag) string {
	switch f.Scope {
	case ScopeGlobalDefined:
		return "use.desc"
	case ScopeUseExpand:
		return "use_expand"
	case ScopePackageScoped:
		return "local/metadata"
	case ScopeUndocumented:
		return "undocumented"
	}
	if f.GlobalDesc != "" {
		return "use.desc"
	}
	if len(f.LocalDescs) > 0 || len(f.MetadataDescs) > 0 {
		return "local/metadata"
	}
	return "undocumented"
}

func deterministicBestDesc(f *AggUseFlag) string {
	if f.GlobalDesc != "" {
		return f.GlobalDesc
	}
	if len(f.MetadataDescs) > 0 {
		keys := make([]string, 0, len(f.MetadataDescs))
		for k, v := range f.MetadataDescs {
			if v != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			return f.MetadataDescs[keys[0]]
		}
	}
	if len(f.LocalDescs) > 0 {
		keys := make([]string, 0, len(f.LocalDescs))
		for k, v := range f.LocalDescs {
			if v != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			return f.LocalDescs[keys[0]]
		}
	}
	return ""
}

func useBucketKey(name string) (bucket, label string) {
	if name == "" {
		return "other", "Other"
	}
	ch := name[0]
	if ch >= 'a' && ch <= 'z' {
		return string(ch), strings.ToUpper(string(ch))
	}
	if ch >= 'A' && ch <= 'Z' {
		lower := string(ch + ('a' - 'A'))
		return lower, string(ch)
	}
	if ch >= '0' && ch <= '9' {
		return "0-9", "0-9"
	}
	return "other", "Other"
}

func filterAndSortUseFlags(flags []*AggUseFlag) []*AggUseFlag {
	valid := make([]*AggUseFlag, 0, len(flags))
	seen := make(map[string]bool, len(flags))
	for _, f := range flags {
		if f == nil || !isValidUseDirFlagName(f.Name) || seen[f.Name] {
			continue
		}
		seen[f.Name] = true
		valid = append(valid, f)
	}
	sort.Slice(valid, func(i, j int) bool {
		return valid[i].Name < valid[j].Name
	})
	return valid
}

func buildUseDirectoryData(flags []*AggUseFlag, scopeTitle string, cfg UseTierConfig) (*UseDirectoryView, []UseIndexEntry, map[string][]UseIndexEntry, []*AggUseFlag) {
	if cfg.SmallMax <= 0 {
		cfg.SmallMax = 1000
	}
	if cfg.MediumMax < cfg.SmallMax {
		cfg.MediumMax = 26000
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = 1000
	}
	if cfg.TopLimit <= 0 {
		cfg.TopLimit = 250
	}

	validFlags := filterAndSortUseFlags(flags)
	summary := UseDirectorySummary{TotalFlags: len(validFlags)}
	entries := make([]UseIndexEntry, 0, len(validFlags))
	bucketsMap := make(map[string][]UseIndexEntry)
	bucketLabels := make(map[string]string)

	for _, f := range validFlags {
		scope := classifyUseMetadataScope(f)
		switch scope {
		case "use.desc":
			summary.GlobalDescCount++
		case "use_expand":
			summary.UseExpandCount++
		case "local/metadata":
			summary.LocalDescCount++
		default:
			summary.UndocumentedCount++
		}
		distinctCount := f.DistinctPackageCount
		if distinctCount == 0 && f.Count > 0 {
			distinctCount = f.Count
		}
		instanceCount := f.PackageInstanceCount
		if instanceCount == 0 && distinctCount > 0 {
			instanceCount = distinctCount
		}
		entry := UseIndexEntry{
			N: f.Name,
			D: deterministicBestDesc(f),
			C: distinctCount,
			I: instanceCount,
			S: scope,
		}
		entries = append(entries, entry)
		bk, lbl := useBucketKey(f.Name)
		bucketsMap[bk] = append(bucketsMap[bk], entry)
		bucketLabels[bk] = lbl
	}

	bucketOrder := make([]string, 0, 28)
	for c := byte('a'); c <= 'z'; c++ {
		if len(bucketsMap[string(c)]) > 0 {
			bucketOrder = append(bucketOrder, string(c))
		}
	}
	if len(bucketsMap["0-9"]) > 0 {
		bucketOrder = append(bucketOrder, "0-9")
	}
	if len(bucketsMap["other"]) > 0 {
		bucketOrder = append(bucketOrder, "other")
	}

	bucketLinks := make([]UseBucketLink, 0, len(bucketOrder))
	for _, bk := range bucketOrder {
		bucketLinks = append(bucketLinks, UseBucketLink{
			Bucket: bk,
			Label:  bucketLabels[bk],
			Count:  len(bucketsMap[bk]),
			URL:    "index/" + bk + "/",
		})
	}

	tier := "small"
	if len(validFlags) > cfg.MediumMax {
		tier = "large"
	} else if len(validFlags) >= cfg.SmallMax {
		tier = "medium"
	}

	view := &UseDirectoryView{
		Tier:         tier,
		ScopeTitle:   scopeTitle,
		Summary:      summary,
		Buckets:      bucketLinks,
		IndexJSONURL: "uses_index.json",
	}

	switch tier {
	case "small":
		view.AllFlags = entries
	case "large":
		top := make([]UseIndexEntry, len(entries))
		copy(top, entries)
		sort.Slice(top, func(i, j int) bool {
			if top[i].C != top[j].C {
				return top[i].C > top[j].C
			}
			return top[i].N < top[j].N
		})
		if len(top) > cfg.TopLimit {
			top = top[:cfg.TopLimit]
		}
		view.TopFlags = top
	}

	return view, entries, bucketsMap, validFlags
}

func generateUseDirectoryTree(
	usesRoot string,
	baseDepth int, // 1 for global /uses/, 3 for /repos/<repo>/uses/
	baseBreadcrumbs []g2.Breadcrumb,
	siteTitle string,
	repoScope string,
	tmpl *template.Template,
	flags []*AggUseFlag,
	version string,
	genInfo GenerationInfo,
	cfg UseTierConfig,
) error {
	if err := os.MkdirAll(usesRoot, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", usesRoot, err)
	}

	scopeTitle := "All Overlays"
	if repoScope != "" {
		scopeTitle = repoScope
	}
	view, indexEntries, bucketsMap, validFlags := buildUseDirectoryData(flags, scopeTitle, cfg)

	// Write compact uses_index.json in every scope for fast progressive JS search
	indexBytes, err := json.Marshal(indexEntries)
	if err != nil {
		return fmt.Errorf("marshaling uses_index.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(usesRoot, "uses_index.json"), indexBytes, 0o644); err != nil {
		return fmt.Errorf("writing uses_index.json: %w", err)
	}

	landingTemplate := "uses.html"
	if repoScope != "" {
		landingTemplate = "repo_uses.html"
	}
	landingTitle := "USE Flags"
	if repoScope != "" {
		landingTitle = repoScope + " - USE Flags"
	}

	if err := renderPage(filepath.Join(usesRoot, "index.html"), tmpl, landingTemplate, GenericPageContext{
		Title:        landingTitle,
		BaseURL:      strings.Repeat("../", baseDepth),
		Breadcrumbs:  baseBreadcrumbs,
		UseFlags:     validFlags,
		UseDirectory: view,
		Version:      version,
		GenInfo:      genInfo,
	}); err != nil {
		return fmt.Errorf("rendering USE landing page: %w", err)
	}

	// Render static A-Z / sharded bucket pages for Tier 2 and Tier 3 (plus whenever buckets exist in medium/large)
	if view.Tier == "medium" || view.Tier == "large" {
		for _, bl := range view.Buckets {
			items := bucketsMap[bl.Bucket]
			totalPages := (len(items) + cfg.PageSize - 1) / cfg.PageSize
			if totalPages < 1 {
				totalPages = 1
			}
			bucketDir := filepath.Join(usesRoot, "index", bl.Bucket)
			for pageNum := 1; pageNum <= totalPages; pageNum++ {
				start := (pageNum - 1) * cfg.PageSize
				end := start + cfg.PageSize
				if end > len(items) {
					end = len(items)
				}

				pageDir := bucketDir
				pageDepth := baseDepth + 2
				flagPrefix := "../../"
				landingURL := "../../"
				indexJSONURL := "../../uses_index.json"
				if pageNum > 1 {
					pageDir = filepath.Join(bucketDir, fmt.Sprintf("page-%d", pageNum))
					pageDepth = baseDepth + 3
					flagPrefix = "../../../"
					landingURL = "../../../"
					indexJSONURL = "../../../uses_index.json"
				}
				if err := os.MkdirAll(pageDir, 0o755); err != nil {
					return fmt.Errorf("creating directory %s: %w", pageDir, err)
				}

				navLinks := buildBucketPageNavLinks(pageNum, totalPages)
				prevURL, nextURL := resolvePrevNext(navLinks, pageNum)

				crumbs := make([]g2.Breadcrumb, 0, len(baseBreadcrumbs)+2)
				for idx, b := range baseBreadcrumbs {
					bc := b
					if idx == len(baseBreadcrumbs)-1 && bc.URL == "" {
						bc.URL = landingURL
					} else if bc.URL != "" {
						extra := "../../"
						if pageNum > 1 {
							extra = "../../../"
						}
						bc.URL = extra + bc.URL
					}
					crumbs = append(crumbs, bc)
				}
				crumbs = append(crumbs, g2.Breadcrumb{Name: "Index: " + bl.Label})

				if err := renderPage(filepath.Join(pageDir, "index.html"), tmpl, "uses_bucket.html", GenericPageContext{
					Title:       fmt.Sprintf("%s - USE Flags (%s)", scopeTitle, bl.Label),
					BaseURL:     strings.Repeat("../", pageDepth),
					Breadcrumbs: crumbs,
					UseBucket: &UseBucketPage{
						Bucket:         bl.Bucket,
						Label:          bl.Label,
						Page:           pageNum,
						TotalPages:     totalPages,
						TotalFlags:     len(items),
						Flags:          items[start:end],
						FlagLinkPrefix: flagPrefix,
						LandingURL:     landingURL,
						IndexJSONURL:   indexJSONURL,
						PrevURL:        prevURL,
						NextURL:        nextURL,
						PageLinks:      navLinks,
					},
					Version: version,
					GenInfo: genInfo,
				}); err != nil {
					return fmt.Errorf("rendering USE bucket page %s: %w", pageDir, err)
				}
			}
		}
	}

	// Render flag detail pages (with pagination when packages >= cfg.PageSize)
	detailTemplate := "use.html"
	if repoScope != "" {
		detailTemplate = "repo_use.html"
	}
	for _, f := range validFlags {
		useDir := filepath.Join(usesRoot, f.Name)
		pkgRows, overlayLinks := buildUseDetailRows(f, repoScope, cfg.PageSize)
		totalPages := (len(pkgRows) + cfg.PageSize - 1) / cfg.PageSize
		if totalPages < 1 {
			totalPages = 1
		}

		for pageNum := 1; pageNum <= totalPages; pageNum++ {
			start := (pageNum - 1) * cfg.PageSize
			end := start + cfg.PageSize
			if end > len(pkgRows) {
				end = len(pkgRows)
			}

			pageDir := useDir
			pageDepth := baseDepth + 1
			if pageNum > 1 {
				pageDir = filepath.Join(useDir, fmt.Sprintf("page-%d", pageNum))
				pageDepth = baseDepth + 2
			}
			if err := os.MkdirAll(pageDir, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", pageDir, err)
			}

			baseURL := strings.Repeat("../", pageDepth)
			// Build per-overlay filter links relative to this page
			scopedOverlayLinks := make([]UseOverlayLink, 0, len(overlayLinks))
			for _, ol := range overlayLinks {
				scopedOverlayLinks = append(scopedOverlayLinks, UseOverlayLink{
					RepoName: ol.RepoName,
					Count:    ol.Count,
					URL:      baseURL + "repos/" + ol.RepoName + "/uses/" + f.Name + "/",
				})
			}

			navLinks := buildDetailPageNavLinks(pageNum, totalPages)
			prevURL, nextURL := resolvePrevNext(navLinks, pageNum)

			crumbs := make([]g2.Breadcrumb, 0, len(baseBreadcrumbs)+2)
			extraPrefix := "../"
			if pageNum > 1 {
				extraPrefix = "../../"
			}
			for idx, b := range baseBreadcrumbs {
				bc := b
				if idx == len(baseBreadcrumbs)-1 && bc.URL == "" {
					bc.URL = extraPrefix
				} else if bc.URL != "" {
					bc.URL = extraPrefix + bc.URL
				}
				crumbs = append(crumbs, bc)
			}
			crumbs = append(crumbs, g2.Breadcrumb{Name: f.Name})

			if err := renderPage(filepath.Join(pageDir, "index.html"), tmpl, detailTemplate, GenericPageContext{
				Title:         "USE Flag: " + f.Name,
				BaseURL:       baseURL,
				Breadcrumbs:   crumbs,
				GlobalUseFlag: f,
				Repo:          &g2.SiteData{RepoName: repoScope},
				UseDetailPage: &UseDetailPageView{
					Flag:          f,
					RepoScope:     repoScope,
					TotalPackages: len(pkgRows),
					Page:          pageNum,
					TotalPages:    totalPages,
					IsPaginated:   totalPages > 1,
					Packages:      pkgRows[start:end],
					OverlayLinks:  scopedOverlayLinks,
					PrevURL:       prevURL,
					NextURL:       nextURL,
					PageLinks:     navLinks,
				},
				Version: version,
				GenInfo: genInfo,
			}); err != nil {
				return fmt.Errorf("rendering USE detail page %s: %w", pageDir, err)
			}
		}
	}

	_ = siteTitle
	return nil
}

func buildUseDetailRows(f *AggUseFlag, repoScope string, pageSize int) ([]UseDetailPackageRow, []UseOverlayLink) {
	repoCounts := make(map[string]int)
	var rows []UseDetailPackageRow

	if len(f.Usages) > 0 {
		rows = make([]UseDetailPackageRow, 0, len(f.Usages))
		for _, u := range f.Usages {
			if u.RepoName != "" {
				repoCounts[u.RepoName]++
			}
			if repoScope != "" && u.RepoName != "" && u.RepoName != repoScope {
				continue
			}
			rName := u.RepoName
			if rName == "" {
				rName = repoScope
			}
			rows = append(rows, UseDetailPackageRow{
				Category:     u.Category,
				Name:         u.Package,
				PkgKey:       u.Category + "/" + u.Package,
				RepoName:     rName,
				DeclaredIUSE: u.DeclaredIUSE,
				RequiredUse:  u.RequiredUse,
				Versions:     u.Versions,
				LocalDesc:    u.LocalDesc,
				MetadataDesc: u.MetadataDesc,
			})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].PkgKey != rows[j].PkgKey {
				return rows[i].PkgKey < rows[j].PkgKey
			}
			return rows[i].RepoName < rows[j].RepoName
		})
	} else {
		rows = make([]UseDetailPackageRow, 0, len(f.Packages))
		for _, pkg := range f.Packages {
			if pkg == nil {
				continue
			}
			pkgKey := pkg.Category + "/" + pkg.Name
			var repos []string
			for r := range pkg.Repos {
				repos = append(repos, r)
				repoCounts[r]++
			}
			sort.Strings(repos)
			firstRepo := repoScope
			if firstRepo == "" && len(repos) > 0 {
				firstRepo = repos[0]
			}
			rows = append(rows, UseDetailPackageRow{
				Category:     pkg.Category,
				Name:         pkg.Name,
				PkgKey:       pkgKey,
				RepoName:     firstRepo,
				LocalDesc:    f.LocalDescs[pkgKey],
				MetadataDesc: f.MetadataDescs[pkgKey],
			})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].PkgKey != rows[j].PkgKey {
				return rows[i].PkgKey < rows[j].PkgKey
			}
			return rows[i].RepoName < rows[j].RepoName
		})
	}

	repoNames := make([]string, 0, len(repoCounts))
	for r := range repoCounts {
		repoNames = append(repoNames, r)
	}
	sort.Strings(repoNames)
	overlayLinks := make([]UseOverlayLink, 0, len(repoNames))
	for _, r := range repoNames {
		overlayLinks = append(overlayLinks, UseOverlayLink{
			RepoName: r,
			Count:    repoCounts[r],
		})
	}
	_ = pageSize
	return rows, overlayLinks
}

func buildBucketPageNavLinks(current, total int) []UsePageNavLink {
	if total <= 1 {
		return nil
	}
	links := make([]UsePageNavLink, 0, total)
	for p := 1; p <= total; p++ {
		var u string
		switch {
		case p == current:
			u = "./"
		case current == 1 && p > 1:
			u = fmt.Sprintf("page-%d/", p)
		case current > 1 && p == 1:
			u = "../"
		default:
			u = fmt.Sprintf("../page-%d/", p)
		}
		links = append(links, UsePageNavLink{
			Page:    p,
			URL:     u,
			Current: p == current,
		})
	}
	return links
}

func buildDetailPageNavLinks(current, total int) []UsePageNavLink {
	return buildBucketPageNavLinks(current, total)
}

func resolvePrevNext(links []UsePageNavLink, current int) (prevURL, nextURL string) {
	for _, l := range links {
		if l.Page == current-1 {
			prevURL = l.URL
		}
		if l.Page == current+1 {
			nextURL = l.URL
		}
	}
	return prevURL, nextURL
}
