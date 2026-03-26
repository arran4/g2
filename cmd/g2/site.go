package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TODO evaluate the following they should be redundant OR moved to `/`

var (
	mainGentooCategories map[string]bool
	mainGentooOnce       sync.Once
)

func fetchMainGentooCategories() map[string]bool {
	mainGentooOnce.Do(func() {
		mainGentooCategories = make(map[string]bool)
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("https://raw.githubusercontent.com/gentoo-mirror/gentoo/stable/profiles/categories")
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				if err == nil {
					lines := strings.Split(string(data), "\n")
					for _, line := range lines {
						cat := strings.TrimSpace(line)
						if cat != "" && !strings.HasPrefix(cat, "#") {
							mainGentooCategories[cat] = true
						}
					}
				}
			}
		} else {
			log.Printf("Warning: failed to fetch main gentoo categories: %v", err)
		}
	})
	return mainGentooCategories
}

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

type ProfileDescEntry struct {
	Arch   string
	Path   string
	Status string
}

type ProfileData struct {
	Path     string
	IsDesc   bool
	DescArch string
	DescStat string
	Parents  []string
	Children []string
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
	Title          string
	RepoName       string
	RemoteURL      string
	EAPI           string
	Categories     []CategoryData
	Profiles       []ProfileData
	Authors        []g2.Author
	AuthorsURL     string
	Moves          []g2.PackageMove
	News           []NewsItem
	LayoutConf     *g2.LayoutConf
	LicenseMapping map[string][]string
	QAPolicy       *g2.QAPolicy
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
	Name          string
	Category      string
	Versions      []VersionData
	Metadata      *g2.PkgMetadata
	MetadataError error
	Manifest      *g2.Manifest
	ManifestData  []ManifestEntryData
	Files         []FileData

	// Git info
	MetadataRawURL string
	ModTime        time.Time

	// Lint Info
	LintWarnings []string
}

type VersionData struct {
	Version string
	Ebuild  *g2.Ebuild

	// Git info
	EbuildRawURL string
	ModTime      time.Time
}

// End model TODO check

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
	recentDurOpt := fs.String("recent-duration", "3mo", "Duration to consider an update 'recent' (e.g. 3mo, 14d, 72h)")
	fastGit := fs.Bool("fast-git-modtime", false, "Use fast (O(1)) but potentially less reliable go-git file log lookup")

	if err := fs.Parse(args[2:]); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	recentDuration, recentDurationStr, err := parseDuration(*recentDurOpt)
	if err != nil {
		return fmt.Errorf("invalid recent-duration: %w", err)
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

	siteData, err := parseRepo(os.DirFS(parseLocation), ".", "Gentoo Packages", *fastGit)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	if err := generateSite(*outDir, []*SiteData{siteData}, recentDuration, recentDurationStr); err != nil {
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
	recentDurOpt := fs.String("recent-duration", "3mo", "Duration to consider an update 'recent' (e.g. 3mo, 14d, 72h)")
	fastGit := fs.Bool("fast-git-modtime", false, "Use fast (O(1)) but potentially less reliable go-git file log lookup")

	if err := fs.Parse(args[2:]); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	recentDuration, recentDurationStr, err := parseDuration(*recentDurOpt)
	if err != nil {
		return fmt.Errorf("invalid recent-duration: %w", err)
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
	return cfg.cmdSiteRemote(location, *outDir, recentDuration, recentDurationStr, *fastGit)
}

func parseLayoutConfFromFS(sysFS fs.FS, path string) (*g2.LayoutConf, error) {
	file, err := sysFS.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return g2.ParseLayoutConfFromReader(file)
}

func parseMetadataFromFS(sysFS fs.FS, path string) (interface{}, error) {
	file, err := sysFS.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return g2.ParseMetadataFromReader(file)
}

func parseManifestFromFS(sysFS fs.FS, path string) (*g2.Manifest, error) {
	file, err := sysFS.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return g2.ParseManifestFromReader(file)
}

func parseRepo(sysFS fs.FS, repoDir string, defaultTitle string, fastGit bool) (*SiteData, error) {
	title := defaultTitle
	var repoName string

	// Get Git Info
	remoteURL, err := getGitOriginURL(repoDir)
	if err != nil {
		log.Printf("Warning: failed to get git origin url: %v", err)
	}

	repoNameBytes, err := fs.ReadFile(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "repo_name")))
	if err == nil && len(repoNameBytes) > 0 {
		title = strings.TrimSpace(string(repoNameBytes))
		repoName = title
	} else {
		repoName = filepath.Base(repoDir)
	}

	var eapi string
	eapiBytes, err := fs.ReadFile(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "eapi")))
	if err == nil && len(eapiBytes) > 0 {
		eapi = strings.TrimSpace(string(eapiBytes))
	}

	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	var lc *g2.LayoutConf
	if f, err := sysFS.Open(filepath.ToSlash(layoutConfPath)); err == nil {
		_ = f.Close()
		lc, err = parseLayoutConfFromFS(sysFS, filepath.ToSlash(layoutConfPath))
		if err != nil {
			log.Printf("Warning: failed to parse layout.conf: %v", err)
			lc = nil
		}
	}

	var licenseMapping map[string][]string
	licenseMappingPath := filepath.Join(repoDir, "metadata", "license-mapping.conf")
	if f, err := sysFS.Open(filepath.ToSlash(licenseMappingPath)); err == nil {
		mapping, err := g2.ParseLicenseMapping(f)
		_ = f.Close()
		if err != nil {
			log.Printf("Warning: failed to parse license-mapping.conf: %v", err)
		} else {
			licenseMapping = mapping
		}
	}
	qaPolicyPath := filepath.Join(repoDir, "metadata", "qa-policy.conf")
	var qa *g2.QAPolicy
	if f, err := sysFS.Open(filepath.ToSlash(qaPolicyPath)); err == nil {
		defer func() { _ = f.Close() }()
		qa, err = g2.ParseQAPolicyFromReader(f)
		if err != nil {
			log.Printf("Warning: failed to parse qa-policy.conf: %v", err)
			qa = nil
		}
	}

	site := &SiteData{
		Title:          title,
		RepoName:       repoName,
		RemoteURL:      remoteURL,
		EAPI:           eapi,
		LayoutConf:     lc,
		LicenseMapping: licenseMapping,
		QAPolicy:       qa,
	}

	// Parse News
	newsDir := filepath.Join(repoDir, "metadata", "news")
	if entries, err := fs.ReadDir(sysFS, filepath.ToSlash(newsDir)); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dirName := entry.Name()
			txtFile := filepath.Join(newsDir, dirName, dirName+".en.txt")

			content, err := fs.ReadFile(sysFS, filepath.ToSlash(txtFile))
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

	authorsFile, err := os.Open(filepath.Join(repoDir, "metadata", "AUTHORS"))
	if err == nil {
		if authors, err := g2.ParseAuthors(authorsFile); err == nil {
			site.Authors = authors
			if remoteURL != "" {
				commitHash, err := getFileCommit(repoDir, "metadata/AUTHORS")
				if err == nil && commitHash != "" {
					site.AuthorsURL = generateGitHubRawURL(remoteURL, commitHash, "metadata/AUTHORS")
				}
			}
		} else {
			log.Printf("Warning: failed to parse metadata/AUTHORS: %v", err)
		}
		_ = authorsFile.Close()
	}

	var profilesDescEntries []ProfileDescEntry
	profilesDescBytes, err := os.ReadFile(filepath.Join(repoDir, "profiles", "profiles.desc"))
	if err == nil {
		profilesDescEntries = parseProfilesDesc(string(profilesDescBytes))
	}

	profilesData, err := parseProfilesDir(repoDir, profilesDescEntries)
	if err != nil {
		log.Printf("Warning: failed to parse profiles dir: %v", err)
	}
	site.Profiles = profilesData

	supportedCategories := make(map[string]bool)
	categoriesBytes, err := fs.ReadFile(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "categories")))
	if err == nil {
		lines := strings.Split(string(categoriesBytes), "\n")
		for _, line := range lines {
			cat := strings.TrimSpace(line)
			if cat != "" && !strings.HasPrefix(cat, "#") {
				supportedCategories[cat] = true
			}
		}
	}

	updates, err := g2.ParseUpdatesDirFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "updates")))
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse updates: %v", err)
	}
	if updates != nil {
		site.Moves = updates.Moves
	}

	entries, err := fs.ReadDir(sysFS, filepath.ToSlash(repoDir))
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

		pkgEntries, err := fs.ReadDir(sysFS, filepath.ToSlash(catPath))
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
			files, err := fs.ReadDir(sysFS, filepath.ToSlash(pkgPath))
			if err != nil {
				log.Printf("Warning: reading package dir %s: %v", pkgPath, err)
				continue
			}

			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".ebuild") {
					continue
				}

				ebuildPath := filepath.Join(pkgPath, file.Name())
				subFS, err := fs.Sub(sysFS, filepath.ToSlash(filepath.Dir(ebuildPath)))
				if err != nil {
					log.Printf("Warning: subfs ebuild %s: %v", ebuildPath, err)
					continue
				}
				ebuild, err := g2.ParseEbuild(subFS, file.Name(), g2.ParseFull)
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
				relPath, _ := filepath.Rel(repoDir, ebuildPath)
				if remoteURL != "" {
					commitHash, _ := getFileCommit(repoDir, relPath)
					if commitHash != "" {
						ebuildRawURL = generateGitHubRawURL(remoteURL, commitHash, relPath)
					}
				}

				modTime := getFileModTime(repoDir, relPath, fastGit)
				if modTime.After(pkgData.ModTime) {
					pkgData.ModTime = modTime
				}

				pkgData.Versions = append(pkgData.Versions, VersionData{
					Version:      version,
					Ebuild:       ebuild,
					EbuildRawURL: ebuildRawURL,
					ModTime:      modTime,
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
			metadata, err := parseMetadataFromFS(sysFS, filepath.ToSlash(metaPath))
			if err == nil {
				if pkgMd, ok := metadata.(*g2.PkgMetadata); ok {
					pkgData.Metadata = pkgMd
				} else {
					pkgData.MetadataError = fmt.Errorf("metadata.xml is not a pkgmetadata")
				}
			} else {
				pkgData.MetadataError = err
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
			manifest, err := parseManifestFromFS(sysFS, filepath.ToSlash(manifestPath))
			if err == nil {
				pkgData.Manifest = manifest
				pkgData.ManifestData = buildManifestData(manifest, pkgData.Versions)
			}

			// Read files/ directory
			filesDirPath := filepath.Join(pkgPath, "files")
			if info, err := fs.Stat(sysFS, filepath.ToSlash(filesDirPath)); err == nil && info.IsDir() {
				fileEntries, err := fs.ReadDir(sysFS, filepath.ToSlash(filesDirPath))
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

			g2PkgData := g2.PackageData{
				Name:          pkgData.Name,
				Category:      pkgData.Category,
				Metadata:      pkgData.Metadata,
				MetadataError: pkgData.MetadataError,
				Manifest:      pkgData.Manifest,
			}
			for _, v := range pkgData.Versions {
				g2PkgData.Versions = append(g2PkgData.Versions, g2.VersionData{
					Version:      v.Version,
					Ebuild:       v.Ebuild,
					EbuildRawURL: v.EbuildRawURL,
				})
			}
			pkgData.LintWarnings = lints.PerformLinting(repoDir, &g2PkgData)

			catData.Packages = append(catData.Packages, pkgData)
		}

		if len(catData.Packages) > 0 {
			// TODO: Make a lint rule
			if len(supportedCategories) > 0 && !supportedCategories[name] {
				log.Printf("Warning: category '%s' is not listed in repo's profiles/categories", name)
			}
			mainCats := fetchMainGentooCategories()
			if len(mainCats) > 0 && !mainCats[name] {
				log.Printf("Warning: category '%s' is not in the main gentoo categories list", name)
			}

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

func parseProfilesDir(repoDir string, entries []ProfileDescEntry) ([]ProfileData, error) {
	profilesDir := filepath.Join(repoDir, "profiles")

	if info, err := os.Stat(profilesDir); err != nil || !info.IsDir() {
		return nil, nil
	}

	descMap := make(map[string]ProfileDescEntry)
	for _, e := range entries {
		descMap[e.Path] = e
	}

	profilesMap := make(map[string]*ProfileData)

	err := filepath.Walk(profilesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(profilesDir, path)
		if err != nil || relPath == "." {
			return nil
		}

		pData := &ProfileData{
			Path: relPath,
		}

		if desc, ok := descMap[relPath]; ok {
			pData.IsDesc = true
			pData.DescArch = desc.Arch
			pData.DescStat = desc.Status
		}

		parentBytes, err := os.ReadFile(filepath.Join(path, "parent"))
		if err == nil {
			lines := strings.Split(string(parentBytes), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				parentRelPath := filepath.Clean(filepath.Join(relPath, line))
				if !strings.HasPrefix(parentRelPath, "..") {
					pData.Parents = append(pData.Parents, parentRelPath)
				}
			}
		}

		profilesMap[relPath] = pData
		return nil
	})

	if err != nil {
		return nil, err
	}

	for path, pData := range profilesMap {
		for _, parentPath := range pData.Parents {
			if parent, ok := profilesMap[parentPath]; ok {
				parent.Children = append(parent.Children, path)
			}
		}
	}

	var result []ProfileData
	for _, pData := range profilesMap {
		result = append(result, *pData)
	}

	return result, nil
}

func parseProfilesDesc(content string) []ProfileDescEntry {
	var entries []ProfileDescEntry
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			entries = append(entries, ProfileDescEntry{
				Arch:   parts[0],
				Path:   parts[1],
				Status: parts[2],
			})
		}
	}
	return entries
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

// TODO check model's should be redundant OR migrated to /

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
	Aliases  []string
}

func parseDuration(s string) (time.Duration, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 90 * 24 * time.Hour, "3 months", nil
	}
	if strings.HasSuffix(s, "mo") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "mo"))
		if err != nil {
			return 0, "", err
		}
		if val == 1 {
			return time.Duration(val) * 30 * 24 * time.Hour, "1 month", nil
		}
		return time.Duration(val) * 30 * 24 * time.Hour, fmt.Sprintf("%d months", val), nil
	}
	if strings.HasSuffix(s, "d") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, "", err
		}
		if val == 1 {
			return time.Duration(val) * 24 * time.Hour, "1 day", nil
		}
		return time.Duration(val) * 24 * time.Hour, fmt.Sprintf("%d days", val), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, "", err
	}
	return d, s, nil
}

// TODO migrate to / if it hasn't been done already check for differences

type AggProfileRepo struct {
	RepoName string
	Profile  ProfileData
}

type AggProfile struct {
	Path     string
	IsDesc   bool
	DescArch string
	DescStat string
	Repos    []AggProfileRepo
}

type AggPackageMove struct {
	Old string
	New string
}

type AggNewsItem struct {
	NewsItem
	RepoName string
}

func generateSite(outDir string, sites []*SiteData, recentDuration time.Duration, recentDurationStr string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"join": strings.Join,
	}).ParseFS(siteTemplates, "sitegen_templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	var allPackages []PackageData
	for _, site := range sites {
		for _, cat := range site.Categories {
			allPackages = append(allPackages, cat.Packages...)
		}
	}
	aggCategories := make(map[string]*AggCategory)
	aggPackages := make(map[string]*AggPackage)
	aggLicenses := make(map[string]*AggLicense)
	aggProfiles := make(map[string]*AggProfile)
	aggMoves := make(map[string]*AggPackageMove)
	var globalNews []AggNewsItem

	totalPackages := 0

	for _, site := range sites {
		for _, p := range site.Profiles {
			if _, ok := aggProfiles[p.Path]; !ok {
				aggProfiles[p.Path] = &AggProfile{
					Path: p.Path,
				}
			}
			aggProfiles[p.Path].Repos = append(aggProfiles[p.Path].Repos, AggProfileRepo{
				RepoName: site.RepoName,
				Profile:  p,
			})
			if p.IsDesc {
				aggProfiles[p.Path].IsDesc = true
				aggProfiles[p.Path].DescArch = p.DescArch
				aggProfiles[p.Path].DescStat = p.DescStat
			}
		}
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

							// Add aliases from this site's license mapping
							if site.LicenseMapping != nil {
								if aliases, ok := site.LicenseMapping[lic]; ok {
									for _, alias := range aliases {
										hasAlias := false
										for _, existing := range aggLicenses[lic].Aliases {
											if existing == alias {
												hasAlias = true
												break
											}
										}
										if !hasAlias {
											aggLicenses[lic].Aliases = append(aggLicenses[lic].Aliases, alias)
										}
									}
								}
							}
						}
					}
				}
			}
		}
		for _, move := range site.Moves {
			if _, ok := aggMoves[move.Old]; !ok {
				aggMoves[move.Old] = &AggPackageMove{Old: move.Old, New: move.New}
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
		sort.Strings(l.Aliases)
		sortedLicenses = append(sortedLicenses, l)
	}
	sort.Slice(sortedLicenses, func(i, j int) bool { return sortedLicenses[i].Name < sortedLicenses[j].Name })

	var sortedProfiles []*AggProfile
	for _, p := range aggProfiles {
		sortedProfiles = append(sortedProfiles, p)
	}
	sort.Slice(sortedProfiles, func(i, j int) bool { return sortedProfiles[i].Path < sortedProfiles[j].Path })
	sort.Slice(globalNews, func(i, j int) bool {
		return globalNews[i].Posted.After(globalNews[j].Posted)
	})

	// Generate Feeds for Repo
	var repoFeedItems []g2.FeedItem
	for _, pkg := range allPackages {
		for _, ver := range pkg.Versions {
			desc := ""
			if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
				desc = ver.Ebuild.Vars["DESCRIPTION"]
			}
			_ = append(repoFeedItems, g2.FeedItem{
				Title:       fmt.Sprintf("%s/%s-%s", pkg.Category, pkg.Name, ver.Version),
				Link:        fmt.Sprintf("packages/%s/", pkg.Name),
				Description: desc,
				PubDate:     time.Now().Format(time.RFC1123Z),
				Updated:     time.Now().Format(time.RFC3339),
			})
		}
	}
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

	// Global Moved Packages Pages
	for oldPath, move := range aggMoves {
		parts := strings.Split(oldPath, "/")
		if len(parts) != 2 {
			continue
		}
		oldCat, oldName := parts[0], parts[1]

		pkgKey := oldCat + "/" + oldName
		if _, exists := aggPackages[pkgKey]; exists {
			continue // skip if a package now exists at this location
		}

		newParts := strings.Split(move.New, "/")
		if len(newParts) != 2 {
			continue
		}

		pkgDir := filepath.Join(outDir, "packages", oldCat, oldName)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", pkgDir, err)
		}

		if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "moved_package.html", map[string]interface{}{
			"Title":       "Package Moved: " + oldCat + "/" + oldName,
			"BaseURL":     "../../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Packages", URL: "../../"}, {Name: oldCat}, {Name: oldName}},
			"OldName":     oldCat + "/" + oldName,
			"NewName":     move.New,
			"NewURL":      "../../" + newParts[0] + "/" + newParts[1] + "/",
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
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
					t := ver.ModTime
					if t.IsZero() {
						t = time.Now()
					}
					globalFeedItems = append(globalFeedItems, FeedItem{
						Title:       fmt.Sprintf("%s/%s-%s (%s)", sPkg.Category, sPkg.Name, ver.Version, site.RepoName),
						Link:        fmt.Sprintf("repos/%s/categories/%s/packages/%s/", site.RepoName, sPkg.Category, sPkg.Name),
						Description: desc,
						PubDate:     t.Format(time.RFC1123Z),
						Updated:     t.Format(time.RFC3339),
						Time:        t,
					})
				}
			}
		}
	}
	sort.Slice(globalFeedItems, func(i, j int) bool {
		return globalFeedItems[i].Time.After(globalFeedItems[j].Time)
	})
	var recentGlobalUpdates []FeedItem
	recentLimit := time.Now().Add(-recentDuration)
	for _, item := range globalFeedItems {
		if item.Time.After(recentLimit) {
			recentGlobalUpdates = append(recentGlobalUpdates, item)
			if len(recentGlobalUpdates) >= 10 {
				break
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(outDir, "recent"), 0755); err != nil {
		return err
	}
	var allRecentGlobal []FeedItem
	if len(globalFeedItems) > 500 {
		allRecentGlobal = append([]FeedItem(nil), globalFeedItems[:500]...)
	} else {
		allRecentGlobal = append([]FeedItem(nil), globalFeedItems...)
	}
	for i := range allRecentGlobal {
		if allRecentGlobal[i].Link != "" && !strings.HasPrefix(allRecentGlobal[i].Link, "http") {
			allRecentGlobal[i].Link = "../" + allRecentGlobal[i].Link
		}
	}
	if err := renderPage(filepath.Join(outDir, "recent", "index.html"), tmpl, "recent.html", map[string]interface{}{
		"Title":       "Recent Updates",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Recent Updates"}},
		"Updates":     allRecentGlobal,
		"Version":     version,
	}); err != nil {
		return err
	}

	if len(globalFeedItems) > 50 {
		globalFeedItems = globalFeedItems[:50]
	}
	var globalG2FeedItems []g2.FeedItem
	for _, fi := range globalFeedItems {
		globalG2FeedItems = append(globalG2FeedItems, g2.FeedItem{
			Title:       fi.Title,
			Link:        fi.Link,
			Description: fi.Description,
			PubDate:     fi.PubDate,
			Updated:     fi.Updated,
		})
	}
	if err := generateFeeds(filepath.Join(outDir, "index"), title, "Latest updates to global repository", "", globalG2FeedItems); err != nil {
		log.Printf("Warning: failed to generate global feed: %v", err)
	}

	// 1. Root Dashboard
	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "dashboard.html", map[string]interface{}{
		"Title":                title,
		"BaseURL":              "",
		"Repos":                sites,
		"Categories":           sortedCategories,
		"Packages":             sortedPackages,
		"Licenses":             sortedLicenses,
		"Updates":              recentGlobalUpdates,
		"Version":              version,
		"RecentDurationString": recentDurationStr,
		"RecentNews":           recentNews,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	// 1b. Global News Dashboard
	if len(globalNews) > 0 {
		if err := os.MkdirAll(filepath.Join(outDir, "news"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "news", "index.html"), tmpl, "news_dashboard.html", map[string]interface{}{
			"Title":       "News Dashboard",
			"BaseURL":     "../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "News"}},
			"RecentNews":  recentNews,
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		// Global News Archive
		if err := os.MkdirAll(filepath.Join(outDir, "news", "archive"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "news", "archive", "index.html"), tmpl, "news_archive.html", map[string]interface{}{
			"Title":       "News Archive",
			"BaseURL":     "../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
			"News":        globalNews,
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		// Global News Articles
		for _, n := range globalNews {
			newsDir := filepath.Join(outDir, "news", "archive", n.DirName)
			if err := os.MkdirAll(newsDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", newsDir, err)
			}
			if err := renderPage(filepath.Join(newsDir, "index.html"), tmpl, "news_article.html", map[string]interface{}{
				"Title":       n.Title,
				"BaseURL":     "../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
				"NewsItem":    n,
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}

	// 2. Overlays List
	if err := os.MkdirAll(filepath.Join(outDir, "overlays"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "overlays", "index.html"), tmpl, "overlays.html", map[string]interface{}{
		"Title":       "Overlays",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Overlays"}},
		"Repos":       sites,
		"Version":     version,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	// 3. Global Categories
	if err := os.MkdirAll(filepath.Join(outDir, "categories"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "categories", "index.html"), tmpl, "categories.html", map[string]interface{}{
		"Title":       "Categories",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Categories"}},
		"Categories":  sortedCategories,
		"Version":     version,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, cat := range sortedCategories {
		catDir := filepath.Join(outDir, "categories", cat.Name)
		if err := os.MkdirAll(catDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", catDir, err)
		}

		var catPkgs []*AggPackage
		for _, p := range cat.Packages {
			catPkgs = append(catPkgs, p)
		}
		sort.Slice(catPkgs, func(i, j int) bool { return catPkgs[i].Name < catPkgs[j].Name })

		type TmplPkg struct {
			Name      string
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
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	// Profiles
	if err := os.MkdirAll(filepath.Join(outDir, "profiles"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "profiles", "index.html"), tmpl, "profiles.html", map[string]interface{}{
		"Title":       "Profiles",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Profiles"}},
		"Profiles":    sortedProfiles,
		"Version":     version,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, p := range sortedProfiles {
		profDir := filepath.Join(outDir, "profiles", p.Path)
		if err := os.MkdirAll(profDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", profDir, err)
		}

		relToRoot := "../../"
		for i := 0; i < strings.Count(p.Path, "/"); i++ {
			relToRoot += "../"
		}

		if err := renderPage(filepath.Join(profDir, "index.html"), tmpl, "profile.html", map[string]interface{}{
			"Title":       "Profile: " + p.Path,
			"BaseURL":     relToRoot,
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: relToRoot}, {Name: "Profiles", URL: relToRoot + "profiles/"}, {Name: p.Path}},
			"ProfilePath": p.Path,
			"ProfileList": p.Repos,
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	// 4. Global Packages
	if err := os.MkdirAll(filepath.Join(outDir, "packages"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "packages", "index.html"), tmpl, "packages.html", map[string]interface{}{
		"Title":       "Packages",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Packages"}},
		"Packages":    sortedPackages,
		"Version":     version,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, pkg := range sortedPackages {
		pkgDir := filepath.Join(outDir, "packages", pkg.Category, pkg.Name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", pkgDir, err)
		}

		reposList := mapToList(pkg.Repos)

		if len(reposList) == 1 {
			targetURL := fmt.Sprintf("../../../repos/%s/categories/%s/packages/%s/", reposList[0].RepoName, pkg.Category, pkg.Name)
			redirectHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0; url=%s"></head><body><a href="%s">Redirecting...</a></body></html>`, targetURL, targetURL)
			if err := os.WriteFile(filepath.Join(pkgDir, "index.html"), []byte(redirectHTML), 0644); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		} else {
			var movedToName, movedToURL string
			if move, ok := aggMoves[pkg.Category+"/"+pkg.Name]; ok {
				newParts := strings.Split(move.New, "/")
				if len(newParts) == 2 {
					movedToName = move.New
					movedToURL = "../../" + newParts[0] + "/" + newParts[1] + "/"
				}
			}

			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "package_picker.html", map[string]interface{}{
				"Title":       "Package: " + pkg.Category + "/" + pkg.Name,
				"BaseURL":     "../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Packages", URL: "../../"}, {Name: pkg.Category}, {Name: pkg.Name}},
				"Package":     map[string]interface{}{"Category": pkg.Category, "Name": pkg.Name, "ReposList": reposList},
				"MovedToName": movedToName,
				"MovedToURL":  movedToURL,
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}

	// 5. Global Licenses
	if err := os.MkdirAll(filepath.Join(outDir, "licenses"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "licenses", "index.html"), tmpl, "licenses.html", map[string]interface{}{
		"Title":       "Licenses",
		"BaseURL":     "../",
		"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../"}, {Name: "Licenses"}},
		"Licenses":    sortedLicenses,
		"Version":     version,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, lic := range sortedLicenses {
		licDir := filepath.Join(outDir, "licenses", lic.Name)
		if err := os.MkdirAll(licDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", licDir, err)
		}

		type TmplPkg struct {
			Name      string
			Category  string
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
			"License":     map[string]interface{}{"Name": lic.Name, "Packages": tmplPkgs, "Text": lic.Text, "Aliases": lic.Aliases},
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	// 6. Repo-Specific Pages
	for _, site := range sites {
		repoDir := filepath.Join(outDir, "repos", site.RepoName)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", repoDir, err)
		}

		// Repo Moved Packages Pages
		for _, move := range site.Moves {
			parts := strings.Split(move.Old, "/")
			if len(parts) != 2 {
				continue
			}
			oldCat, oldName := parts[0], parts[1]

			// Check if package exists in this repo currently
			pkgExists := false
			for _, cat := range site.Categories {
				if cat.Name == oldCat {
					for _, pkg := range cat.Packages {
						if pkg.Name == oldName {
							pkgExists = true
							break
						}
					}
				}
				if pkgExists {
					break
				}
			}
			if pkgExists {
				continue
			}

			newParts := strings.Split(move.New, "/")
			if len(newParts) != 2 {
				continue
			}

			pkgDir := filepath.Join(repoDir, "categories", oldCat, "packages", oldName)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", pkgDir, err)
			}

			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "moved_package.html", map[string]interface{}{
				"Title":       fmt.Sprintf("%s - %s/%s (Moved)", site.RepoName, oldCat, oldName),
				"BaseURL":     "../../../../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../../../"}, {Name: site.RepoName, URL: "../../../../"}, {Name: "Categories", URL: "../../../"}, {Name: oldCat}, {Name: oldName}},
				"Repo":        site,
				"OldName":     oldCat + "/" + oldName,
				"NewName":     move.New,
				"NewURL":      "../../../" + newParts[0] + "/packages/" + newParts[1] + "/",
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
		var repoFeedItems []FeedItem
		for _, cat := range site.Categories {
			for _, pkg := range cat.Packages {
				for _, ver := range pkg.Versions {
					desc := ""
					if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
						desc = ver.Ebuild.Vars["DESCRIPTION"]
					}
					t := ver.ModTime
					if t.IsZero() {
						t = time.Now()
					}
					repoFeedItems = append(repoFeedItems, FeedItem{
						Title:       fmt.Sprintf("%s/%s-%s", pkg.Category, pkg.Name, ver.Version),
						Link:        fmt.Sprintf("categories/%s/packages/%s/", pkg.Category, pkg.Name),
						Description: desc,
						PubDate:     t.Format(time.RFC1123Z),
						Updated:     t.Format(time.RFC3339),
						Time:        t,
					})
				}
			}
		}
		sort.Slice(repoFeedItems, func(i, j int) bool {
			return repoFeedItems[i].Time.After(repoFeedItems[j].Time)
		})
		var recentRepoUpdates []FeedItem
		for _, item := range repoFeedItems {
			if item.Time.After(recentLimit) {
				recentRepoUpdates = append(recentRepoUpdates, item)
				if len(recentRepoUpdates) >= 10 {
					break
				}
			}
		}

		if err := os.MkdirAll(filepath.Join(repoDir, "recent"), 0755); err != nil {
			return err
		}
		var allRecentRepo []FeedItem
		if len(repoFeedItems) > 500 {
			allRecentRepo = append([]FeedItem(nil), repoFeedItems[:500]...)
		} else {
			allRecentRepo = append([]FeedItem(nil), repoFeedItems...)
		}
		for i := range allRecentRepo {
			if allRecentRepo[i].Link != "" && !strings.HasPrefix(allRecentRepo[i].Link, "http") {
				allRecentRepo[i].Link = "../" + allRecentRepo[i].Link
			}
		}
		if err := renderPage(filepath.Join(repoDir, "recent", "index.html"), tmpl, "recent.html", map[string]interface{}{
			"Title":       site.RepoName + " - Recent Updates",
			"BaseURL":     "../../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Recent Updates"}},
			"Updates":     allRecentRepo,
			"Version":     version,
		}); err != nil {
			return err
		}

		if len(repoFeedItems) > 50 {
			repoFeedItems = repoFeedItems[:50]
		}
		var repoG2FeedItems []g2.FeedItem
		for _, fi := range repoFeedItems {
			repoG2FeedItems = append(repoG2FeedItems, g2.FeedItem{
				Title:       fi.Title,
				Link:        fi.Link,
				Description: fi.Description,
				PubDate:     fi.PubDate,
				Updated:     fi.Updated,
			})
		}
		if err := generateFeeds(filepath.Join(repoDir, "index"), site.RepoName, "Latest updates to repository", "", repoG2FeedItems); err != nil {
			log.Printf("Warning: failed to generate repo feed: %v", err)
		}

		pkgCount := 0
		for _, c := range site.Categories {
			pkgCount += len(c.Packages)
		}

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
			"Title":                site.RepoName,
			"BaseURL":              "../../",
			"Breadcrumbs":          []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Overlays", URL: "../../overlays/"}, {Name: site.RepoName}},
			"Repo":                 site,
			"PackageCount":         pkgCount,
			"Updates":              recentRepoUpdates,
			"Version":              version,
			"RecentDurationString": recentDurationStr,
			"RecentNews":           repoRecentNews,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		if err := os.MkdirAll(filepath.Join(repoDir, "profiles"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "profiles", "index.html"), tmpl, "repo_profiles.html", map[string]interface{}{
			"Title":       site.RepoName + " - Profiles",
			"BaseURL":     "../../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Profiles"}},
			"Repo":        site,
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, p := range site.Profiles {
			profDir := filepath.Join(repoDir, "profiles", p.Path)
			if err := os.MkdirAll(profDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", profDir, err)
			}

			relToRoot := "../../../../"
			for i := 0; i < strings.Count(p.Path, "/"); i++ {
				relToRoot += "../"
			}

			if err := renderPage(filepath.Join(profDir, "index.html"), tmpl, "repo_profile.html", map[string]interface{}{
				"Title":       site.RepoName + " - Profile: " + p.Path,
				"BaseURL":     relToRoot,
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: relToRoot}, {Name: site.RepoName, URL: relToRoot + "repos/" + site.RepoName + "/"}, {Name: "Profiles", URL: relToRoot + "repos/" + site.RepoName + "/profiles/"}, {Name: p.Path}},
				"RepoName":    site.RepoName,
				"ProfilePath": p.Path,
				"Profile":     p,
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}

		// Repo News Dashboard
		if len(site.News) > 0 {
			if err := os.MkdirAll(filepath.Join(repoDir, "news"), 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			if err := renderPage(filepath.Join(repoDir, "news", "index.html"), tmpl, "news_dashboard.html", map[string]interface{}{
				"Title":       site.RepoName + " - News Dashboard",
				"BaseURL":     "../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Overlays", URL: "../../../overlays/"}, {Name: site.RepoName, URL: "../"}, {Name: "News"}},
				"RecentNews":  repoRecentNews,
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}

			// Repo News Archive
			if err := os.MkdirAll(filepath.Join(repoDir, "news", "archive"), 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			if err := renderPage(filepath.Join(repoDir, "news", "archive", "index.html"), tmpl, "news_archive.html", map[string]interface{}{
				"Title":       site.RepoName + " - News Archive",
				"BaseURL":     "../../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: "Overlays", URL: "../../../../overlays/"}, {Name: site.RepoName, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
				"News":        site.News,
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}

			// Repo News Articles
			for _, n := range site.News {
				newsDir := filepath.Join(repoDir, "news", "archive", n.DirName)
				if err := os.MkdirAll(newsDir, 0755); err != nil {
					return fmt.Errorf("creating directory %s: %w", newsDir, err)
				}
				if err := renderPage(filepath.Join(newsDir, "index.html"), tmpl, "news_article.html", map[string]interface{}{
					"Title":       n.Title,
					"BaseURL":     "../../../../../",
					"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../../"}, {Name: "Overlays", URL: "../../../../../overlays/"}, {Name: site.RepoName, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
					"NewsItem":    n,
					"Version":     version,
				}); err != nil {
					return fmt.Errorf("rendering page: %w", err)
				}
			}
		}

		if err := os.MkdirAll(filepath.Join(repoDir, "categories"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "categories", "index.html"), tmpl, "categories.html", map[string]interface{}{
			"Title":       site.RepoName + " - Categories",
			"BaseURL":     "../../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Categories"}},
			"Categories":  site.Categories,
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		if len(site.Authors) > 0 {
			if err := os.MkdirAll(filepath.Join(repoDir, "authors"), 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			if err := renderPage(filepath.Join(repoDir, "authors", "index.html"), tmpl, "authors.html", map[string]interface{}{
				"Title":       site.RepoName + " - Authors",
				"BaseURL":     "../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Authors"}},
				"Authors":     site.Authors,
				"Repo":        site,
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
		for _, cat := range site.Categories {
			catDir := filepath.Join(repoDir, "categories", cat.Name)
			if err := os.MkdirAll(catDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", catDir, err)
			}

			type TmplPkg struct {
				Name      string
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
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}

		if err := os.MkdirAll(filepath.Join(repoDir, "packages"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		var repoPkgs []PackageData
		for _, c := range site.Categories {
			repoPkgs = append(repoPkgs, c.Packages...)
		}
		sort.Slice(repoPkgs, func(i, j int) bool {
			if repoPkgs[i].Category == repoPkgs[j].Category {
				return repoPkgs[i].Name < repoPkgs[j].Name
			}
			return repoPkgs[i].Category < repoPkgs[j].Category
		})

		if err := renderPage(filepath.Join(repoDir, "packages", "index.html"), tmpl, "repo_packages.html", map[string]interface{}{
			"Title":       site.RepoName + " - Packages",
			"BaseURL":     "../../../",
			"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Packages"}},
			"Packages":    repoPkgs,
			"Repo":        site,
			"Version":     version,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, pkg := range repoPkgs {
			pkgDir := filepath.Join(repoDir, "categories", pkg.Category, "packages", pkg.Name)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", pkgDir, err)
			}

			var pkgFeedItems []FeedItem
			for _, ver := range pkg.Versions {
				desc := ""
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					desc = ver.Ebuild.Vars["DESCRIPTION"]
				}
				t := ver.ModTime
				if t.IsZero() {
					t = time.Now()
				}
				pkgFeedItems = append(pkgFeedItems, FeedItem{
					Title:       fmt.Sprintf("%s/%s-%s", pkg.Category, pkg.Name, ver.Version),
					Link:        "",
					Description: desc,
					PubDate:     t.Format(time.RFC1123Z),
					Updated:     t.Format(time.RFC3339),
					Time:        t,
				})
			}
			sort.Slice(pkgFeedItems, func(i, j int) bool {
				return pkgFeedItems[i].Time.After(pkgFeedItems[j].Time)
			})
			var pkgG2FeedItems []g2.FeedItem
			for _, fi := range pkgFeedItems {
				pkgG2FeedItems = append(pkgG2FeedItems, g2.FeedItem{
					Title:       fi.Title,
					Link:        fi.Link,
					Description: fi.Description,
					PubDate:     fi.PubDate,
					Updated:     fi.Updated,
				})
			}
			if err := generateFeeds(filepath.Join(pkgDir, "index"), pkg.Category+"/"+pkg.Name, "Latest updates to package", "", pkgG2FeedItems); err != nil {
				log.Printf("Warning: failed to generate package feed: %v", err)
			}

			var movedToName, movedToURL string
			for _, move := range site.Moves {
				if move.Old == pkg.Category+"/"+pkg.Name {
					newParts := strings.Split(move.New, "/")
					if len(newParts) == 2 {
						movedToName = move.New
						movedToURL = "../../../" + newParts[0] + "/packages/" + newParts[1] + "/"
					}
					break
				}
			}

			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "repo_package.html", map[string]interface{}{
				"Title":       fmt.Sprintf("%s - %s/%s", site.RepoName, pkg.Category, pkg.Name),
				"BaseURL":     "../../../../../../",
				"Breadcrumbs": []Breadcrumb{{Name: title, URL: "../../../../../../"}, {Name: site.RepoName, URL: "../../../../"}, {Name: "Categories", URL: "../../../"}, {Name: pkg.Category}, {Name: pkg.Name}},
				"Repo":        site,
				"Package":     pkg,
				"MovedToName": movedToName,
				"MovedToURL":  movedToURL,
				"Version":     version,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}

	return nil
}
func renderPage(path string, tmpl *template.Template, name string, data map[string]interface{}) error {
	log.Printf("Rendering page %s using template %s", path, name)
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("executing template %s for path %s: %w", name, path, err)
	}

	data["Content"] = template.HTML(buf.String())

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if err := tmpl.ExecuteTemplate(f, "layout.html", data); err != nil {
		return fmt.Errorf("executing layout template for %s: %w", path, err)
	}
	return nil
}

func (cfg *MainArgConfig) cmdSiteRemote(repositoriesFile string, outDir string, recentDuration time.Duration, recentDurationStr string, fastGit bool) error {
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

	var repos g2.RemoteRepositories
	if err := xml.Unmarshal(data, &repos); err != nil {
		return fmt.Errorf("parsing repositories.xml: %w", err)
	}

	// Create a temporary directory to clone repos into
	tmpDir, err := os.MkdirTemp("", "g2-sitegen-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	overallSiteData := &g2.SiteData{
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
		siteData, err := parseRepo(os.DirFS(repoPath), ".", repo.Name, fastGit)
		if err != nil {
			log.Printf("Failed to parse repo %s: %v", repo.Name, err)
			continue
		}

		// Since user requested a "list of overlays" we should put each repo in a subfolder
		// generateSite will write into outDir/repo.Name
		repoOutDir := filepath.Join(outDir, repo.Name)
		log.Printf("Generating site for repo: %s", repo.Name)
		if err := generateSite(repoOutDir, []*SiteData{siteData}, recentDuration, recentDurationStr); err != nil {
			log.Printf("Failed to generate site for repo %s: %v", repo.Name, err)
		}
		// Add as a root category simply so we can create an index.html listing the repos
		overallSiteData.Categories = append(overallSiteData.Categories, g2.CategoryData{
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

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"join": strings.Join,
	}).ParseFS(siteTemplates, "sitegen_templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "index.html", map[string]interface{}{
		"Title":       overallSiteData.Title,
		"BaseURL":     "",
		"Categories":  overallSiteData.Categories,
		"Breadcrumbs": []g2.Breadcrumb{},
	}); err != nil {
		return fmt.Errorf("rendering overall index page: %w", err)
	}

	log.Println("Remote site generation complete.")
	return nil
}
