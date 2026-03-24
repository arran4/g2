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

type SiteData struct {
	Title      string
	RepoName   string
	RemoteURL  string
	Categories []CategoryData
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

	if err := generateSite(*outDir, siteData); err != nil {
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

func generateSite(outDir string, site *SiteData) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.ParseFS(siteTemplates, "sitegen_templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	var allPackages []PackageData
	licenseMap := make(map[string]*LicenseData)

	for _, cat := range site.Categories {
		for _, pkg := range cat.Packages {
			allPackages = append(allPackages, pkg)
			for _, ver := range pkg.Versions {
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					lic := ver.Ebuild.Vars["LICENSE"]
					if lic != "" {
						if _, ok := licenseMap[lic]; !ok {
							licenseMap[lic] = &LicenseData{Name: lic}
						}
						// simple deduplication
						found := false
						for _, p := range licenseMap[lic].Packages {
							if p.Name == pkg.Name && p.Category == pkg.Category {
								found = true
								break
							}
						}
						if !found {
							licenseMap[lic].Packages = append(licenseMap[lic].Packages, pkg)
							licenseMap[lic].Count++
						}
					}
				}
			}
		}
	}

	sort.Slice(allPackages, func(i, j int) bool {
		if allPackages[i].Category == allPackages[j].Category {
			return allPackages[i].Name < allPackages[j].Name
		}
		return allPackages[i].Category < allPackages[j].Category
	})

	var sortedLicenses []*LicenseData
	for _, ld := range licenseMap {
		sortedLicenses = append(sortedLicenses, ld)
	}
	sort.Slice(sortedLicenses, func(i, j int) bool {
		return sortedLicenses[i].Name < sortedLicenses[j].Name
	})

	// Generate Feeds for Repo
	var repoFeedItems []FeedItem
	for _, pkg := range allPackages {
		for _, ver := range pkg.Versions {
			desc := ""
			if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
				desc = ver.Ebuild.Vars["DESCRIPTION"]
			}
			repoFeedItems = append(repoFeedItems, FeedItem{
				Title:       fmt.Sprintf("%s/%s-%s", pkg.Category, pkg.Name, ver.Version),
				Link:        fmt.Sprintf("packages/%s/", pkg.Name),
				Description: desc,
				PubDate:     time.Now().Format(time.RFC1123Z),
				Updated:     time.Now().Format(time.RFC3339),
			})
		}
	}
	if len(repoFeedItems) > 50 {
		repoFeedItems = repoFeedItems[:50]
	}
	if err := generateFeeds(filepath.Join(outDir, "index"), site.Title, "Latest updates to repository", "", repoFeedItems); err != nil {
		log.Printf("Warning: failed to generate repo feed: %v", err)
	}

	// Generate index
	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "index.html", map[string]interface{}{
		"Title":       site.Title,
		"BaseURL":     "",
		"RepoName":    site.RepoName,
		"RemoteURL":   site.RemoteURL,
		"Categories":  site.Categories,
		"AllPackages": allPackages,
		"Licenses":    sortedLicenses,
		"Version":     version,
	}); err != nil {
		return err
	}

	// Generate Categories
	for _, cat := range site.Categories {
		catDir := filepath.Join(outDir, "categories", cat.Name)
		if err := os.MkdirAll(catDir, 0755); err != nil {
			return err
		}
		breadcrumbs := []Breadcrumb{
			{Name: site.Title, URL: "../../"},
			{Name: "Categories"},
			{Name: cat.Name},
		}

		if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category.html", map[string]interface{}{
			"Title":       fmt.Sprintf("%s - %s", site.Title, cat.Name),
			"BaseURL":     "../../",
			"Breadcrumbs": breadcrumbs,
			"Category":    cat,
			"Version":     version,
		}); err != nil {
			return err
		}
	}

	// Generate Packages
	for _, pkg := range allPackages {
		pkgDir := filepath.Join(outDir, "packages", pkg.Name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return err
		}
		breadcrumbs := []Breadcrumb{
			{Name: site.Title, URL: "../../"},
			{Name: "Categories", URL: "../../categories/" + pkg.Category + "/"},
			{Name: pkg.Category},
			{Name: pkg.Name},
		}

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

		if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "package.html", map[string]interface{}{
			"Title":       fmt.Sprintf("%s - %s/%s", site.Title, pkg.Category, pkg.Name),
			"BaseURL":     "../../",
			"Breadcrumbs": breadcrumbs,
			"Package":     pkg,
			"Version":     version,
		}); err != nil {
			return err
		}
	}

	// Generate Licenses
	for _, lic := range sortedLicenses {
		licDir := filepath.Join(outDir, "licenses", lic.Name, "packages")
		if err := os.MkdirAll(licDir, 0755); err != nil {
			return err
		}
		breadcrumbs := []Breadcrumb{
			{Name: site.Title, URL: "../../../"},
			{Name: "Licenses"},
			{Name: lic.Name},
		}

		if err := renderPage(filepath.Join(licDir, "index.html"), tmpl, "license.html", map[string]interface{}{
			"Title":       fmt.Sprintf("%s - License: %s", site.Title, lic.Name),
			"BaseURL":     "../../../",
			"Breadcrumbs": breadcrumbs,
			"License":     lic,
			"Version":     version,
		}); err != nil {
			return err
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

	overallSiteData := &SiteData{
		Title: "Remote Gentoo Repositories",
	}

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

		// Since user requested a "list of overlays" we should put each repo in a subfolder
		// generateSite will write into outDir/repo.Name
		repoOutDir := filepath.Join(outDir, repo.Name)
		log.Printf("Generating site for repo: %s", repo.Name)
		if err := generateSite(repoOutDir, siteData); err != nil {
			log.Printf("Failed to generate site for repo %s: %v", repo.Name, err)
		}
		// Add as a root category simply so we can create an index.html listing the repos
		overallSiteData.Categories = append(overallSiteData.Categories, CategoryData{
			Name: repo.Name,
		})
	}

	// Sort categories (which are now repos in this context)
	sort.Slice(overallSiteData.Categories, func(i, j int) bool {
		return overallSiteData.Categories[i].Name < overallSiteData.Categories[j].Name
	})

	// Generate the root index.html listing the overlays
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.ParseFS(siteTemplates, "sitegen_templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "index.html", map[string]interface{}{
		"Title":      overallSiteData.Title,
		"Categories": overallSiteData.Categories,
		"Version":    version,
	}); err != nil {
		return err
	}

	log.Println("Remote site generation complete.")
	return nil
}
