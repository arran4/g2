package main

import (
	"context"

	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	path2 "path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arran4/g2"

	"github.com/arran4/g2/lints/ebuild"
	"golang.org/x/sync/errgroup"
)

func getProcessMemUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

type SourceURL string

func getDefaultCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return filepath.Join(os.TempDir(), "g2-cache")
	}
	return filepath.Join(cacheDir, "g2")
}

func (cfg *MainArgConfig) cmdOverlay(args []string) error {
	ebuild.SkipForSiteGen = true
	if len(args) < 1 {
		return fmt.Errorf("missing subcommand for overlay (e.g., site)")
	}
	subcmd := args[0]
	if subcmd == "ebuild" {
		return cfg.cmdOverlayEbuild(args[1:])
	}
	if subcmd == "license" {
		return cfg.cmdOverlayLicense(args[1:])
	}
	if subcmd == "info-vars" {
		return cfg.cmdOverlayInfoVars(args[1:])
	}
	if subcmd == "info-pkgs" {
		return cfg.cmdOverlayInfoPkgs(args[1:])
	}
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
	profileSiteGen := fs.Bool("profile", false, "Generate a profile report of site generation")
	profileOut := fs.String("profile-out", "profile.txt", "Output file for the profile report, or '-' for stdout")
	outDir := fs.String("out", "site_out", "Output directory for the generated site")
	clear := fs.Bool("clear", false, "Clear output directory before generation")
	recentDurOpt := fs.String("recent-duration", "3mo", "Duration to consider an update 'recent' (e.g. 3mo, 14d, 72h)")
	fastGit := fs.Bool("fast-git-modtime", false, "Use fast (O(1)) but potentially less reliable go-git file log lookup")
	useZip := fs.Bool("use-zip", false, "Download zip archives instead of git clone when supported")

	workMode := fs.String("work-mode", "persistent", "Work mode: 'persistent' or 'temp'")
	persistentDir := fs.String("persistent-dir", getDefaultCacheDir(), "Directory to persistently store checked out repositories instead of a temporary directory")
	tempDir := fs.String("temp-dir", "", "Directory to use for temporary files instead of the default")
	includeGentoo := fs.Bool("include-gentoo", false, "Include the base Gentoo repository")
	includeGuru := fs.Bool("include-guru", false, "Include the Guru repository")
	reposConfOpt := fs.String("repos-conf", "", "Path to repos.conf file or directory")

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

	log.Printf("Generating site (v%s) from overlay location %s into %s", version, location, *outDir)

	type repoTask struct {
		Name     string
		Location string
	}

	tasks := []repoTask{
		{Name: "Gentoo Packages", Location: location},
	}

	if *includeGentoo {
		tasks = append(tasks, repoTask{Name: "gentoo", Location: "https://github.com/gentoo-mirror/gentoo.git"})
	}
	if *includeGuru {
		tasks = append(tasks, repoTask{Name: "guru", Location: "https://github.com/gentoo-mirror/guru.git"})
	}

	if *reposConfOpt != "" {
		rc, err := g2.ParseReposConf(*reposConfOpt)
		if err != nil {
			return fmt.Errorf("parsing repos.conf: %w", err)
		}
		for _, f := range rc.Files {
			for _, s := range f.Sections {
				if s.Name == "DEFAULT" || s.Disabled {
					continue
				}
				loc := s.Get("location")
				syncURI := s.Get("sync-uri")
				if syncURI != "" {
					tasks = append(tasks, repoTask{Name: s.Name, Location: syncURI})
				} else if loc != "" {
					tasks = append(tasks, repoTask{Name: s.Name, Location: loc})
				}
			}
		}
	}

	var allSites []*g2.SiteData
	var allSitesMu sync.Mutex
	var lastLogTime time.Time

	g, _ := errgroup.WithContext(context.Background())

	for _, task := range tasks {
		task := task
		g.Go(func() error {
			isRemote := strings.HasPrefix(task.Location, "http://") || strings.HasPrefix(task.Location, "https://") || strings.HasPrefix(task.Location, "git://")
			var parseLocation string
			var cleanup func()
			var siteData *g2.SiteData

			if isRemote {
				var tmpDir string
				var err error

				if *workMode == "persistent" {
					tmpDir = filepath.Join(*persistentDir, task.Name)
					if err := os.MkdirAll(tmpDir, 0755); err != nil {
						return fmt.Errorf("creating persistent dir for %s: %w", task.Name, err)
					}
					cleanup = func() {}
				} else {
					tmpDir, err = os.MkdirTemp(*tempDir, "g2-overlay-"+task.Name+"-*")
					if err != nil {
						return fmt.Errorf("creating temp dir for %s: %w", task.Name, err)
					}
					cleanup = func() { _ = os.RemoveAll(tmpDir) }
				}
				defer cleanup()

				log.Printf("Cloning remote repository: %s", task.Location)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				t0 := time.Now()
				if err := FetchRepo(ctx, task.Location, tmpDir, *useZip, *workMode, 0); err != nil {
					return fmt.Errorf("cloning repository %s: %w", task.Name, err)
				}
				checkoutTime := time.Since(t0)

				parseLocation = tmpDir

				size, err := getDirSize(parseLocation)
				var gitSize string
				if err == nil {
					gitSize = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
				}

				t1 := time.Now()
				siteData, err = parseRepo(os.DirFS(parseLocation), ".", task.Name, *fastGit, nil, SourceURL(task.Location))
				if err != nil {
					return fmt.Errorf("parsing repo %s: %w", task.Name, err)
				}
				processTime := time.Since(t1)

				siteData.CheckoutTime = checkoutTime.String()
				siteData.ProcessTime = processTime.String()
				siteData.GitSize = gitSize

			} else {
				parseLocation = task.Location
				cleanup = func() {}
				defer cleanup()

				size, err := getDirSize(parseLocation)
				var gitSize string
				if err == nil {
					gitSize = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
				}

				t1 := time.Now()
				siteData, err = parseRepo(os.DirFS(parseLocation), ".", task.Name, *fastGit, nil, SourceURL(task.Location))
				if err != nil {
					return fmt.Errorf("parsing repo %s: %w", task.Name, err)
				}
				processTime := time.Since(t1)

				siteData.ProcessTime = processTime.String()
				siteData.GitSize = gitSize
			}

			allSitesMu.Lock()
			allSites = append(allSites, siteData)
			now := time.Now()
			if lastLogTime.IsZero() || now.Sub(lastLogTime) >= 10*time.Minute {
				lastLogTime = now
				currentRepos := len(allSites)
				var currentPackages int
				for _, site := range allSites {
					if site != nil {
						currentPackages += site.PackageCount
					}
				}
				log.Printf("[PROGRESS] Currently processed %d repositories and %d total packages so far", currentRepos, currentPackages)
			}
			allSitesMu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("fetching and parsing repos: %w", err)
	}

	sort.Slice(allSites, func(i, j int) bool {
		return allSites[i].RepoName < allSites[j].RepoName
	})

	genInfo := GenerationInfo{Args: cfg.Args, FastGit: *fastGit, RecentDuration: recentDurationStr}
	profiler := NewProfiler(*profileSiteGen, *profileOut)
	genInfo.Profiler = profiler
	if err := generateSite(*outDir, allSites, recentDuration, recentDurationStr, genInfo); err != nil {
		return fmt.Errorf("generating site: %w", err)
	}

	log.Println("Site generation complete.")
	return nil
}

func (cfg *MainArgConfig) cmdOverlays(args []string) error {
	ebuild.SkipForSiteGen = true
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
	profileSiteGen := fs.Bool("profile", false, "Generate a profile report of site generation")
	profileOut := fs.String("profile-out", "profile.txt", "Output file for the profile report, or '-' for stdout")
	outDir := fs.String("out", "site_out", "Output directory for the generated site")
	clear := fs.Bool("clear", false, "Clear output directory before generation")
	recentDurOpt := fs.String("recent-duration", "3mo", "Duration to consider an update 'recent' (e.g. 3mo, 14d, 72h)")
	fastGit := fs.Bool("fast-git-modtime", false, "Use fast (O(1)) but potentially less reliable go-git file log lookup")
	useZip := fs.Bool("use-zip", false, "Download zip archives instead of git clone when supported")
	smartMode := fs.Bool("smart-mode", true, "Use smart mode for parsing repositories in memory without disk access")

	concurrency := fs.Int("concurrency", 4, "Maximum number of concurrent repository fetches/parses")
	retries := fs.Int("retries", 3, "Number of times to retry fetching a repository")
	continueOnError := fs.Bool("continue-on-error", true, "Continue parsing other repositories even if fetching one fails")
	workMode := fs.String("work-mode", "persistent", "Work mode: 'persistent' or 'temp'")
	persistentDir := fs.String("persistent-dir", getDefaultCacheDir(), "Directory to persistently store checked out repositories instead of a temporary directory")
	tempDir := fs.String("temp-dir", "", "Directory to use for temporary files instead of the default")
	reposConfOpt := fs.String("repos-conf", "", "Path to repos.conf file or directory")
	mode := fs.String("mode", "standard", "Processing mode: 'standard' or 'pipeline'")

	if err := fs.Parse(args[2:]); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	recentDuration, recentDurationStr, err := parseDuration(*recentDurOpt)
	if err != nil {
		return fmt.Errorf("invalid recent-duration: %w", err)
	}

	if fs.NArg() == 0 && *reposConfOpt == "" {
		return fmt.Errorf("missing location argument (url, file path, or - for stdin) and repos-conf is empty")
	}

	location := ""
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	if *clear {
		if err := os.RemoveAll(*outDir); err != nil {
			return fmt.Errorf("clearing output directory: %w", err)
		}
	}

	log.Printf("Generating site (v%s) from remote repositories: %s into %s", version, location, *outDir)
	return cfg.cmdSiteRemote(location, *outDir, recentDuration, recentDurationStr, *fastGit, *useZip, *concurrency, *retries, *continueOnError, *persistentDir, *reposConfOpt, *tempDir, *workMode, *mode, *profileSiteGen, *profileOut, *smartMode)
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

func getHighestVersionsAndCount(versions []g2.VersionData, site *g2.SiteData) ([]g2.VersionGroup, []g2.VersionGroup, string, int) {
	// Parse KEYWORDS and group versions

	stableMap := make(map[string]string)
	testingMap := make(map[string]string)

	var snapshot string

	for _, ver := range versions {
		if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
			continue
		}

		if strings.Contains(ver.Version, "9999") {
			if snapshot == "" || g2.CompareVersions(ver.Version, snapshot) > 0 {
				snapshot = ver.Version
			}
			continue
		}

		keywords := ver.Ebuild.Vars["KEYWORDS"]
		parts := strings.Fields(keywords)

		for _, p := range parts {
			if strings.HasPrefix(p, "-") {
				continue
			}
			if strings.HasPrefix(p, "~") {
				arch := p[1:]
				if current, ok := testingMap[arch]; !ok || g2.CompareVersions(ver.Version, current) > 0 {
					testingMap[arch] = ver.Version
				}
			} else {
				arch := p
				if current, ok := stableMap[arch]; !ok || g2.CompareVersions(ver.Version, current) > 0 {
					stableMap[arch] = ver.Version
				}
			}
		}
	}

	// Group archs by version
	stableGroup := make(map[string][]string)
	for arch, ver := range stableMap {
		stableGroup[ver] = append(stableGroup[ver], arch)
	}

	testingGroup := make(map[string][]string)
	for arch, ver := range testingMap {
		testingGroup[ver] = append(testingGroup[ver], arch)
	}

	formatGroups := func(groups map[string][]string) []g2.VersionGroup {
		if len(groups) == 0 {
			return nil
		}

		var sortedVersions []string
		for ver := range groups {
			sortedVersions = append(sortedVersions, ver)
		}

		// Sort descending
		for i := 0; i < len(sortedVersions); i++ {
			for j := i + 1; j < len(sortedVersions); j++ {
				if g2.CompareVersions(sortedVersions[i], sortedVersions[j]) < 0 {
					sortedVersions[i], sortedVersions[j] = sortedVersions[j], sortedVersions[i]
				}
			}
		}

		var result []g2.VersionGroup
		for _, ver := range sortedVersions {
			archs := groups[ver]

			// sort archs
			sort.Strings(archs)

			result = append(result, g2.VersionGroup{
				Version: ver,
				Archs:   strings.Join(archs, " "),
			})
		}

		return result
	}

	return formatGroups(stableGroup), formatGroups(testingGroup), snapshot, len(versions)
}

type ResolvedDepNode struct {
	Type      string            `json:"type"`
	Name      string            `json:"name,omitempty"`
	Link      string            `json:"link,omitempty"`
	Flag      string            `json:"flag,omitempty"`
	IsNegated bool              `json:"is_negated,omitempty"`
	Children  []ResolvedDepNode `json:"children,omitempty"`
}

func resolveDependencies(node g2.DepNode, pkgMap map[string]bool) ResolvedDepNode {
	switch n := node.(type) {
	case g2.DepString:
		raw := string(n)
		pkgName := g2.ExtractPackageNameFromDep(raw)
		link := ""

		if pkgName != "" {
			if pkgMap[pkgName] {
				parts := strings.SplitN(pkgName, "/", 2)
				if len(parts) == 2 {
					link = "../../../../../../categories/" + parts[0] + "/packages/" + parts[1] + "/"
				}
			}
		}

		return ResolvedDepNode{
			Type: "string",
			Name: raw,
			Link: link,
		}

	case g2.DepAnyOf:
		res := ResolvedDepNode{Type: "any_of"}
		for _, child := range n.Children {
			res.Children = append(res.Children, resolveDependencies(child, pkgMap))
		}
		return res

	case g2.DepAllOf:
		res := ResolvedDepNode{Type: "all_of"}
		for _, child := range n.Children {
			res.Children = append(res.Children, resolveDependencies(child, pkgMap))
		}
		return res

	case g2.DepUseConditional:
		res := ResolvedDepNode{
			Type:      "use_conditional",
			Flag:      n.Flag,
			IsNegated: n.IsNegated,
		}
		for _, child := range n.Children {
			res.Children = append(res.Children, resolveDependencies(child, pkgMap))
		}
		return res
	}
	return ResolvedDepNode{}
}

// isValidLicense checks if a license string contains at least one alphanumeric character
func isValidLicense(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return true
		}
	}
	return false
}

// sanitizeFilename restricts a string to characters safe for file and directory names
// and limits its length to prevent "file name too long" errors.
func sanitizeFilename(s string) string {
	if len(s) > 250 {
		log.Printf("Warning: name %q is too long, truncating to 250 characters", s)
		s = s[:250]
	}
	var sb strings.Builder
	for _, r := range s {
		if r == '\x00' {
			log.Printf("Warning: name %q contains null bytes, stripping them", s)
			continue
		}
		// Replace characters that are invalid or problematic in file names and URLs
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == ' ' {
			sb.WriteRune('_')
		} else {
			sb.WriteRune(r)
		}
	}
	res := sb.String()
	if res != s {
		log.Printf("Warning: name %q was sanitized to %q", s, res)
	}
	return res
}

func parseRepo(sysFS fs.FS, repoDir string, defaultTitle string, fastGit bool, repoInfo *g2.Repository, opts ...any) (*g2.SiteData, error) {
	title := defaultTitle
	var repoName string
	var remoteURL string

	for _, opt := range opts {
		switch o := opt.(type) {
		case SourceURL:
			remoteURL = string(o)
		}
	}

	remoteURL = resolveRemoteURL(repoDir, repoInfo, remoteURL)

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

	site := &g2.SiteData{
		Title:           title,
		RepoName:        repoName,
		RemoteURL:       remoteURL,
		SourceURL:       remoteURL,
		Repository:      repoInfo,
		EAPI:            eapi,
		ValidUseExpands: make(map[string]bool),
		PackageCount:    0,
	}

	parseRepoProfilesDir(sysFS, repoDir, site)

	if site.UseExpandDescs != nil {
		for prefix := range site.UseExpandDescs {
			site.ValidUseExpands[prefix] = true
		}
	}

	parseRepoNews(sysFS, repoDir, site)
	parseRepoAuthors(repoDir, site, remoteURL)

	var profilesDescEntries []g2.ProfileDescEntry
	profilesDescBytes, err := fs.ReadFile(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "profiles.desc")))
	if err == nil {
		profilesDescEntries = parseProfilesDesc(string(profilesDescBytes))
	}

	profilesData, err := parseProfilesDirFS(sysFS, repoDir, profilesDescEntries)
	if err != nil {
		log.Printf("Warning: failed to parse profiles dir: %v", err)
	}
	site.Profiles = profilesData

	eclassDir := filepath.Join(repoDir, "eclass")
	eclassEntries, err := fs.ReadDir(sysFS, filepath.ToSlash(eclassDir))
	if err == nil {
		for _, eclassEntry := range eclassEntries {
			if !eclassEntry.IsDir() && strings.HasSuffix(eclassEntry.Name(), ".eclass") {
				eclassName := strings.TrimSuffix(eclassEntry.Name(), ".eclass")
				site.DefinedEclasses = append(site.DefinedEclasses, g2.EclassData{
					Name: eclassName,
				})
			}
		}
		sort.Slice(site.DefinedEclasses, func(i, j int) bool {
			return site.DefinedEclasses[i].Name < site.DefinedEclasses[j].Name
		})
	}

	updates, err := g2.ParseUpdatesDirFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "updates")))
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse updates: %v", err)
	}
	if updates != nil {
		site.Moves = updates.Moves
		site.SlotMoves = updates.SlotMoves
	}

	pf, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "metadata", "projects.xml")))
	if err != nil {
		if fastGit {
			pf, err = os.Open(filepath.Join(repoDir, "metadata", "projects.xml"))
		}
	}
	if err == nil {
		if projects, err := g2.ParseProjectsFromReader(pf); err == nil {
			site.Projects = projects
		} else {
			log.Printf("Warning: failed to parse projects.xml: %v", err)
		}
		_ = pf.Close()
	}

	if err := parseRepoCategoriesAndPackages(sysFS, repoDir, repoName, fastGit, remoteURL, site); err != nil {
		return nil, err
	}

	extractVirtualDeps(site)
	parseRepoEclasses(sysFS, repoDir, site)

	count := 0
	for _, cat := range site.Categories {
		count += len(cat.Packages)
	}
	site.PackageCount = count

	return site, nil
}

func extractVirtualDeps(site *g2.SiteData) {
	pkgMap := make(map[string]*g2.PackageData)
	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			key := site.Categories[i].Name + "/" + site.Categories[i].Packages[j].Name
			pkgMap[key] = &site.Categories[i].Packages[j]
		}
	}

	for i := range site.Categories {
		if site.Categories[i].Name != "virtual" && !strings.HasPrefix(site.Categories[i].Name, "virtual-") {
			continue
		}
		for j := range site.Categories[i].Packages {
			pkg := &site.Categories[i].Packages[j]
			depsMap := make(map[string]bool)
			for _, ver := range pkg.Versions {
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					types := []string{"DEPEND", "RDEPEND", "PDEPEND"}
					for _, depType := range types {
						depStr := ver.Ebuild.Vars[depType]
						if depStr != "" {
							tree := g2.ParseDepTree(depStr)
							extractDepNodes(tree.Nodes, depsMap)
						}
					}
				}
			}
			for dep := range depsMap {
				pkg.VirtualDeps = append(pkg.VirtualDeps, dep)
				if targetPkg, ok := pkgMap[dep]; ok {
					virtualName := pkg.Category + "/" + pkg.Name
					found := false
					for _, v := range targetPkg.ReverseVirtuals {
						if v == virtualName {
							found = true
							break
						}
					}
					if !found {
						targetPkg.ReverseVirtuals = append(targetPkg.ReverseVirtuals, virtualName)
					}
				}
			}
			sort.Strings(pkg.VirtualDeps)

			// Compute Equivalents for each package in VirtualDeps
			for _, dep := range pkg.VirtualDeps {
				if targetPkg, ok := pkgMap[dep]; ok {
					for _, otherDep := range pkg.VirtualDeps {
						if otherDep != dep {
							found := false
							for _, e := range targetPkg.Equivalents {
								if e == otherDep {
									found = true
									break
								}
							}
							if !found {
								targetPkg.Equivalents = append(targetPkg.Equivalents, otherDep)
							}
						}
					}
				}
			}
		}
	}
}

func extractDepNodes(nodes []g2.DepNode, depsMap map[string]bool) {
	for _, node := range nodes {
		switch n := node.(type) {
		case g2.DepString:
			pkgName := g2.ExtractPackageNameFromDep(string(n))
			if pkgName != "" {
				depsMap[pkgName] = true
			}
		case g2.DepAnyOf:
			extractDepNodes(n.Children, depsMap)
		case g2.DepAllOf:
			extractDepNodes(n.Children, depsMap)
		case g2.DepUseConditional:
			extractDepNodes(n.Children, depsMap)
		}
	}
}

func buildManifestData(manifest *g2.Manifest, versions []g2.VersionData, thirdPartyMirrors map[string][]string) []g2.ManifestEntryData {
	var manifestData []g2.ManifestEntryData
	for _, entry := range manifest.Entries {
		md := g2.ManifestEntryData{
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

						if strings.HasPrefix(uri.URL, "mirror://") {
							parts := strings.SplitN(uri.URL[len("mirror://"):], "/", 2)
							if len(parts) == 2 {
								mirrorName := parts[0]
								filePath := parts[1]
								if mirrors, ok := thirdPartyMirrors[mirrorName]; ok {
									for _, mirrorURL := range mirrors {
										var resolvedURL string
										if strings.HasSuffix(mirrorURL, "/") {
											resolvedURL = mirrorURL + filePath
										} else {
											resolvedURL = mirrorURL + "/" + filePath
										}
										md.ResolvedURLs = append(md.ResolvedURLs, resolvedURL)
									}
								} else {
									md.ResolvedURLs = append(md.ResolvedURLs, uri.URL)
								}
							} else {
								md.ResolvedURLs = append(md.ResolvedURLs, uri.URL)
							}
						} else {
							md.ResolvedURLs = append(md.ResolvedURLs, uri.URL)
						}
					}
				}
			}
		}
		// Sort versions descending
		sort.Slice(md.Versions, func(i, j int) bool {
			return md.Versions[i] > md.Versions[j]
		})
		sort.Strings(md.URLs)
		sort.Strings(md.ResolvedURLs)

		manifestData = append(manifestData, md)
	}
	return manifestData
}

func parseProfilesDir(repoDir string, entries []g2.ProfileDescEntry) ([]g2.ProfileData, error) {
	return parseProfilesDirFS(os.DirFS(repoDir), ".", entries)
}

func parseProfilesDirFS(sysFS fs.FS, repoDir string, entries []g2.ProfileDescEntry) ([]g2.ProfileData, error) {
	profilesDir := path2.Join(repoDir, "profiles")

	if info, err := fs.Stat(sysFS, profilesDir); err != nil || !info.IsDir() {
		return nil, nil
	}

	descMap := make(map[string]g2.ProfileDescEntry)
	for _, e := range entries {
		descMap[e.Path] = e
	}

	profilesMap := make(map[string]*g2.ProfileData)

	err := fs.WalkDir(sysFS, profilesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		relPath := strings.TrimPrefix(path, profilesDir)
		relPath = strings.TrimPrefix(relPath, "/")
		if relPath == "" || relPath == "." {
			return nil
		}

		pData := &g2.ProfileData{
			Path:  relPath,
			Files: make(map[string]string),
		}

		if desc, ok := descMap[relPath]; ok {
			pData.IsDesc = true
			pData.DescArch = desc.Arch
			pData.DescStat = desc.Status
		}

		// Read commonly known files
		fileNames := []string{
			"parent", "eapi", "make.defaults", "package.mask", "package.use",
			"package.use.force", "package.use.mask", "package.use.stable.force",
			"package.use.stable.mask", "packages", "use.force", "use.mask",
			"use.stable.force", "use.stable.mask",
		}

		var g errgroup.Group
		var mu sync.Mutex
		for _, fname := range fileNames {
			fname := fname
			g.Go(func() error {
				b, err := fs.ReadFile(sysFS, path2.Join(path, fname))
				if err == nil {
					content := string(b)
					var parents []string
					if fname == "parent" {
						for _, line := range strings.Split(content, "\n") {
							line = strings.TrimSpace(line)
							if line == "" || strings.HasPrefix(line, "#") {
								continue
							}
							parentRelPath := path2.Clean(path2.Join(relPath, line))
							if !strings.HasPrefix(parentRelPath, "..") {
								parents = append(parents, parentRelPath)
							}
						}
					}
					mu.Lock()
					pData.Files[fname] = content
					pData.Parents = append(pData.Parents, parents...)
					mu.Unlock()
				}
				return nil
			})
		}
		_ = g.Wait()

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

	var result []g2.ProfileData
	for _, pData := range profilesMap {
		result = append(result, *pData)
	}

	return result, nil
}

func parseProfilesDesc(content string) []g2.ProfileDescEntry {
	var entries []g2.ProfileDescEntry
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			entries = append(entries, g2.ProfileDescEntry{
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

type AggCategory struct {
	Name     string
	Packages []*AggPackage
}
type AggPackage struct {
	Name                string
	Category            string
	Repos               map[string]*g2.SiteData
	DominantDescription string
	DominantHomepage    string
	DominantLicense     string
	ReverseVirtuals     []string
	VirtualDeps         []string
}

func (a *AggPackage) ReposList() []*g2.SiteData {
	return mapToList(a.Repos)
}

type AggProject struct {
	ReposList []*g2.SiteData
	Project   *g2.Project
	Packages  []*AggPackage
}

type AggLicense struct {
	Name            string
	Count           int
	Packages        []*AggPackage
	Text            string
	Aliases         []string
	ProvidedByRepos map[string]*g2.SiteData
	UsedByRepos     map[string]*g2.SiteData
}

type AggUseFlag struct {
	Name          string
	Count         int
	GlobalDesc    string
	LocalDescs    map[string]string
	MetadataDescs map[string]string
	Packages      []*AggPackage
	Warnings      []string
}

func getRepoEclasses(site *g2.SiteData, aggPackages map[string]*AggPackage) []*AggEclass {
	eclassMap := make(map[string]*AggEclass)
	seenPackages := make(map[string]map[string]bool)

	for _, eclass := range site.DefinedEclasses {
		if _, ok := eclassMap[eclass.Name]; !ok {
			eclassMap[eclass.Name] = &AggEclass{
				Name:  eclass.Name,
				Repos: make(map[string]*g2.SiteData),
			}
			seenPackages[eclass.Name] = make(map[string]bool)
		}
		eclassMap[eclass.Name].Repos[site.RepoName] = site
	}

	for _, cat := range site.Categories {
		for _, pkg := range cat.Packages {
			pkgKey := cat.Name + "/" + pkg.Name

			for _, ver := range pkg.Versions {
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					inherited := ver.Ebuild.Vars["INHERITED"]
					if inherited != "" {
						eclasses := strings.Fields(inherited)
						for _, ec := range eclasses {
							if _, ok := eclassMap[ec]; !ok {
								eclassMap[ec] = &AggEclass{
									Name:  ec,
									Repos: make(map[string]*g2.SiteData),
								}
								eclassMap[ec].Repos[site.RepoName] = site
								seenPackages[ec] = make(map[string]bool)
							}

							if !seenPackages[ec][pkgKey] {
								eclassMap[ec].Packages = append(eclassMap[ec].Packages, aggPackages[pkgKey])
								seenPackages[ec][pkgKey] = true
							}
						}
					}
				}
			}
		}
	}

	var sortedEclasses []*AggEclass
	for _, ec := range eclassMap {
		sort.Slice(ec.Packages, func(i, j int) bool {
			if ec.Packages[i].Category == ec.Packages[j].Category {
				return ec.Packages[i].Name < ec.Packages[j].Name
			}
			return ec.Packages[i].Category < ec.Packages[j].Category
		})
		sortedEclasses = append(sortedEclasses, ec)
	}
	sort.Slice(sortedEclasses, func(i, j int) bool { return sortedEclasses[i].Name < sortedEclasses[j].Name })

	return sortedEclasses
}

func getRepoLicenses(site *g2.SiteData, aggPackages map[string]*AggPackage) []*AggLicense {
	aggLicenses := make(map[string]*AggLicense)

	for _, providedLic := range site.ProvidedLicenses {
		if _, ok := aggLicenses[providedLic]; !ok {
			aggLicenses[providedLic] = &AggLicense{
				Name:            providedLic,
				ProvidedByRepos: map[string]*g2.SiteData{site.RepoName: site},
				UsedByRepos:     make(map[string]*g2.SiteData),
			}
		}
	}

	for _, cat := range site.Categories {
		for _, pkg := range cat.Packages {
			pkgKey := pkg.Category + "/" + pkg.Name

			for _, ver := range pkg.Versions {
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					licenseStr := ver.Ebuild.Vars["LICENSE"]
					licenses := g2.ParseLicense(licenseStr)

					for _, lic := range licenses {
						if lic != "" {
							if !isValidLicense(lic) {
								continue
							}
							if _, ok := aggLicenses[lic]; !ok {
								aggLicenses[lic] = &AggLicense{
									Name:            lic,
									ProvidedByRepos: make(map[string]*g2.SiteData),
									UsedByRepos:     make(map[string]*g2.SiteData),
								}
							}

							aggLicenses[lic].UsedByRepos[site.RepoName] = site

							found := false
							for _, p := range aggLicenses[lic].Packages {
								if p.Name == pkg.Name && p.Category == pkg.Category {
									found = true
									break
								}
							}
							if !found {
								if aggPkg, ok := aggPackages[pkgKey]; ok {
									aggLicenses[lic].Packages = append(aggLicenses[lic].Packages, aggPkg)
									aggLicenses[lic].Count++
								}
							}

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
	}

	var sortedLicenses []*AggLicense
	for _, l := range aggLicenses {
		sort.Strings(l.Aliases)
		sortedLicenses = append(sortedLicenses, l)
	}
	sort.Slice(sortedLicenses, func(i, j int) bool { return sortedLicenses[i].Name < sortedLicenses[j].Name })

	site.AggLicenses = sortedLicenses
	return sortedLicenses
}

func getRepoUseFlags(site *g2.SiteData, aggPackages map[string]*AggPackage) []*AggUseFlag {
	aggUseFlags := make(map[string]*AggUseFlag)

	for _, cat := range site.Categories {
		for _, pkg := range cat.Packages {
			pkgKey := pkg.Category + "/" + pkg.Name

			for _, ver := range pkg.Versions {
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					iuse := ver.Ebuild.Vars["IUSE"]
					if iuse != "" {
						parsedFlags := parseIUSEFlagsFunc(iuse)
						for _, flagObj := range parsedFlags {
							flag := flagObj.Name
							if _, ok := aggUseFlags[flag]; !ok {
								aggUseFlags[flag] = &AggUseFlag{
									Name:          flag,
									LocalDescs:    make(map[string]string),
									MetadataDescs: make(map[string]string),
								}
							}

							foundPkg := false
							for _, p := range aggUseFlags[flag].Packages {
								if p.Name == pkg.Name && p.Category == pkg.Category {
									foundPkg = true
									break
								}
							}
							if !foundPkg {
								aggUseFlags[flag].Packages = append(aggUseFlags[flag].Packages, aggPackages[pkgKey])
								aggUseFlags[flag].Count++
							}
						}
					}

					requiredUse := ver.Ebuild.Vars["REQUIRED_USE"]
					if requiredUse != "" {
						parsedFlags := parseIUSEFlagsFunc(requiredUse)
						for _, flagObj := range parsedFlags {
							flag := flagObj.Name
							if flag == "(" || flag == ")" || flag == "||" || flag == "^^" || flag == "??" || strings.HasSuffix(flag, "?") {
								continue
							}
							flag = strings.TrimPrefix(flag, "!") // remove negations

							if _, ok := aggUseFlags[flag]; !ok {
								aggUseFlags[flag] = &AggUseFlag{
									Name:          flag,
									LocalDescs:    make(map[string]string),
									MetadataDescs: make(map[string]string),
								}
							}

							foundPkg := false
							for _, p := range aggUseFlags[flag].Packages {
								if p.Name == pkg.Name && p.Category == pkg.Category {
									foundPkg = true
									break
								}
							}
							if !foundPkg {
								aggUseFlags[flag].Packages = append(aggUseFlags[flag].Packages, aggPackages[pkgKey])
								aggUseFlags[flag].Count++
							}
						}
					}
				}
			}

			if pkg.Metadata != nil {
				for _, useBlock := range pkg.Metadata.Use {
					for _, flag := range useBlock.Flags {
						if _, ok := aggUseFlags[flag.Name]; !ok {
							aggUseFlags[flag.Name] = &AggUseFlag{
								Name:          flag.Name,
								LocalDescs:    make(map[string]string),
								MetadataDescs: make(map[string]string),
							}
						}

						foundPkg := false
						for _, p := range aggUseFlags[flag.Name].Packages {
							if p.Name == pkg.Name && p.Category == pkg.Category {
								foundPkg = true
								break
							}
						}
						if !foundPkg {
							aggUseFlags[flag.Name].Packages = append(aggUseFlags[flag.Name].Packages, aggPackages[pkgKey])
							aggUseFlags[flag.Name].Count++
						}

						if flag.Text != "" {
							aggUseFlags[flag.Name].MetadataDescs[pkgKey] = flag.Text
						}
					}
				}
			}
		}
	}

	if site.UseDesc != nil {
		for flag, desc := range site.UseDesc.Flags {
			if _, ok := aggUseFlags[flag]; !ok {
				aggUseFlags[flag] = &AggUseFlag{
					Name:          flag,
					LocalDescs:    make(map[string]string),
					MetadataDescs: make(map[string]string),
				}
			}
			aggUseFlags[flag].GlobalDesc = desc
		}
	}

	// Integrate USE_EXPAND descriptions
	if site.UseExpandDescs != nil {
		for prefix, desc := range site.UseExpandDescs {
			for suffix, text := range desc.Flags {
				flagName := prefix + "_" + suffix
				if aggFlag, ok := aggUseFlags[flagName]; ok {
					if aggFlag.GlobalDesc == "" {
						aggFlag.GlobalDesc = text
					}
				}
			}
		}
	}
	if site.UseLocalDesc != nil {
		for pkg, flags := range site.UseLocalDesc.Flags {
			for flag, desc := range flags {
				if aggFlag, ok := aggUseFlags[flag]; ok {
					aggFlag.LocalDescs[pkg] = desc
				}
			}
		}
	}

	var sortedUseFlags []*AggUseFlag
	for _, flag := range aggUseFlags {
		for _, pkg := range flag.Packages {
			if pkg == nil {
				continue
			}
			pkgKey := pkg.Category + "/" + pkg.Name
			hasLocal := flag.LocalDescs[pkgKey] != ""
			hasMetadata := flag.MetadataDescs[pkgKey] != ""

			if !hasMetadata && !hasLocal && flag.GlobalDesc == "" {
				flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' used by %s but has no description in metadata.xml, use.local.desc or use.desc", flag.Name, pkgKey))
			} else if !hasMetadata && flag.GlobalDesc == "" {
				flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' used by %s but not documented in its metadata.xml", flag.Name, pkgKey))
			}
		}
		sortedUseFlags = append(sortedUseFlags, flag)
	}
	sort.Slice(sortedUseFlags, func(i, j int) bool { return sortedUseFlags[i].Name < sortedUseFlags[j].Name })

	return sortedUseFlags
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

type AggEclass struct {
	Name     string
	Repos    map[string]*g2.SiteData
	Packages []*AggPackage
}

type AggPackageMove struct {
	Old string
	New string
}

type AggNewsItem struct {
	g2.NewsItem
	RepoName    string
	ArchivePath string
}

type AggArch struct {
	Name   string
	Status string
	Repos  []*g2.SiteData
}

type RepoGroup struct {
	Quality string
	Status  string
	Repos   []*g2.SiteData
}

type AggregatedData struct {
	Categories      []*AggCategory
	Packages        []*AggPackage
	Licenses        []*AggLicense
	Projects        []*AggProject
	Profiles        []*g2.AggProfile
	Arches          []*AggArch
	Moves           map[string]*AggPackageMove
	GlobalNews      []AggNewsItem
	RecentNews      []AggNewsItem
	TotalPackages   int
	UseFlags        []*AggUseFlag
	Eclasses        []*AggEclass
	UseExpandDescs  map[string]*g2.UseExpandDesc
	ValidLicenses   map[string]bool
	ValidUseExpands map[string]bool
	GroupedRepos    []RepoGroup
}

func aggregateGroupedRepos(sites []*g2.SiteData) map[string]*RepoGroup {
	groupedReposMap := make(map[string]*RepoGroup)
	for _, site := range sites {
		quality := "experimental"
		status := "unofficial"
		if site.Repository != nil {
			if site.Repository.Quality != "" {
				quality = site.Repository.Quality
			}
			if site.Repository.Status != "" {
				status = site.Repository.Status
			}
		}

		groupKey := quality + "|" + status
		if _, ok := groupedReposMap[groupKey]; !ok {
			groupedReposMap[groupKey] = &RepoGroup{
				Quality: quality,
				Status:  status,
			}
		}
		groupedReposMap[groupKey].Repos = append(groupedReposMap[groupKey].Repos, site)
	}
	return groupedReposMap
}

func aggregateUseExpandDescs(sites []*g2.SiteData) map[string]*g2.UseExpandDesc {
	aggUseExpandDescs := make(map[string]*g2.UseExpandDesc)
	for _, site := range sites {
		if site.UseExpandDescs != nil {
			for prefix, desc := range site.UseExpandDescs {
				if _, ok := aggUseExpandDescs[prefix]; !ok {
					aggUseExpandDescs[prefix] = desc
				}
			}
		}
	}
	return aggUseExpandDescs
}

func aggregateProjects(sites []*g2.SiteData) map[string]*AggProject {
	aggProjects := make(map[string]*AggProject)

	// First pass: collect projects and count occurrences by email across distinct repos
	emailRepoCount := make(map[string]map[string]bool)
	for _, site := range sites {
		if site.Projects != nil {
			for i := range site.Projects.Projects {
				proj := &site.Projects.Projects[i]
				if emailRepoCount[proj.Email] == nil {
					emailRepoCount[proj.Email] = make(map[string]bool)
				}
				emailRepoCount[proj.Email][site.RepoName] = true
			}
		}
	}

	// Second pass: construct keys correctly
	for _, site := range sites {
		if site.Projects != nil {
			for i := range site.Projects.Projects {
				proj := &site.Projects.Projects[i]
				key := proj.Email

				// If the same project email appears in more than 1 distinct repo, disambiguate
				if len(emailRepoCount[proj.Email]) > 1 {
					key = proj.Email + "-" + site.RepoName
					projCopy := *proj
					projCopy.Name = projCopy.Name + " (" + site.RepoName + ")"
					projCopy.Email = key
					proj = &projCopy
				}

				if _, ok := aggProjects[key]; !ok {
					aggProjects[key] = &AggProject{Project: proj, ReposList: []*g2.SiteData{site}}
				}
			}
		}
	}
	return aggProjects
}

func aggregateProfiles(sites []*g2.SiteData) map[string]*g2.AggProfile {
	aggProfiles := make(map[string]*g2.AggProfile)
	for _, site := range sites {
		for _, p := range site.Profiles {
			if _, ok := aggProfiles[p.Path]; !ok {
				aggProfiles[p.Path] = &g2.AggProfile{
					Path: p.Path,
				}
			}
			aggProfiles[p.Path].Repos = append(aggProfiles[p.Path].Repos, g2.AggProfileRepo{
				RepoName: site.RepoName,
				Profile:  p,
			})
			if p.IsDesc {
				aggProfiles[p.Path].IsDesc = true
				aggProfiles[p.Path].DescArch = p.DescArch
				aggProfiles[p.Path].DescStat = p.DescStat
			}
		}
	}
	return aggProfiles
}

func aggregateGlobalNews(sites []*g2.SiteData) []AggNewsItem {
	var globalNews []AggNewsItem
	for _, site := range sites {
		for _, news := range site.News {
			globalNews = append(globalNews, AggNewsItem{
				NewsItem:    news,
				RepoName:    site.RepoName,
				ArchivePath: path2.Join(site.RepoName, news.DirName),
			})
		}
	}
	return globalNews
}

func aggregateArches(sites []*g2.SiteData) map[string]*AggArch {
	aggArches := make(map[string]*AggArch)
	for _, site := range sites {
		if site.ArchList != nil {
			for _, arch := range site.ArchList.Arches {
				if _, ok := aggArches[arch]; !ok {
					aggArches[arch] = &AggArch{Name: arch}
				}
				aggArches[arch].Repos = append(aggArches[arch].Repos, site)
			}
		}
		if site.ArchesDesc != nil {
			for arch, status := range site.ArchesDesc.Arches {
				if _, ok := aggArches[arch]; !ok {
					aggArches[arch] = &AggArch{Name: arch}
				}
				if aggArches[arch].Status == "" {
					aggArches[arch].Status = status
				}

				found := false
				for _, r := range aggArches[arch].Repos {
					if r.RepoName == site.RepoName {
						found = true
						break
					}
				}
				if !found {
					aggArches[arch].Repos = append(aggArches[arch].Repos, site)
				}
			}
		}
	}
	return aggArches
}

func aggregateMoves(sites []*g2.SiteData) map[string]*AggPackageMove {
	aggMoves := make(map[string]*AggPackageMove)
	for _, site := range sites {
		for _, move := range site.Moves {
			if _, ok := aggMoves[move.Old]; !ok {
				aggMoves[move.Old] = &AggPackageMove{Old: move.Old, New: move.New}
			}
		}
	}
	return aggMoves
}

func aggregateEclasses(sites []*g2.SiteData) map[string]*AggEclass {
	aggEclasses := make(map[string]*AggEclass)
	for _, site := range sites {
		for _, eclass := range site.AggEclasses.([]*AggEclass) {
			if _, ok := aggEclasses[eclass.Name]; !ok {
				aggEclasses[eclass.Name] = &AggEclass{
					Name:  eclass.Name,
					Repos: make(map[string]*g2.SiteData),
				}
			}
			for rName, rData := range eclass.Repos {
				aggEclasses[eclass.Name].Repos[rName] = rData
			}
			for _, pkg := range eclass.Packages {
				foundPkg := false
				for _, existingPkg := range aggEclasses[eclass.Name].Packages {
					if existingPkg.Name == pkg.Name && existingPkg.Category == pkg.Category {
						newRepos := make(map[string]*g2.SiteData)
						for k, v := range existingPkg.Repos {
							newRepos[k] = v
						}
						for rName, rData := range pkg.Repos {
							newRepos[rName] = rData
						}
						existingPkg.Repos = newRepos
						foundPkg = true
						break
					}
				}
				if !foundPkg {
					newPkg := *pkg
					newRepos := make(map[string]*g2.SiteData)
					for k, v := range pkg.Repos {
						newRepos[k] = v
					}
					newPkg.Repos = newRepos
					aggEclasses[eclass.Name].Packages = append(aggEclasses[eclass.Name].Packages, &newPkg)
				}
			}
		}
	}
	return aggEclasses
}

func aggregatePackagesAndCategories(sites []*g2.SiteData, aggProjects map[string]*AggProject) (map[string]*AggCategory, map[string]*AggPackage, map[string]*AggLicense, int) {
	aggCategories := make(map[string]*AggCategory)
	aggPackages := make(map[string]*AggPackage)
	aggLicenses := make(map[string]*AggLicense)
	totalPackages := 0

	catPkgMap := make(map[string]map[string]*AggPackage)

	for _, site := range sites {
		for _, providedLic := range site.ProvidedLicenses {
			if _, ok := aggLicenses[providedLic]; !ok {
				aggLicenses[providedLic] = &AggLicense{
					Name:            providedLic,
					ProvidedByRepos: make(map[string]*g2.SiteData),
					UsedByRepos:     make(map[string]*g2.SiteData),
				}
			}
			aggLicenses[providedLic].ProvidedByRepos[site.RepoName] = site
		}

		for _, cat := range site.Categories {
			if _, ok := aggCategories[cat.Name]; !ok {
				aggCategories[cat.Name] = &AggCategory{Name: cat.Name}
				catPkgMap[cat.Name] = make(map[string]*AggPackage)
			}
			for _, pkg := range cat.Packages {
				pkgKey := cat.Name + "/" + pkg.Name
				if _, ok := aggPackages[pkgKey]; !ok {
					aggPackages[pkgKey] = &AggPackage{Name: pkg.Name, Category: cat.Name, Repos: make(map[string]*g2.SiteData)}
					totalPackages++
				}
				aggPackages[pkgKey].Repos[site.RepoName] = site
				if aggPackages[pkgKey].DominantDescription == "" {
					aggPackages[pkgKey].DominantDescription = pkg.DominantDescription
				}
				if aggPackages[pkgKey].DominantHomepage == "" {
					aggPackages[pkgKey].DominantHomepage = pkg.DominantHomepage
				}
				if aggPackages[pkgKey].DominantLicense == "" {
					aggPackages[pkgKey].DominantLicense = pkg.DominantLicense
				}
				catPkgMap[cat.Name][pkg.Name] = aggPackages[pkgKey]
				for _, rev := range pkg.ReverseVirtuals {
					found := false
					for _, existingRev := range aggPackages[pkgKey].ReverseVirtuals {
						if existingRev == rev {
							found = true
							break
						}
					}
					if !found {
						aggPackages[pkgKey].ReverseVirtuals = append(aggPackages[pkgKey].ReverseVirtuals, rev)
					}
				}
				sort.Strings(aggPackages[pkgKey].ReverseVirtuals)

				for _, dep := range pkg.VirtualDeps {
					found := false
					for _, existingDep := range aggPackages[pkgKey].VirtualDeps {
						if existingDep == dep {
							found = true
							break
						}
					}
					if !found {
						aggPackages[pkgKey].VirtualDeps = append(aggPackages[pkgKey].VirtualDeps, dep)
					}
				}
				sort.Strings(aggPackages[pkgKey].VirtualDeps)

				if pkg.Metadata != nil {
					for _, maint := range pkg.Metadata.Maintainers {
						if proj, ok := aggProjects[maint.Email]; ok {
							found := false
							for _, p := range proj.Packages {
								if p.Name == pkg.Name && p.Category == pkg.Category {
									found = true
									break
								}
							}
							if !found {
								proj.Packages = append(proj.Packages, aggPackages[pkgKey])
							}
						}
					}
				}

				for _, ver := range pkg.Versions {
					if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
						licenseStr := ver.Ebuild.Vars["LICENSE"]
						licenses := g2.ParseLicense(licenseStr)

						for _, lic := range licenses {
							if lic != "" {
								if !isValidLicense(lic) {
									log.Printf("Warning: Invalid license skipped: %q in package %s", lic, pkgKey)
									continue
								}
								if _, ok := aggLicenses[lic]; !ok {
									aggLicenses[lic] = &AggLicense{
										Name:            lic,
										ProvidedByRepos: make(map[string]*g2.SiteData),
										UsedByRepos:     make(map[string]*g2.SiteData),
									}
								}

								aggLicenses[lic].UsedByRepos[site.RepoName] = site

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
		}
	}

	for catName, pkgs := range catPkgMap {
		var sortedPkgs []*AggPackage
		for _, p := range pkgs {
			sortedPkgs = append(sortedPkgs, p)
		}
		sort.Slice(sortedPkgs, func(i, j int) bool { return sortedPkgs[i].Name < sortedPkgs[j].Name })
		aggCategories[catName].Packages = sortedPkgs
	}

	return aggCategories, aggPackages, aggLicenses, totalPackages
}

func sortCategories(aggCategories map[string]*AggCategory) []*AggCategory {
	var sortedCategories []*AggCategory
	for _, c := range aggCategories {
		sortedCategories = append(sortedCategories, c)
	}
	sort.Slice(sortedCategories, func(i, j int) bool { return sortedCategories[i].Name < sortedCategories[j].Name })
	return sortedCategories
}

func sortPackages(aggPackages map[string]*AggPackage) []*AggPackage {
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
	return sortedPackages
}

func aggregateValidUseExpands(sites []*g2.SiteData) map[string]bool {
	validUseExpands := make(map[string]bool)
	for _, site := range sites {
		for prefix := range site.ValidUseExpands {
			validUseExpands[prefix] = true
		}
	}
	return validUseExpands
}

func sortLicenses(aggLicenses map[string]*AggLicense) ([]*AggLicense, map[string]bool) {
	validLicenses := make(map[string]bool)
	var sortedLicenses []*AggLicense
	for _, l := range aggLicenses {
		sort.Strings(l.Aliases)
		sortedLicenses = append(sortedLicenses, l)
		validLicenses[l.Name] = true
	}
	sort.Slice(sortedLicenses, func(i, j int) bool { return sortedLicenses[i].Name < sortedLicenses[j].Name })
	return sortedLicenses, validLicenses
}

func sortProjects(aggProjects map[string]*AggProject) []*AggProject {
	var sortedProjects []*AggProject
	for _, p := range aggProjects {
		sortedProjects = append(sortedProjects, p)
	}
	sort.Slice(sortedProjects, func(i, j int) bool { return sortedProjects[i].Project.Name < sortedProjects[j].Project.Name })
	return sortedProjects
}

func sortProfiles(aggProfiles map[string]*g2.AggProfile) []*g2.AggProfile {
	var sortedProfiles []*g2.AggProfile
	for _, p := range aggProfiles {
		sortedProfiles = append(sortedProfiles, p)
	}
	sort.Slice(sortedProfiles, func(i, j int) bool { return sortedProfiles[i].Path < sortedProfiles[j].Path })
	return sortedProfiles
}

func sortArches(aggArches map[string]*AggArch) []*AggArch {
	var sortedArches []*AggArch
	for _, a := range aggArches {
		sortedArches = append(sortedArches, a)
	}
	sort.Slice(sortedArches, func(i, j int) bool { return sortedArches[i].Name < sortedArches[j].Name })
	return sortedArches
}

func sortEclasses(aggEclasses map[string]*AggEclass) []*AggEclass {
	var sortedEclasses []*AggEclass
	for _, ec := range aggEclasses {
		sort.Slice(ec.Packages, func(i, j int) bool {
			if ec.Packages[i].Category == ec.Packages[j].Category {
				return ec.Packages[i].Name < ec.Packages[j].Name
			}
			return ec.Packages[i].Category < ec.Packages[j].Category
		})
		sortedEclasses = append(sortedEclasses, ec)
	}
	sort.Slice(sortedEclasses, func(i, j int) bool { return sortedEclasses[i].Name < sortedEclasses[j].Name })
	return sortedEclasses
}

func extractRecentNews(globalNews []AggNewsItem) []AggNewsItem {
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
	return recentNews
}

func sortGroupedRepos(groupedReposMap map[string]*RepoGroup) []RepoGroup {
	var sortedGroupedRepos []RepoGroup
	for _, group := range groupedReposMap {
		sortedGroupedRepos = append(sortedGroupedRepos, *group)
	}
	sort.Slice(sortedGroupedRepos, func(i, j int) bool {
		if sortedGroupedRepos[i].Quality == sortedGroupedRepos[j].Quality {
			return sortedGroupedRepos[i].Status < sortedGroupedRepos[j].Status
		}
		return sortedGroupedRepos[i].Quality < sortedGroupedRepos[j].Quality
	})
	return sortedGroupedRepos
}

func prepareAggregatedData(sites []*g2.SiteData) *AggregatedData {
	groupedReposMap := aggregateGroupedRepos(sites)
	aggUseExpandDescs := aggregateUseExpandDescs(sites)
	aggProjects := aggregateProjects(sites)
	aggProfiles := aggregateProfiles(sites)
	globalNews := aggregateGlobalNews(sites)
	aggArches := aggregateArches(sites)
	aggMoves := aggregateMoves(sites)
	aggEclasses := aggregateEclasses(sites)
	aggCategories, aggPackages, aggLicenses, totalPackages := aggregatePackagesAndCategories(sites, aggProjects)

	sortedCategories := sortCategories(aggCategories)
	sortedPackages := sortPackages(aggPackages)
	validUseExpands := aggregateValidUseExpands(sites)
	sortedLicenses, validLicenses := sortLicenses(aggLicenses)
	sortedProjects := sortProjects(aggProjects)
	sortedProfiles := sortProfiles(aggProfiles)
	sortedArches := sortArches(aggArches)
	sortedEclasses := sortEclasses(aggEclasses)
	recentNews := extractRecentNews(globalNews)
	sortedGroupedRepos := sortGroupedRepos(groupedReposMap)

	sortedUseFlags, _ := AggregateUseFlags(sites, aggPackages)

	return &AggregatedData{
		Categories:      sortedCategories,
		Packages:        sortedPackages,
		Licenses:        sortedLicenses,
		Projects:        sortedProjects,
		Profiles:        sortedProfiles,
		Arches:          sortedArches,
		Moves:           aggMoves,
		GlobalNews:      globalNews,
		RecentNews:      recentNews,
		TotalPackages:   totalPackages,
		UseFlags:        sortedUseFlags,
		ValidLicenses:   validLicenses,
		Eclasses:        sortedEclasses,
		UseExpandDescs:  aggUseExpandDescs,
		ValidUseExpands: validUseExpands,
		GroupedRepos:    sortedGroupedRepos,
	}
}

func renderPage(path string, tmpl *template.Template, name string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if err := tmpl.ExecuteTemplate(f, "layout_header.html", data); err != nil {
		return fmt.Errorf("executing layout_header template for %s: %w", path, err)
	}
	if err := tmpl.ExecuteTemplate(f, name, data); err != nil {
		return fmt.Errorf("executing template %s for %s: %w", name, path, err)
	}
	if err := tmpl.ExecuteTemplate(f, "layout_footer.html", data); err != nil {
		return fmt.Errorf("executing layout_footer template for %s: %w", path, err)
	}

	return nil
}

func (cfg *MainArgConfig) cmdSiteRemote(repositoriesFile string, outDir string, recentDuration time.Duration, recentDurationStr string, fastGit bool, useZip bool, concurrency int, retries int, continueOnError bool, persistentDir string, reposConfPath string, tempDir string, workMode string, mode string, profileSiteGen bool, profileOut string, smartMode bool) error {
	var repos g2.Repositories

	if reposConfPath != "" {
		rc, err := g2.ParseReposConf(reposConfPath)
		if err != nil {
			return fmt.Errorf("parsing repos.conf: %w", err)
		}
		for _, f := range rc.Files {
			for _, s := range f.Sections {
				if s.Name == "DEFAULT" || s.Disabled {
					continue
				}
				loc := s.Get("location")
				syncURI := s.Get("sync-uri")
				if syncURI != "" {
					repos.Repositories = append(repos.Repositories, g2.Repository{
						Name:    s.Name,
						Sources: []g2.RepositorySource{{Type: "git", Text: syncURI}},
					})
				} else if loc != "" {
					repos.Repositories = append(repos.Repositories, g2.Repository{
						Name:    s.Name,
						Sources: []g2.RepositorySource{{Type: "git", Text: loc}},
					})
				}
			}
		}
	}

	if repositoriesFile != "" {
		var data []byte
		var err error

		if repositoriesFile == "-" {
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading repositories.xml from stdin: %w", err)
			}
		} else if strings.HasPrefix(repositoriesFile, "http://") || strings.HasPrefix(repositoriesFile, "https://") {
			// Convert github blob URL to raw URL to download the actual XML content, not the HTML page.
			if strings.HasPrefix(repositoriesFile, "https://github.com/") && strings.Contains(repositoriesFile, "/blob/") {
				repositoriesFile = strings.Replace(repositoriesFile, "https://github.com/", "https://raw.githubusercontent.com/", 1)
				repositoriesFile = strings.Replace(repositoriesFile, "/blob/", "/", 1)
			}
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

		var fileRepos g2.Repositories
		if err := xml.Unmarshal(data, &fileRepos); err != nil {
			return fmt.Errorf("parsing repositories.xml: %w", err)
		}
		repos.Repositories = append(repos.Repositories, fileRepos.Repositories...)
	}

	// Create a temporary or persistent directory to clone repos into
	var tmpDir string
	var cleanup func()

	if workMode == "persistent" {
		tmpDir = persistentDir
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("creating persistent dir: %w", err)
		}
		cleanup = func() {}
	} else {
		var err error
		tmpDir, err = os.MkdirTemp(tempDir, "g2-sitegen-*")
		if err != nil {
			return fmt.Errorf("creating temp dir: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(tmpDir) }
	}
	defer cleanup()

	var allSites []*g2.SiteData
	var allSitesMu sync.Mutex
	var lastLogTime time.Time

	var processedRepos int
	var totalCategories int
	var totalPackages int
	var totalPackageVersions int

	limit := concurrency
	if limit <= 0 {
		limit = 10
	}

	memManager := NewMemoryManager()
	freeMem, err := getFreeMemory()
	var defaultAlloc uint64 = 0
	if err == nil && limit > 0 {
		defaultAlloc = freeMem / 2 / uint64(limit)
	}
	if mode == "pipeline" {
		log.Printf("Starting pipelined remote repository processing")
		type fetchTask struct {
			repo   g2.Repository
			gitUrl string
		}
		type parseTask struct {
			repo         g2.Repository
			gitUrl       string
			repoPath     string
			sysFS        fs.FS
			checkoutTime time.Duration
		}
		type cleanTask struct {
			repoPath string
		}

		limit := concurrency
		if limit <= 0 {
			limit = 10
		}

		fetchCh := make(chan fetchTask, limit)
		parseCh := make(chan parseTask, limit)
		cleanCh := make(chan cleanTask, limit)

		var fetchWg sync.WaitGroup
		var parseWg sync.WaitGroup
		var cleanWg sync.WaitGroup

		var pipelineErr error
		var pipelineErrMu sync.Mutex

		setErr := func(err error) {
			pipelineErrMu.Lock()
			if pipelineErr == nil {
				pipelineErr = err
			}
			pipelineErrMu.Unlock()
		}

		// Clean workers
		for i := 0; i < limit; i++ {
			cleanWg.Add(1)
			go func() {
				defer cleanWg.Done()
				for task := range cleanCh {
					_ = os.RemoveAll(task.repoPath)
				}
			}()
		}

		// Parse workers
		for i := 0; i < limit; i++ {
			parseWg.Add(1)
			go func() {
				defer parseWg.Done()
				for task := range parseCh {
					log.Printf("[START] Parsing repository: %s", task.repo.Name)

					size, err := getDirSize(task.repoPath)
					var gitSize string
					if err == nil {
						gitSize = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
					}

					repoCopy := task.repo

					t1 := time.Now()
					var parseFS fs.FS
					if task.sysFS != nil {
						parseFS = task.sysFS
					} else {
						parseFS = os.DirFS(task.repoPath)
					}
					var siteData *g2.SiteData
					func() {
						memManager.Acquire(defaultAlloc)
						defer memManager.Release(defaultAlloc)
						siteData, err = parseRepo(parseFS, ".", task.repo.Name, fastGit, &repoCopy, SourceURL(task.gitUrl))
					}()

					cleanCh <- cleanTask{repoPath: task.repoPath}

					if err != nil {
						log.Printf("Failed to parse repo %s: %v", task.repo.Name, err)
						continue
					}
					processTime := time.Since(t1)
					nodeCount := 0
					if siteData != nil {
						nodeCount = len(siteData.Categories) + siteData.PackageCount + len(siteData.Profiles) + len(siteData.News) + len(siteData.Moves) + len(siteData.DefinedEclasses)
						for _, cat := range siteData.Categories {
							for _, pkg := range cat.Packages {
								nodeCount += len(pkg.Versions)
							}
						}
					}
					logParseStats(task.repo.Name, processTime, task.repoPath, nodeCount)

					if concurrency == 1 {
						numCategories := len(siteData.Categories)
						numPackages := siteData.PackageCount
						numVersions := 0
						numLicenses := 0
						uniqueLicenses := make(map[string]bool)
						for _, cat := range siteData.Categories {
							for _, pkg := range cat.Packages {
								numVersions += len(pkg.Versions)
								for _, v := range pkg.Versions {
									if v.Ebuild != nil && v.Ebuild.Vars != nil {
										licStr := v.Ebuild.Vars["LICENSE"]
										if licStr != "" {
											lics := g2.ParseLicense(licStr)
											for _, l := range lics {
												if l != "" {
													uniqueLicenses[l] = true
												}
											}
										}
									}
								}
							}
						}
						numLicenses = len(uniqueLicenses)
						numEclasses := len(siteData.DefinedEclasses)
						numProfiles := len(siteData.Profiles)
						numNews := len(siteData.News)

						log.Printf("[STATS] Domain Objects - Repos: 1, Categories: %d, Packages: %d, Ebuilds: %d, Profiles: %d, Eclasses: %d, News: %d, Licenses: %d", numCategories, numPackages, numVersions, numProfiles, numEclasses, numNews, numLicenses)

						var memStatsAfter runtime.MemStats
						runtime.ReadMemStats(&memStatsAfter)
						log.Printf("[STATS] Heap Objects: %d, Alloc: %.2f MB, Total Alloc: %.2f MB, Sys: %.2f MB, NumGC: %d",
							memStatsAfter.HeapObjects,
							float64(memStatsAfter.Alloc)/(1024*1024),
							float64(memStatsAfter.TotalAlloc)/(1024*1024),
							float64(memStatsAfter.Sys)/(1024*1024),
							memStatsAfter.NumGC,
						)
					}

					siteData.CheckoutTime = task.checkoutTime.String()
					siteData.ProcessTime = processTime.String()
					siteData.GitSize = gitSize
					allSitesMu.Lock()
					allSites = append(allSites, siteData)

					processedRepos++
					totalCategories += len(siteData.Categories)

					repoPackages := 0
					repoPackageVersions := 0
					for _, cat := range siteData.Categories {
						repoPackages += len(cat.Packages)
						for _, pkg := range cat.Packages {
							repoPackageVersions += len(pkg.Versions)
						}
					}
					totalPackages += repoPackages
					totalPackageVersions += repoPackageVersions

					now := time.Now()
					if lastLogTime.IsZero() || now.Sub(lastLogTime) >= 10*time.Minute {
						lastLogTime = now
						currentRepos := len(allSites)
						var currentPackages int
						for _, site := range allSites {
							if site != nil {
								currentPackages += site.PackageCount
							}
						}
						log.Printf("[PROGRESS] Currently processed %d repositories and %d total packages so far", currentRepos, currentPackages)
					}

					if processedRepos%10 == 0 {
						memUsage := getProcessMemUsage()
						log.Printf("[PROGRESS] Processed %d repositories. Memory Usage: %.2f MB. Cumulative: Categories: %d, Packages: %d, Versions: %d",
							processedRepos,
							float64(memUsage)/(1024*1024),
							totalCategories,
							totalPackages,
							totalPackageVersions,
						)
					}

					allSitesMu.Unlock()
				}
			}()
		}

		// Fetch workers
		for i := 0; i < limit; i++ {
			fetchWg.Add(1)
			go func() {
				defer fetchWg.Done()
				for task := range fetchCh {
					log.Printf("[START] Fetching remote repository: %s (%s)", task.repo.Name, task.gitUrl)

					repoPath := filepath.Join(tmpDir, task.repo.Name)

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

					t0 := time.Now()

					var sysFS fs.FS
					var err error

					if smartMode {
						zipUrl := getZipUrl(task.gitUrl)
						if zipUrl != "" {
							sysFS, err = tryFetchZipFS(ctx, zipUrl)
							if err == nil {
								log.Printf("Successfully fetched zip in memory for %s", task.repo.Name)
							}
						}
						if sysFS == nil {
							sysFS, err = tryFetchGitFS(ctx, task.gitUrl)
							if err == nil {
								log.Printf("Successfully cloned into memory for %s", task.repo.Name)
							}
						}
					}

					if sysFS == nil {
						err = FetchRepo(ctx, task.gitUrl, repoPath, useZip, workMode, retries)
					}

					cancel()

					if err != nil && sysFS == nil {
						log.Printf("Failed to fetch %s: %v", task.repo.Name, err)
						if !continueOnError {
							setErr(fmt.Errorf("fetching %s: %w", task.repo.Name, err))
						}
						_ = os.RemoveAll(repoPath)
						continue
					}
					checkoutTime := time.Since(t0)
					logFetchStats(task.repo.Name, checkoutTime, repoPath)

					parseCh <- parseTask{
						repo:         task.repo,
						gitUrl:       task.gitUrl,
						repoPath:     repoPath,
						sysFS:        sysFS,
						checkoutTime: checkoutTime,
					}
				}
			}()
		}

		for _, repo := range repos.Repositories {
			if len(repo.Sources) == 0 {
				continue
			}

			var gitUrl string
			for _, src := range repo.Sources {
				if src.Type == "git" && strings.HasPrefix(src.Text, "http") {
					gitUrl = src.Text
					break
				}
			}

			if gitUrl == "" {
				continue
			}

			fetchCh <- fetchTask{
				repo:   repo,
				gitUrl: gitUrl,
			}
		}

		close(fetchCh)
		fetchWg.Wait()
		close(parseCh)
		parseWg.Wait()
		close(cleanCh)
		cleanWg.Wait()

		if pipelineErr != nil {
			return pipelineErr
		}
	} else {
		g, _ := errgroup.WithContext(context.Background())
		if concurrency > 0 {
			g.SetLimit(concurrency)
			log.Printf("Starting concurrent remote repository processing with %d concurrency limit", concurrency)
		} else {
			log.Printf("Starting concurrent remote repository processing with unbounded concurrency")
		}

		for _, repo := range repos.Repositories {
			repo := repo // loop variable capture
			if len(repo.Sources) == 0 {
				continue
			}

			var gitUrl string
			for _, src := range repo.Sources {
				if src.Type == "git" && strings.HasPrefix(src.Text, "http") {
					gitUrl = src.Text
					break
				}
			}

			if gitUrl == "" {
				continue // skip non-http git repos for this tool
			}

			g.Go(func() error {
				log.Printf("[START] Fetching remote repository: %s (%s)", repo.Name, gitUrl)

				repoPath := filepath.Join(tmpDir, repo.Name)
				defer func() { _ = os.RemoveAll(repoPath) }()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				t0 := time.Now()
				var sysFS fs.FS
				var err error

				if smartMode {
					zipUrl := getZipUrl(gitUrl)
					if zipUrl != "" {
						sysFS, err = tryFetchZipFS(ctx, zipUrl)
						if err == nil {
							log.Printf("Successfully fetched zip in memory for %s", repo.Name)
						}
					}
					if sysFS == nil {
						sysFS, err = tryFetchGitFS(ctx, gitUrl)
						if err == nil {
							log.Printf("Successfully cloned into memory for %s", repo.Name)
						}
					}
				}

				if sysFS == nil {
					err = FetchRepo(ctx, gitUrl, repoPath, useZip, workMode, retries)
				}

				if err != nil && sysFS == nil {
					log.Printf("Failed to fetch %s: %v", repo.Name, err)
					if !continueOnError {
						return fmt.Errorf("fetching %s: %w", repo.Name, err)
					}
					return nil
				}
				checkoutTime := time.Since(t0)
				logFetchStats(repo.Name, checkoutTime, repoPath)

				log.Printf("[START] Parsing repository: %s", repo.Name)

				size, err := getDirSize(repoPath)
				var gitSize string
				if err == nil {
					gitSize = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
				}

				repoCopy := repo

				t1 := time.Now()
				var parseFS fs.FS
				if sysFS != nil {
					parseFS = sysFS
				} else {
					parseFS = os.DirFS(repoPath)
				}
				var siteData *g2.SiteData
				func() {
					memManager.Acquire(defaultAlloc)
					defer memManager.Release(defaultAlloc)
					siteData, err = parseRepo(parseFS, ".", repo.Name, fastGit, &repoCopy, SourceURL(gitUrl))
				}()
				if err != nil {
					log.Printf("Failed to parse repo %s: %v", repo.Name, err)
					return nil
				}
				processTime := time.Since(t1)
				nodeCount := 0
				if siteData != nil {
					nodeCount = len(siteData.Categories) + siteData.PackageCount + len(siteData.Profiles) + len(siteData.News) + len(siteData.Moves) + len(siteData.DefinedEclasses)
					for _, cat := range siteData.Categories {
						for _, pkg := range cat.Packages {
							nodeCount += len(pkg.Versions)
						}
					}
				}
				logParseStats(repo.Name, processTime, repoPath, nodeCount)

				if concurrency == 1 {
					numCategories := len(siteData.Categories)
					numPackages := siteData.PackageCount
					numVersions := 0
					numLicenses := 0
					uniqueLicenses := make(map[string]bool)
					for _, cat := range siteData.Categories {
						for _, pkg := range cat.Packages {
							numVersions += len(pkg.Versions)
							for _, v := range pkg.Versions {
								if v.Ebuild != nil && v.Ebuild.Vars != nil {
									licStr := v.Ebuild.Vars["LICENSE"]
									if licStr != "" {
										lics := g2.ParseLicense(licStr)
										for _, l := range lics {
											if l != "" {
												uniqueLicenses[l] = true
											}
										}
									}
								}
							}
						}
					}
					numLicenses = len(uniqueLicenses)
					numEclasses := len(siteData.DefinedEclasses)
					numProfiles := len(siteData.Profiles)
					numNews := len(siteData.News)

					log.Printf("[STATS] Domain Objects - Repos: 1, Categories: %d, Packages: %d, Ebuilds: %d, Profiles: %d, Eclasses: %d, News: %d, Licenses: %d", numCategories, numPackages, numVersions, numProfiles, numEclasses, numNews, numLicenses)

					var memStatsAfter runtime.MemStats
					runtime.ReadMemStats(&memStatsAfter)
					log.Printf("[STATS] Heap Objects: %d, Alloc: %.2f MB, Total Alloc: %.2f MB, Sys: %.2f MB, NumGC: %d",
						memStatsAfter.HeapObjects,
						float64(memStatsAfter.Alloc)/(1024*1024),
						float64(memStatsAfter.TotalAlloc)/(1024*1024),
						float64(memStatsAfter.Sys)/(1024*1024),
						memStatsAfter.NumGC,
					)
				}

				siteData.CheckoutTime = checkoutTime.String()
				siteData.ProcessTime = processTime.String()
				siteData.GitSize = gitSize
				allSitesMu.Lock()
				allSites = append(allSites, siteData)

				processedRepos++
				totalCategories += len(siteData.Categories)

				repoPackages := 0
				repoPackageVersions := 0
				for _, cat := range siteData.Categories {
					repoPackages += len(cat.Packages)
					for _, pkg := range cat.Packages {
						repoPackageVersions += len(pkg.Versions)
					}
				}
				totalPackages += repoPackages
				totalPackageVersions += repoPackageVersions

				now := time.Now()
				if lastLogTime.IsZero() || now.Sub(lastLogTime) >= 10*time.Minute {
					lastLogTime = now
					currentRepos := len(allSites)
					var currentPackages int
					for _, site := range allSites {
						if site != nil {
							currentPackages += site.PackageCount
						}
					}
					log.Printf("[PROGRESS] Currently processed %d repositories and %d total packages so far", currentRepos, currentPackages)
				}

				if processedRepos%10 == 0 {
					memUsage := getProcessMemUsage()
					log.Printf("[PROGRESS] Processed %d repositories. Memory Usage: %.2f MB. Cumulative: Categories: %d, Packages: %d, Versions: %d",
						processedRepos,
						float64(memUsage)/(1024*1024),
						totalCategories,
						totalPackages,
						totalPackageVersions,
					)
				}

				allSitesMu.Unlock()
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			if !continueOnError {
				return fmt.Errorf("parallel repository fetching: %w", err)
			}
			log.Printf("Warning: error during parallel repository fetching: %v", err)
		}
	}
	// Sort the resulting sites alphabetically by RepoName for deterministic ordering
	sort.Slice(allSites, func(i, j int) bool {
		return allSites[i].RepoName < allSites[j].RepoName
	})

	finalMemUsage := getProcessMemUsage()
	appFreeSpaceFinal, appErrFinal := getFreeSpace(".")
	actualTempDirFinal := tempDir
	if actualTempDirFinal == "" {
		actualTempDirFinal = os.TempDir()
	}
	tmpFreeSpaceFinal, tmpErrFinal := getFreeSpace(actualTempDirFinal)
	log.Printf("--------------------------------------------------")
	log.Printf("[FINAL SUMMARY] Repository Processing Complete")
	log.Printf("Total Repositories:      %d", len(allSites))
	log.Printf("Total Categories:        %d", totalCategories)
	log.Printf("Total Packages:          %d", totalPackages)
	log.Printf("Total Package Versions:  %d", totalPackageVersions)
	log.Printf("Final Memory Usage:      %.2f MB", float64(finalMemUsage)/(1024*1024))
	if appErrFinal == nil {
		log.Printf("Final App Free Space:    %.2f MB", float64(appFreeSpaceFinal)/(1024*1024))
	}
	if tmpErrFinal == nil {
		log.Printf("Final Tmp Free Space:    %.2f MB", float64(tmpFreeSpaceFinal)/(1024*1024))
	}
	log.Printf("--------------------------------------------------")

	log.Printf("Generating integrated site (v%s) for %d repos", version, len(allSites))
	profiler := NewProfiler(profileSiteGen, profileOut)
	genInfo := GenerationInfo{
		Profiler:       profiler,
		Args:           cfg.Args,
		FastGit:        fastGit,
		RecentDuration: recentDurationStr,
	}
	if err := generateSite(outDir, allSites, recentDuration, recentDurationStr, genInfo); err != nil {
		return fmt.Errorf("generating integrated site: %w", err)
	}

	log.Println("Remote site generation complete.")
	return nil
}

func mapToList(m map[string]*g2.SiteData) []*g2.SiteData {
	var l []*g2.SiteData
	for _, v := range m {
		l = append(l, v)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].RepoName < l[j].RepoName })
	return l
}

func (e *AggEclass) IsDefinedLocally(repoName string) bool {
	for rn := range e.Repos {
		if rn == repoName {
			return true
		}
	}
	return false
}

func (a *AggPackage) GetVersionsInRepo(repoName string) []g2.VersionData {
	if site, ok := a.Repos[repoName]; ok {
		for _, cat := range site.Categories {
			if cat.Name == a.Category {
				for _, pkg := range cat.Packages {
					if pkg.Name == a.Name {
						return pkg.Versions
					}
				}
			}
		}
	}
	return nil
}
