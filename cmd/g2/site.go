package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RemoteRepositories struct {
	XMLName xml.Name     `xml:"repositories"`
	Repos   []RemoteRepo `xml:"repo"`
}

type RemoteRepo struct {
	Name    string       `xml:"name"`
	Sources []RepoSource `xml:"source"`
}

type RepoSource struct {
	Type string `xml:"type,attr"`
	URL  string `xml:",chardata"`
}

type NewsItem struct {
	Title    string
	Author   string
	Posted   time.Time
	Revision string
	Body     string
	DirName  string
	FileName string
}

type SiteData struct {
	Title      string
	RepoName   string
	RemoteURL  string
	Categories []CategoryData
	News       []NewsItem
}

type LicenseData struct {
	Name     string
	Count    int
	Packages []PackageData
}

type Breadcrumb struct {
	Name string
	URL  string
}

type CategoryData struct {
	Name     string
	Packages []PackageData
}

type FileData struct {
	Name   string
	Path   string
	RawURL string
}

type ManifestEntryData struct {
	Entry    *g2.ManifestEntry
	Versions []string
	URLs     []string
}

type PackageData struct {
	Name         string
	Category     string
	Versions     []VersionData
	Metadata     *g2.PkgMetadata
	Manifest     *g2.Manifest
	ManifestData []ManifestEntryData
	Files        []FileData

	// Git info
	MetadataRawURL string

	// Lint Info
	LintWarnings []string
}

type VersionData struct {
	Version string
	Ebuild  *g2.Ebuild

	// Git info
	EbuildRawURL string
}

func (cfg *MainArgConfig) cmdOverlay(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing subcommand for overlay (e.g., site)")
	}
	subcmd := args[0]
	if subcmd != "site" {
		return fmt.Errorf("unknown overlay subcommand: %s", subcmd)
	}

	if len(args) < 2 {
		return fmt.Errorf("missing subcommand for overlay site (e.g., generate)")
	}
	subcmd2 := args[1]
	if subcmd2 != "generate" {
		return fmt.Errorf("unknown overlay site subcommand: %s", subcmd2)
	}

	fs := flag.NewFlagSet("overlay site generate", flag.ExitOnError)
	outDir := fs.String("out", "site_out", "Output directory for the generated site")
	clear := fs.Bool("clear", false, "Clear output directory before generation")

	if err := fs.Parse(args[2:]); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	if *clear {
		if err := os.RemoveAll(*outDir); err != nil {
			return fmt.Errorf("clearing output directory: %w", err)
		}
	}

	log.Printf("Generating site from overlay location %s into %s", location, *outDir)

	// if location is a url, clone it temporarily
	isRemote := strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") || strings.HasPrefix(location, "git://")
	var parseLocation string
	var cleanup func()

	if isRemote {
		tmpDir, err := os.MkdirTemp("", "g2-overlay-*")
		if err != nil {
			return fmt.Errorf("creating temp dir: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(tmpDir) }

		log.Printf("Cloning remote repository: %s", location)
		cmd := exec.Command("git", "clone", "--depth", "1", location, tmpDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			cleanup()
			return fmt.Errorf("cloning repository: %w", err)
		}
		parseLocation = tmpDir
	} else {
		parseLocation = location
		cleanup = func() {}
	}
	defer cleanup()

	siteData, err := parseRepo(parseLocation, "Gentoo Packages")
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	if err := generateSite(*outDir, []*SiteData{siteData}); err != nil {
		return fmt.Errorf("generating site: %w", err)
	}

	log.Println("Site generation complete.")
	return nil
}

func (cfg *MainArgConfig) cmdOverlays(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing subcommand for overlays (e.g., site)")
	}
	subcmd := args[0]
	if subcmd != "site" {
		return fmt.Errorf("unknown overlays subcommand: %s", subcmd)
	}

	if len(args) < 2 {
		return fmt.Errorf("missing subcommand for overlays site (e.g., generate)")
	}
	subcmd2 := args[1]
	if subcmd2 != "generate" {
		return fmt.Errorf("unknown overlays site subcommand: %s", subcmd2)
	}

	fs := flag.NewFlagSet("overlays site generate", flag.ExitOnError)
	outDir := fs.String("out", "site_out", "Output directory for the generated site")
	clear := fs.Bool("clear", false, "Clear output directory before generation")

	if err := fs.Parse(args[2:]); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing location argument (url, file path, or - for stdin)")
	}
	location := fs.Arg(0)

	if *clear {
		if err := os.RemoveAll(*outDir); err != nil {
			return fmt.Errorf("clearing output directory: %w", err)
		}
	}

	log.Printf("Generating site from remote repositories: %s into %s", location, *outDir)
	return cfg.cmdSiteRemote(location, *outDir)
}

func parseRepo(repoDir string, defaultTitle string) (*SiteData, error) {
	title := defaultTitle
	var repoName string

	// Get Git Info
	remoteURL, err := getGitOriginURL(repoDir)
	if err != nil {
		log.Printf("Warning: failed to get git origin url: %v", err)
	}

	repoNameBytes, err := os.ReadFile(filepath.Join(repoDir, "profiles", "repo_name"))
	if err == nil && len(repoNameBytes) > 0 {
		title = strings.TrimSpace(string(repoNameBytes))
		repoName = title
	} else {
		repoName = filepath.Base(repoDir)
	}

	site := &SiteData{
		Title:     title,
		RepoName:  repoName,
		RemoteURL: remoteURL,
	}

	// Parse News
	newsDir := filepath.Join(repoDir, "metadata", "news")
	if entries, err := os.ReadDir(newsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dirName := entry.Name()
			txtFile := filepath.Join(newsDir, dirName, dirName+".en.txt")

			content, err := os.ReadFile(txtFile)
			if err != nil {
				continue
			}

			lines := strings.Split(string(content), "\n")
			var item NewsItem
			item.DirName = dirName
			item.FileName = dirName + ".en.txt"

			inBody := false
			var bodyLines []string

			for _, line := range lines {
				if inBody {
					bodyLines = append(bodyLines, line)
					continue
				}

				if strings.TrimSpace(line) == "" {
					inBody = true
					continue
				}

				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}

				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])

				switch key {
				case "Title":
					item.Title = val
				case "Author":
					item.Author = val
				case "Posted":
					t, err := time.Parse("2006-01-02", val)
					if err == nil {
						item.Posted = t
					}
				case "Revision":
					item.Revision = val
				}
			}

			item.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
			site.News = append(site.News, item)
		}

		// Sort news descending by posted date
		sort.Slice(site.News, func(i, j int) bool {
			return site.News[i].Posted.After(site.News[j].Posted)
		})
	}

	supportedCategories := make(map[string]bool)
	categoriesBytes, err := os.ReadFile(filepath.Join(repoDir, "profiles", "categories"))
	if err == nil {
		lines := strings.Split(string(categoriesBytes), "\n")
		for _, line := range lines {
			cat := strings.TrimSpace(line)
			if cat != "" && !strings.HasPrefix(cat, "#") {
				supportedCategories[cat] = true
			}
		}
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, fmt.Errorf("reading repo dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isIgnoredDir(name) {
			continue
		}

		if len(supportedCategories) > 0 && !supportedCategories[name] {
			continue
		}

		catData := CategoryData{Name: name}
		catPath := filepath.Join(repoDir, name)

		pkgEntries, err := os.ReadDir(catPath)
		if err != nil {
			log.Printf("Warning: reading category dir %s: %v", catPath, err)
			continue
		}

		for _, pkgEntry := range pkgEntries {
			if !pkgEntry.IsDir() {
				continue
			}
			pkgName := pkgEntry.Name()
			if strings.HasPrefix(pkgName, ".") {
				continue
			}

			pkgPath := filepath.Join(catPath, pkgName)
			pkgData := PackageData{
				Name:     pkgName,
				Category: name,
			}

			// Read ebuilds
			files, err := os.ReadDir(pkgPath)
			if err != nil {
				log.Printf("Warning: reading package dir %s: %v", pkgPath, err)
				continue
			}

			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".ebuild") {
					continue
				}

				ebuildPath := filepath.Join(pkgPath, file.Name())
				// Use DirFS for ParseEbuild
				ebuild, err := g2.ParseEbuild(os.DirFS(filepath.Dir(ebuildPath)), file.Name(), g2.ParseFull)
				if err != nil {
					log.Printf("Warning: parsing ebuild %s: %v", ebuildPath, err)
					continue
				}

				version := ""
				if ebuild.Vars != nil {
					version = ebuild.Vars["PV"]
				}
				if version == "" {
					// Fallback from filename
					vars := g2.ParseEbuildVariables(file.Name())
					if vars != nil {
						version = vars["PV"]
					}
				}

				var ebuildRawURL string
				if remoteURL != "" {
					relPath, _ := filepath.Rel(repoDir, ebuildPath)
					commitHash, _ := getFileCommit(repoDir, relPath)
					if commitHash != "" {
						ebuildRawURL = generateGitHubRawURL(remoteURL, commitHash, relPath)
					}
				}

				pkgData.Versions = append(pkgData.Versions, VersionData{
					Version:      version,
					Ebuild:       ebuild,
					EbuildRawURL: ebuildRawURL,
				})
			}

			if len(pkgData.Versions) == 0 {
				continue // No ebuilds, skip package
			}

			// Sort versions descending
			sort.Slice(pkgData.Versions, func(i, j int) bool {
				return pkgData.Versions[i].Version > pkgData.Versions[j].Version
			})

			// Read metadata.xml
			metaPath := filepath.Join(pkgPath, "metadata.xml")
			metadata, err := g2.ParseMetadata(metaPath)
			if err == nil {
				if pkgMd, ok := metadata.(*g2.PkgMetadata); ok {
					pkgData.Metadata = pkgMd
				}
			}

			if remoteURL != "" {
				relPath, _ := filepath.Rel(repoDir, metaPath)
				commitHash, _ := getFileCommit(repoDir, relPath)
				if commitHash != "" {
					pkgData.MetadataRawURL = generateGitHubRawURL(remoteURL, commitHash, relPath)
				}
			}

			// Read Manifest
			manifestPath := filepath.Join(pkgPath, "Manifest")
			manifest, err := g2.ParseManifest(manifestPath)
			if err == nil {
				pkgData.Manifest = manifest
				pkgData.ManifestData = buildManifestData(manifest, pkgData.Versions)
			}

			// Read files/ directory
			filesDirPath := filepath.Join(pkgPath, "files")
			if info, err := os.Stat(filesDirPath); err == nil && info.IsDir() {
				fileEntries, err := os.ReadDir(filesDirPath)
				if err == nil {
					for _, fe := range fileEntries {
						if !fe.IsDir() {
							fd := FileData{
								Name: fe.Name(),
								Path: filepath.Join(filesDirPath, fe.Name()),
							}
							if remoteURL != "" {
								relPath, _ := filepath.Rel(repoDir, fd.Path)
								commitHash, _ := getFileCommit(repoDir, relPath)
								if commitHash != "" {
									fd.RawURL = generateGitHubRawURL(remoteURL, commitHash, relPath)
								}
							}
							pkgData.Files = append(pkgData.Files, fd)
						}
					}
				}
			}

			pkgData.LintWarnings = performLinting(repoDir, pkgData)

			catData.Packages = append(catData.Packages, pkgData)
		}

		if len(catData.Packages) > 0 {
			// Sort packages by name
			sort.Slice(catData.Packages, func(i, j int) bool {
				return catData.Packages[i].Name < catData.Packages[j].Name
			})
			site.Categories = append(site.Categories, catData)
		}
	}

	// Sort categories by name
	sort.Slice(site.Categories, func(i, j int) bool {
		return site.Categories[i].Name < site.Categories[j].Name
	})

	return site, nil
}

func buildManifestData(manifest *g2.Manifest, versions []VersionData) []ManifestEntryData {
	var manifestData []ManifestEntryData
	for _, entry := range manifest.Entries {
		md := ManifestEntryData{
			Entry: entry,
		}

		urlMap := make(map[string]bool)
		versionMap := make(map[string]bool)

		for _, ver := range versions {
			if ver.Ebuild == nil {
				continue
			}
			for _, uri := range ver.Ebuild.SrcUri {
				fname := uri.Filename
				if fname == "" {
					fname = filepath.Base(uri.URL)
				}
				if fname == entry.Filename {
					verStr := ver.Version
					if ver.Ebuild.Vars != nil && ver.Ebuild.Vars["PV"] != "" {
						verStr = ver.Ebuild.Vars["PV"]
					}
					if !versionMap[verStr] {
						md.Versions = append(md.Versions, verStr)
						versionMap[verStr] = true
					}
					if !urlMap[uri.URL] {
						md.URLs = append(md.URLs, uri.URL)
						urlMap[uri.URL] = true
					}
				}
			}
		}
		// Sort versions descending
		sort.Slice(md.Versions, func(i, j int) bool {
			return md.Versions[i] > md.Versions[j]
		})
		sort.Strings(md.URLs)

		manifestData = append(manifestData, md)
	}
	return manifestData
}

func isIgnoredDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	ignored := map[string]bool{
		"profiles":  true,
		"metadata":  true,
		"eclass":    true,
		"scripts":   true,
		"distfiles": true,
		"packages":  true,
		"licenses":  true,
	}
	return ignored[name]
}


type AggCategory struct {
	Name     string
	Packages map[string]*AggPackage
}
type AggPackage struct {
	Name     string
	Category string
	Repos    map[string]*SiteData
}
type AggLicense struct {
	Name     string
	Count    int
	Packages []*AggPackage
	Text     string
}

type AggNewsItem struct {
	NewsItem
	RepoName string
}

func generateSite(outDir string, sites []*SiteData) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.ParseFS(siteTemplates, "sitegen_templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	aggCategories := make(map[string]*AggCategory)
	aggPackages := make(map[string]*AggPackage)
	aggLicenses := make(map[string]*AggLicense)
	var globalNews []AggNewsItem

	totalPackages := 0

	for _, site := range sites {
		for _, news := range site.News {
			globalNews = append(globalNews, AggNewsItem{
				NewsItem: news,
				RepoName: site.RepoName,
			})
		}

		for _, cat := range site.Categories {
			if _, ok := aggCategories[cat.Name]; !ok {
				aggCategories[cat.Name] = &AggCategory{Name: cat.Name, Packages: make(map[string]*AggPackage)}
			}
			for _, pkg := range cat.Packages {
				pkgKey := cat.Name + "/" + pkg.Name
				if _, ok := aggPackages[pkgKey]; !ok {
					aggPackages[pkgKey] = &AggPackage{Name: pkg.Name, Category: cat.Name, Repos: make(map[string]*SiteData)}
					totalPackages++
				}
				aggPackages[pkgKey].Repos[site.RepoName] = site
				aggCategories[cat.Name].Packages[pkg.Name] = aggPackages[pkgKey]

				for _, ver := range pkg.Versions {
					if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
						lic := ver.Ebuild.Vars["LICENSE"]
						if lic != "" {
							if _, ok := aggLicenses[lic]; !ok {
								aggLicenses[lic] = &AggLicense{Name: lic}
							}

							found := false
							for _, p := range aggLicenses[lic].Packages {
								if p.Name == pkg.Name && p.Category == pkg.Category {
									found = true
									break
								}
							}
							if !found {
								aggLicenses[lic].Packages = append(aggLicenses[lic].Packages, aggPackages[pkgKey])
								aggLicenses[lic].Count++
							}
						}
					}
				}
			}
		}
	}

	// Sort structures for templates
	var sortedCategories []*AggCategory
	for _, c := range aggCategories {
		sortedCategories = append(sortedCategories, c)
	}
	sort.Slice(sortedCategories, func(i, j int) bool { return sortedCategories[i].Name < sortedCategories[j].Name })

	var sortedPackages []*AggPackage
	for _, p := range aggPackages {
		sortedPackages = append(sortedPackages, p)
	}
	sort.Slice(sortedPackages, func(i, j int) bool {
		if sortedPackages[i].Category == sortedPackages[j].Category {
			return sortedPackages[i].Name < sortedPackages[j].Name
		}
		return sortedPackages[i].Category < sortedPackages[j].Category
	})

	var sortedLicenses []*AggLicense
	for _, l := range aggLicenses {
		sortedLicenses = append(sortedLicenses, l)
	}
	sort.Slice(sortedLicenses, func(i, j int) bool { return sortedLicenses[i].Name < sortedLicenses[j].Name })

	sort.Slice(globalNews, func(i, j int) bool {
		return globalNews[i].Posted.After(globalNews[j].Posted)
	})

	var recentNews []AggNewsItem
	cutoffDate := time.Now().AddDate(0, -3, 0)
	for _, n := range globalNews {
		if n.Posted.After(cutoffDate) {
			recentNews = append(recentNews, n)
		} else {
			break
		}
	}
	if len(recentNews) == 0 && len(globalNews) > 0 {
		// fallback if no news in last 3 months, show the last 3 items
		for i := 0; i < len(globalNews) && i < 3; i++ {
			recentNews = append(recentNews, globalNews[i])
		}
	}

	mapToList := func(m map[string]*SiteData) []*SiteData {
		var l []*SiteData
		for _, v := range m {
			l = append(l, v)
		}
		sort.Slice(l, func(i, j int) bool { return l[i].RepoName < l[j].RepoName })
		return l
	}

	// Title
	title := "Gentoo Packages"
	if len(sites) == 1 {
		title = sites[0].Title
	}

	// Generate Global Feeds
	var globalFeedItems []FeedItem
	for _, pkg := range sortedPackages {
		for _, site := range pkg.Repos {
			var sPkg *PackageData
			for _, cat := range site.Categories {
				if cat.Name == pkg.Category {
					for _, p := range cat.Packages {
						if p.Name == pkg.Name {
							sPkg = &p
							break
						}
					}
				}
			}
			if sPkg != nil {
				for _, ver := range sPkg.Versions {
					desc := ""
					if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
						desc = ver.Ebuild.Vars["DESCRIPTION"]
					}
					globalFeedItems = append(globalFeedItems, FeedItem{
						Title:       fmt.Sprintf("%s/%s-%s (%s)", sPkg.Category, sPkg.Name, ver.Version, site.RepoName),
						Link:        fmt.Sprintf("repos/%s/categories/%s/packages/%s/", site.RepoName, sPkg.Category, sPkg.Name),
						Description: desc,
						PubDate:     time.Now().Format(time.RFC1123Z),
						Updated:     time.Now().Format(time.RFC3339),
					})
				}
			}
		}
	}
	if len(globalFeedItems) > 50 {
		globalFeedItems = globalFeedItems[:50]
	}
	if err := generateFeeds(filepath.Join(outDir, "index"), title, "Latest updates to global repository", "", globalFeedItems); err != nil {
		log.Printf("Warning: failed to generate global feed: %v", err)
	}

	// 1. Root Dashboard
	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "dashboard.html", map[string]interface{}{
		"Title":      title,
		"BaseURL":    "",
		"Repos":      sites,
		"Categories": sortedCategories,
		"Packages":   sortedPackages,
		"Licenses":   sortedLicenses,
		"Updates":    globalFeedItems,
		"RecentNews": recentNews,
		"Version":    version,
	}); err != nil { return err }

	// 1b. Global News Dashboard
	if len(globalNews) > 0 {
		if err := os.MkdirAll(filepath.Join(outDir, "news"), 0755); err != nil { return err }
		if err := renderPage(filepath.Join(outDir, "news", "index.html"), tmpl, "news_dashboard.html", map[string]interface{}{
			"Title":       "News Dashboard",
			"BaseURL":     "../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "News"}},
			"RecentNews":  recentNews,
			"Version":     version,
		}); err != nil { return err }

		// Global News Archive
		if err := os.MkdirAll(filepath.Join(outDir, "news", "archive"), 0755); err != nil { return err }
		if err := renderPage(filepath.Join(outDir, "news", "archive", "index.html"), tmpl, "news_archive.html", map[string]interface{}{
			"Title":       "News Archive",
			"BaseURL":     "../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
			"News":        globalNews,
			"Version":     version,
		}); err != nil { return err }

		// Global News Articles
		for _, n := range globalNews {
			newsDir := filepath.Join(outDir, "news", "archive", n.DirName)
			if err := os.MkdirAll(newsDir, 0755); err != nil { return err }
			if err := renderPage(filepath.Join(newsDir, "index.html"), tmpl, "news_article.html", map[string]interface{}{
				"Title":       n.Title,
				"BaseURL":     "../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
				"NewsItem":    n,
				"Version":     version,
			}); err != nil { return err }
		}
	}

	// 2. Overlays List
	if err := os.MkdirAll(filepath.Join(outDir, "overlays"), 0755); err != nil { return err }
	if err := renderPage(filepath.Join(outDir, "overlays", "index.html"), tmpl, "overlays.html", map[string]interface{}{
		"Title":       "Overlays",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Overlays"}},
		"Repos":       sites,
		"Version":     version,
	}); err != nil { return err }

	// 3. Global Categories
	if err := os.MkdirAll(filepath.Join(outDir, "categories"), 0755); err != nil { return err }
	if err := renderPage(filepath.Join(outDir, "categories", "index.html"), tmpl, "categories.html", map[string]interface{}{
		"Title":       "Categories",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Categories"}},
		"Categories":  sortedCategories,
		"Version":     version,
	}); err != nil { return err }

	for _, cat := range sortedCategories {
		catDir := filepath.Join(outDir, "categories", cat.Name)
		if err := os.MkdirAll(catDir, 0755); err != nil { return err }

		var catPkgs []*AggPackage
		for _, p := range cat.Packages {
			catPkgs = append(catPkgs, p)
		}
		sort.Slice(catPkgs, func(i, j int) bool { return catPkgs[i].Name < catPkgs[j].Name })

		type TmplPkg struct {
			Name string
			ReposList []*SiteData
		}
		var tmplPkgs []TmplPkg
		for _, p := range catPkgs {
			tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: mapToList(p.Repos)})
		}

		if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category.html", map[string]interface{}{
			"Title":       "Category: " + cat.Name,
			"BaseURL":     "../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
			"Category":    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
			"Version":     version,
		}); err != nil { return err }
	}

	// 4. Global Packages
	if err := os.MkdirAll(filepath.Join(outDir, "packages"), 0755); err != nil { return err }
	if err := renderPage(filepath.Join(outDir, "packages", "index.html"), tmpl, "packages.html", map[string]interface{}{
		"Title":       "Packages",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Packages"}},
		"Packages":    sortedPackages,
		"Version":     version,
	}); err != nil { return err }

	for _, pkg := range sortedPackages {
		pkgDir := filepath.Join(outDir, "packages", pkg.Category, pkg.Name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil { return err }

		reposList := mapToList(pkg.Repos)

		if len(reposList) == 1 {
			targetURL := fmt.Sprintf("../../../repos/%s/categories/%s/packages/%s/", reposList[0].RepoName, pkg.Category, pkg.Name)
			redirectHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0; url=%s"></head><body><a href="%s">Redirecting...</a></body></html>`, targetURL, targetURL)
			if err := os.WriteFile(filepath.Join(pkgDir, "index.html"), []byte(redirectHTML), 0644); err != nil { return err }
		} else {
			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "package_picker.html", map[string]interface{}{
				"Title":       "Package: " + pkg.Category + "/" + pkg.Name,
				"BaseURL":     "../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Packages", URL: "../../"}, {Name: pkg.Category}, {Name: pkg.Name}},
				"Package":     map[string]interface{}{"Category": pkg.Category, "Name": pkg.Name, "ReposList": reposList},
				"Version":     version,
			}); err != nil { return err }
		}
	}

	// 5. Global Licenses
	if err := os.MkdirAll(filepath.Join(outDir, "licenses"), 0755); err != nil { return err }
	if err := renderPage(filepath.Join(outDir, "licenses", "index.html"), tmpl, "licenses.html", map[string]interface{}{
		"Title":       "Licenses",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Licenses"}},
		"Licenses":    sortedLicenses,
		"Version":     version,
	}); err != nil { return err }

	for _, lic := range sortedLicenses {
		licDir := filepath.Join(outDir, "licenses", lic.Name)
		if err := os.MkdirAll(licDir, 0755); err != nil { return err }

		type TmplPkg struct {
			Name string
			Category string
			ReposList []*SiteData
		}
		var tmplPkgs []TmplPkg
		for _, p := range lic.Packages {
			tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, Category: p.Category, ReposList: mapToList(p.Repos)})
		}

		if err := renderPage(filepath.Join(licDir, "index.html"), tmpl, "license.html", map[string]interface{}{
			"Title":       "License: " + lic.Name,
			"BaseURL":     "../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Licenses", URL: "../"}, {Name: lic.Name}},
			"License":     map[string]interface{}{"Name": lic.Name, "Packages": tmplPkgs, "Text": lic.Text},
			"Version":     version,
		}); err != nil { return err }
	}

	// 6. Repo-Specific Pages
	for _, site := range sites {
		repoDir := filepath.Join(outDir, "repos", site.RepoName)
		if err := os.MkdirAll(repoDir, 0755); err != nil { return err }

		var repoFeedItems []FeedItem
		for _, cat := range site.Categories {
			for _, pkg := range cat.Packages {
				for _, ver := range pkg.Versions {
					desc := ""
					if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
						desc = ver.Ebuild.Vars["DESCRIPTION"]
					}
					repoFeedItems = append(repoFeedItems, FeedItem{
						Title:       fmt.Sprintf("%s/%s-%s", pkg.Category, pkg.Name, ver.Version),
						Link:        fmt.Sprintf("categories/%s/packages/%s/", pkg.Category, pkg.Name),
						Description: desc,
						PubDate:     time.Now().Format(time.RFC1123Z),
						Updated:     time.Now().Format(time.RFC3339),
					})
				}
			}
		}
		if len(repoFeedItems) > 50 {
			repoFeedItems = repoFeedItems[:50]
		}
		if err := generateFeeds(filepath.Join(repoDir, "index"), site.RepoName, "Latest updates to repository", "", repoFeedItems); err != nil {
			log.Printf("Warning: failed to generate repo feed: %v", err)
		}

		pkgCount := 0
		for _, c := range site.Categories { pkgCount += len(c.Packages) }

		var repoRecentNews []NewsItem
		for _, n := range site.News {
			if n.Posted.After(cutoffDate) {
				repoRecentNews = append(repoRecentNews, n)
			} else {
				break
			}
		}
		if len(repoRecentNews) == 0 && len(site.News) > 0 {
			for i := 0; i < len(site.News) && i < 3; i++ {
				repoRecentNews = append(repoRecentNews, site.News[i])
			}
		}

		if err := renderPage(filepath.Join(repoDir, "index.html"), tmpl, "repo_index.html", map[string]interface{}{
			"Title":       site.RepoName,
			"BaseURL":     "../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Overlays", URL: "../../overlays/"}, {Name: site.RepoName}},
			"Repo":        site,
			"PackageCount": pkgCount,
			"Updates":     repoFeedItems,
			"RecentNews":  repoRecentNews,
			"Version":     version,
		}); err != nil { return err }

		// Repo News Dashboard
		if len(site.News) > 0 {
			if err := os.MkdirAll(filepath.Join(repoDir, "news"), 0755); err != nil { return err }
			if err := renderPage(filepath.Join(repoDir, "news", "index.html"), tmpl, "news_dashboard.html", map[string]interface{}{
				"Title":       site.RepoName + " - News Dashboard",
				"BaseURL":     "../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Overlays", URL: "../../../overlays/"}, {Name: site.RepoName, URL: "../"}, {Name: "News"}},
				"RecentNews":  repoRecentNews,
				"Version":     version,
			}); err != nil { return err }

			// Repo News Archive
			if err := os.MkdirAll(filepath.Join(repoDir, "news", "archive"), 0755); err != nil { return err }
			if err := renderPage(filepath.Join(repoDir, "news", "archive", "index.html"), tmpl, "news_archive.html", map[string]interface{}{
				"Title":       site.RepoName + " - News Archive",
				"BaseURL":     "../../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: "Overlays", URL: "../../../../overlays/"}, {Name: site.RepoName, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
				"News":        site.News,
				"Version":     version,
			}); err != nil { return err }

			// Repo News Articles
			for _, n := range site.News {
				newsDir := filepath.Join(repoDir, "news", "archive", n.DirName)
				if err := os.MkdirAll(newsDir, 0755); err != nil { return err }
				if err := renderPage(filepath.Join(newsDir, "index.html"), tmpl, "news_article.html", map[string]interface{}{
					"Title":       n.Title,
					"BaseURL":     "../../../../../",
					"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../../"}, {Name: "Overlays", URL: "../../../../../overlays/"}, {Name: site.RepoName, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
					"NewsItem":    n,
					"Version":     version,
				}); err != nil { return err }
			}
		}

		if err := os.MkdirAll(filepath.Join(repoDir, "categories"), 0755); err != nil { return err }
		if err := renderPage(filepath.Join(repoDir, "categories", "index.html"), tmpl, "categories.html", map[string]interface{}{
			"Title":       site.RepoName + " - Categories",
			"BaseURL":     "../../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Categories"}},
			"Categories":  site.Categories,
			"Version":     version,
		}); err != nil { return err }

		for _, cat := range site.Categories {
			catDir := filepath.Join(repoDir, "categories", cat.Name)
			if err := os.MkdirAll(catDir, 0755); err != nil { return err }

			type TmplPkg struct {
				Name string
				ReposList []*SiteData
			}
			var tmplPkgs []TmplPkg
			for _, p := range cat.Packages {
				tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: []*SiteData{site}})
			}

			if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category.html", map[string]interface{}{
				"Title":       "Category: " + cat.Name,
				"BaseURL":     "../../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
				"Category":    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
				"Version":     version,
			}); err != nil { return err }
		}

		if err := os.MkdirAll(filepath.Join(repoDir, "packages"), 0755); err != nil { return err }
		var repoPkgs []PackageData
		for _, c := range site.Categories { repoPkgs = append(repoPkgs, c.Packages...) }
		sort.Slice(repoPkgs, func(i, j int) bool {
			if repoPkgs[i].Category == repoPkgs[j].Category { return repoPkgs[i].Name < repoPkgs[j].Name }
			return repoPkgs[i].Category < repoPkgs[j].Category
		})

		if err := renderPage(filepath.Join(repoDir, "packages", "index.html"), tmpl, "repo_packages.html", map[string]interface{}{
			"Title":       site.RepoName + " - Packages",
			"BaseURL":     "../../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Packages"}},
			"Packages":    repoPkgs,
			"Repo":        site,
			"Version":     version,
		}); err != nil { return err }

		for _, pkg := range repoPkgs {
			pkgDir := filepath.Join(repoDir, "categories", pkg.Category, "packages", pkg.Name)
			if err := os.MkdirAll(pkgDir, 0755); err != nil { return err }

			var pkgFeedItems []FeedItem
			for _, ver := range pkg.Versions {
				desc := ""
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					desc = ver.Ebuild.Vars["DESCRIPTION"]
				}
				pkgFeedItems = append(pkgFeedItems, FeedItem{
					Title:       fmt.Sprintf("%s/%s-%s", pkg.Category, pkg.Name, ver.Version),
					Link:        "",
					Description: desc,
					PubDate:     time.Now().Format(time.RFC1123Z),
					Updated:     time.Now().Format(time.RFC3339),
				})
			}
			if err := generateFeeds(filepath.Join(pkgDir, "index"), pkg.Category+"/"+pkg.Name, "Latest updates to package", "", pkgFeedItems); err != nil {
				log.Printf("Warning: failed to generate package feed: %v", err)
			}

			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "repo_package.html", map[string]interface{}{
				"Title":       fmt.Sprintf("%s - %s/%s", site.RepoName, pkg.Category, pkg.Name),
				"BaseURL":     "../../../../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../../../"}, {Name: site.RepoName, URL: "../../../../"}, {Name: "Categories", URL: "../../../"}, {Name: pkg.Category}, {Name: pkg.Name}},
				"Repo":        site,
				"Package":     pkg,
				"Version":     version,
			}); err != nil { return err }
		}
	}

	return nil
}
func renderPage(path string, tmpl *template.Template, name string, data map[string]interface{}) error {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}

	data["Content"] = template.HTML(buf.String())

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return tmpl.ExecuteTemplate(f, "layout.html", data)
}

func (cfg *MainArgConfig) cmdSiteRemote(repositoriesFile string, outDir string) error {
	var data []byte
	var err error

	if repositoriesFile == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading repositories.xml from stdin: %w", err)
		}
	} else if strings.HasPrefix(repositoriesFile, "http://") || strings.HasPrefix(repositoriesFile, "https://") {
		resp, err := http.Get(repositoriesFile)
		if err != nil {
			return fmt.Errorf("fetching repositories.xml: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading repositories.xml: %w", err)
		}
	} else {
		data, err = os.ReadFile(repositoriesFile)
		if err != nil {
			return fmt.Errorf("reading repositories.xml file: %w", err)
		}
	}

	var repos RemoteRepositories
	if err := xml.Unmarshal(data, &repos); err != nil {
		return fmt.Errorf("parsing repositories.xml: %w", err)
	}

	// Create a temporary directory to clone repos into
	tmpDir, err := os.MkdirTemp("", "g2-sitegen-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var allSites []*SiteData

	for _, repo := range repos.Repos {
		if len(repo.Sources) == 0 {
			continue
		}

		var gitUrl string
		for _, src := range repo.Sources {
			if src.Type == "git" && strings.HasPrefix(src.URL, "http") {
				gitUrl = src.URL
				break
			}
		}

		if gitUrl == "" {
			continue // skip non-http git repos for this tool
		}

		log.Printf("Cloning remote repository: %s (%s)", repo.Name, gitUrl)

		repoPath := filepath.Join(tmpDir, repo.Name)
		// Try to shallow clone
		cmd := exec.Command("git", "clone", "--depth", "1", gitUrl, repoPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to clone %s: %v", repo.Name, err)
			continue
		}

		log.Printf("Parsing repository: %s", repo.Name)
		siteData, err := parseRepo(repoPath, repo.Name)
		if err != nil {
			log.Printf("Failed to parse repo %s: %v", repo.Name, err)
			continue
		}

		allSites = append(allSites, siteData)
	}

	log.Printf("Generating site for %d repositories", len(allSites))
	if err := generateSite(outDir, allSites); err != nil {
		return fmt.Errorf("generating multi-repo site: %w", err)
	}

	log.Println("Remote site generation complete.")
	return nil
}
