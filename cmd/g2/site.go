package main

import (
	"context"
	"encoding/json"
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
	"github.com/arran4/g2/lints"
	"github.com/arran4/g2/lints/ebuild"
	"golang.org/x/sync/errgroup"
)

func getProcessMemUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}


type SourceURL string

type EclassData struct {
	Name string
}

type SiteData struct {
	Title             string
	RepoName          string
	RemoteURL         string
	Repository        *g2.Repository
	EAPI              string
	Projects          *g2.Projects
	Categories        []CategoryData
	Profiles          []g2.ProfileData
	DefinedEclasses   []EclassData
	AggEclasses       []*AggEclass
	Authors           []g2.Author
	AuthorsURL        string
	Moves             []g2.PackageMove
	SlotMoves         []g2.PackageSlotMove
	News              []g2.NewsItem
	LayoutConf        *g2.LayoutConf
	LicenseMapping    map[string][]string
	QAPolicy          *g2.QAPolicy
	UseDesc           *g2.UseDesc
	UseLocalDesc      *g2.UseLocalDesc
	UseExpandDescs    map[string]*g2.UseExpandDesc
	ValidUseExpands   map[string]bool
	ArchList          *g2.ArchList
	ArchesDesc        *g2.ArchesDesc
	InfoPkgs          []g2.InfoPkg
	Masked            []g2.PackageMasked
	Deprecated        []g2.PackageDeprecated
	ParsedEclasses    []*g2.Ebuild
	Eclasses          []*g2.Ebuild
	PackageCount      int
	AggUseFlags       []*AggUseFlag
	ThirdPartyMirrors map[string][]string
	InfoVars          []string
	GitSize           string
	CheckoutTime      string
	ProcessTime       string
	SourceURL         string
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
	Entry        *g2.ManifestEntry
	Versions     []string
	URLs         []string
	ResolvedURLs []string
}

type PackageData struct {
	Name                  string
	Category              string
	Versions              []VersionData
	Metadata              *g2.PkgMetadata
	MetadataError         error
	Manifest              *g2.Manifest
	ManifestData          []ManifestEntryData
	Files                 []FileData
	HighestStableVersion  template.HTML
	HighestTestingVersion template.HTML
	EbuildCount           int
	DominantDescription   string
	DominantHomepage      string
	DominantLicense       string

	// Git info
	MetadataRawURL string
	ModTime        time.Time

	// Processed Uses (per package)
	PkgUseFlags []PkgUseFlag

	// Lint Info
	LintWarnings []string

	// Deprecation
	Masked     *g2.PackageMasked
	Deprecated *g2.PackageDeprecated

	// InfoPkg matching
	IsInfoPkg bool

	ReverseVirtuals []string
	Equivalents     []string
	VirtualDeps     []string
}

type PkgUseFlag struct {
	Name     string
	Desc     string
	Source   string
	Versions map[string]string // Version -> Unicode symbol representing state
}

type VersionData struct {
	Version string
	Ebuild  *g2.Ebuild

	// Git info
	EbuildRawURL string
	ModTime      time.Time

	// Deprecation
	Deprecated *g2.PackageDeprecated
	Masked     *g2.PackageMasked

	// Moves
	MovedToSlot string

	ResolvedDepsJSON string
	// Mirrors
	ApplicableMirrors map[string][]string
}

// End model TODO check

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
	outDir := fs.String("out", "site_out", "Output directory for the generated site")
	clear := fs.Bool("clear", false, "Clear output directory before generation")
	recentDurOpt := fs.String("recent-duration", "3mo", "Duration to consider an update 'recent' (e.g. 3mo, 14d, 72h)")
	fastGit := fs.Bool("fast-git-modtime", false, "Use fast (O(1)) but potentially less reliable go-git file log lookup")
	useZip := fs.Bool("use-zip", false, "Download zip archives instead of git clone when supported")
	persistentDir := fs.String("persistent-dir", "", "Directory to persistently store checked out repositories instead of a temporary directory")
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

	var allSites []*SiteData
	var allSitesMu sync.Mutex
	var lastLogTime time.Time

	g, _ := errgroup.WithContext(context.Background())

	for _, task := range tasks {
		task := task
		g.Go(func() error {
			isRemote := strings.HasPrefix(task.Location, "http://") || strings.HasPrefix(task.Location, "https://") || strings.HasPrefix(task.Location, "git://")
			var parseLocation string
			var cleanup func()
			var siteData *SiteData

			if isRemote {
				var tmpDir string
				var err error

				if *persistentDir != "" {
					tmpDir = filepath.Join(*persistentDir, task.Name)
					if err := os.MkdirAll(tmpDir, 0755); err != nil {
						return fmt.Errorf("creating persistent dir for %s: %w", task.Name, err)
					}
					cleanup = func() {}
				} else {
					tmpDir, err = os.MkdirTemp("", "g2-overlay-"+task.Name+"-*")
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
				if err := FetchRepo(ctx, task.Location, tmpDir, *useZip, *persistentDir != "", 0); err != nil {
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
	outDir := fs.String("out", "site_out", "Output directory for the generated site")
	clear := fs.Bool("clear", false, "Clear output directory before generation")
	recentDurOpt := fs.String("recent-duration", "3mo", "Duration to consider an update 'recent' (e.g. 3mo, 14d, 72h)")
	fastGit := fs.Bool("fast-git-modtime", false, "Use fast (O(1)) but potentially less reliable go-git file log lookup")
	useZip := fs.Bool("use-zip", false, "Download zip archives instead of git clone when supported")
	concurrency := fs.Int("concurrency", 4, "Maximum number of concurrent repository fetches/parses")
	retries := fs.Int("retries", 3, "Number of times to retry fetching a repository")
	continueOnError := fs.Bool("continue-on-error", true, "Continue parsing other repositories even if fetching one fails")
	persistentDir := fs.String("persistent-dir", "", "Directory to persistently store checked out repositories instead of a temporary directory")
	reposConfOpt := fs.String("repos-conf", "", "Path to repos.conf file or directory")

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
	return cfg.cmdSiteRemote(location, *outDir, recentDuration, recentDurationStr, *fastGit, *useZip, *concurrency, *retries, *continueOnError, *persistentDir, *reposConfOpt)
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

func getHighestVersionsAndCount(versions []VersionData, site *SiteData) (template.HTML, template.HTML, int) {
	// Parse KEYWORDS and group versions

	stableMap := make(map[string]string)
	testingMap := make(map[string]string)

	for _, ver := range versions {
		if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
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

	formatGroups := func(groups map[string][]string) string {
		if len(groups) == 0 {
			return ""
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

		var parts []string
		for _, ver := range sortedVersions {
			archs := groups[ver]

			// sort archs
			for i := 0; i < len(archs); i++ {
				for j := i + 1; j < len(archs); j++ {
					if archs[i] > archs[j] {
						archs[i], archs[j] = archs[j], archs[i]
					}
				}
			}

			parts = append(parts, "<span title=\""+strings.Join(archs, " ")+"\">"+ver+"</span>")
		}

		return strings.Join(parts, ", ")
	}

	return template.HTML(formatGroups(stableGroup)), template.HTML(formatGroups(testingGroup)), len(versions)
}

type ResolvedDepNode struct {
	Type      string            `json:"type"`
	Name      string            `json:"name,omitempty"`
	Link      string            `json:"link,omitempty"`
	Flag      string            `json:"flag,omitempty"`
	IsNegated bool              `json:"is_negated,omitempty"`
	Children  []ResolvedDepNode `json:"children,omitempty"`
}

func resolveDependencies(node g2.DepNode, site *SiteData) ResolvedDepNode {
	switch n := node.(type) {
	case g2.DepString:
		raw := string(n)
		pkgName := g2.ExtractPackageNameFromDep(raw)
		link := ""

		if pkgName != "" {
			for i := range site.Categories {
				for j := range site.Categories[i].Packages {
					if site.Categories[i].Packages[j].Category+"/"+site.Categories[i].Packages[j].Name == pkgName {
						link = "../../../../../../categories/" + site.Categories[i].Packages[j].Category + "/packages/" + site.Categories[i].Packages[j].Name + "/"
					}
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
			res.Children = append(res.Children, resolveDependencies(child, site))
		}
		return res

	case g2.DepAllOf:
		res := ResolvedDepNode{Type: "all_of"}
		for _, child := range n.Children {
			res.Children = append(res.Children, resolveDependencies(child, site))
		}
		return res

	case g2.DepUseConditional:
		res := ResolvedDepNode{
			Type:      "use_conditional",
			Flag:      n.Flag,
			IsNegated: n.IsNegated,
		}
		for _, child := range n.Children {
			res.Children = append(res.Children, resolveDependencies(child, site))
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

func parseRepo(sysFS fs.FS, repoDir string, defaultTitle string, fastGit bool, repoInfo *g2.Repository, opts ...any) (*SiteData, error) {
	title := defaultTitle
	var repoName string
	var remoteURL string

	for _, opt := range opts {
		switch o := opt.(type) {
		case SourceURL:
			remoteURL = string(o)
		}
	}

	isUselessURL := func(u string) bool {
		u = strings.TrimSpace(u)
		return u == "" || u == "." || u == ".." || u == "./" || u == "../"
	}

	originalRemoteURL := remoteURL

	// Implement fallback layers for remoteURL
	if isUselessURL(remoteURL) {
		remoteURL = ""

		// 1. Git origin URL
		gitURL, err := getGitOriginURL(repoDir)
		if err == nil && !isUselessURL(gitURL) {
			remoteURL = gitURL
		}

		// 2. RepoInfo Sources
		if remoteURL == "" && repoInfo != nil && len(repoInfo.Sources) > 0 {
			for _, src := range repoInfo.Sources {
				if !isUselessURL(src.Text) {
					remoteURL = src.Text
					break
				}
			}
		}

		// 3. RepoInfo Homepage
		if remoteURL == "" && repoInfo != nil && !isUselessURL(repoInfo.Homepage) {
			remoteURL = repoInfo.Homepage
		}

		// 4. Fallback to original
		if remoteURL == "" {
			remoteURL = originalRemoteURL
		}
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

	var licenseGroups map[string][]string
	licenseGroupsPath := filepath.Join(repoDir, "profiles", "license_groups")
	if f, err := sysFS.Open(filepath.ToSlash(licenseGroupsPath)); err == nil {
		groups, err := g2.ParseLicenseGroups(f)
		_ = f.Close()
		if err != nil {
			log.Printf("Warning: failed to parse license_groups: %v", err)
		} else {
			// ParseLicenseGroups returns group -> licenses, we need license -> groups mapping
			licenseMapping := make(map[string][]string)
			for group, licenses := range groups {
				for _, lic := range licenses {
					licenseMapping[lic] = append(licenseMapping[lic], group)
				}
			}
			licenseGroups = licenseMapping
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

	var useDesc *g2.UseDesc
	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "use.desc"))); err == nil {
		defer func() { _ = f.Close() }()
		if ud, err := g2.ParseUseDesc(f); err == nil {
			useDesc = ud
		}
	}

	var useExpandDescs map[string]*g2.UseExpandDesc
	if descs, err := g2.ParseUseExpandDescFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "desc"))); err == nil {
		useExpandDescs = descs
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse use expand desc: %v", err)
	}
	var useLocalDesc *g2.UseLocalDesc
	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "use.local.desc"))); err == nil {
		defer func() { _ = f.Close() }()
		if uld, err := g2.ParseUseLocalDesc(f); err == nil {
			useLocalDesc = uld
		}
	}
	var archList *g2.ArchList
	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "arch.list"))); err == nil {
		defer func() { _ = f.Close() }()
		if al, err := g2.ParseArchList(f); err == nil {
			archList = al
		}
	}

	var archesDesc *g2.ArchesDesc
	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "arches.desc"))); err == nil {
		defer func() { _ = f.Close() }()
		if ad, err := g2.ParseArchesDesc(f); err == nil {
			archesDesc = ad
		}
	}

	packageDeprecatedPath := filepath.Join(repoDir, "profiles", "package.deprecated")
	var deprecated []g2.PackageDeprecated
	if parsedDeprecated, err := g2.ParsePackageDeprecatedFS(sysFS, filepath.ToSlash(packageDeprecatedPath)); err == nil {
		deprecated = parsedDeprecated
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse package.deprecated: %v", err)
	}

	packageMaskPath := filepath.Join(repoDir, "profiles", "package.mask")
	var masked []g2.PackageMasked
	if parsedMasked, err := g2.ParsePackageMaskedFS(sysFS, filepath.ToSlash(packageMaskPath)); err == nil {
		masked = parsedMasked
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse package.mask: %v", err)
	}

	var thirdPartyMirrors map[string][]string
	if tm, err := g2.ParseThirdPartyMirrorsFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "thirdpartymirrors"))); err == nil {
		thirdPartyMirrors = tm
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse thirdpartymirrors: %v", err)
	}
	infoVarsPath := filepath.Join(repoDir, "profiles", "info_vars")
	var infoVars []string
	if parsedInfoVars, err := g2.ParseInfoVarsFS(sysFS, filepath.ToSlash(infoVarsPath)); err == nil {
		infoVars = parsedInfoVars
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse info_vars: %v", err)
	}
	infoPkgsPath := filepath.Join(repoDir, "profiles", "info_pkgs")
	var infoPkgs []g2.InfoPkg
	if parsedInfoPkgs, err := g2.ParseInfoPkgsFS(sysFS, filepath.ToSlash(infoPkgsPath)); err == nil {
		infoPkgs = parsedInfoPkgs
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse info_pkgs: %v", err)
	}

	site := &SiteData{
		Title:             title,
		RepoName:          repoName,
		RemoteURL:         remoteURL,
		SourceURL:         remoteURL,
		Repository:        repoInfo,
		EAPI:              eapi,
		LayoutConf:        lc,
		LicenseMapping:    licenseGroups,
		QAPolicy:          qa,
		UseDesc:           useDesc,
		UseLocalDesc:      useLocalDesc,
		UseExpandDescs:    useExpandDescs,
		ValidUseExpands:   make(map[string]bool),
		ArchList:          archList,
		ArchesDesc:        archesDesc,
		ThirdPartyMirrors: thirdPartyMirrors,
		Deprecated:        deprecated,
		Masked:            masked,
		InfoVars:          infoVars,
		InfoPkgs:          infoPkgs,
		PackageCount:      0,
	}

	if site.UseExpandDescs != nil {
		for prefix := range site.UseExpandDescs {
			site.ValidUseExpands[prefix] = true
		}
	}

	// Calculate PackageCount correctly after parsing all categories
	defer func() {
		count := 0
		for _, cat := range site.Categories {
			count += len(cat.Packages)
		}
		site.PackageCount = count
	}()

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

			item := g2.ParseNewsItem(string(content))
			item.DirName = dirName
			item.FileName = dirName + ".en.txt"

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
				site.DefinedEclasses = append(site.DefinedEclasses, EclassData{
					Name: eclassName,
				})
			}
		}
		sort.Slice(site.DefinedEclasses, func(i, j int) bool {
			return site.DefinedEclasses[i].Name < site.DefinedEclasses[j].Name
		})
	}

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
		site.SlotMoves = updates.SlotMoves
	}

	pf, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "metadata", "projects.xml")))
	if err != nil {
		if fastGit {
			// fastGit uses an actual os path underneath when overlay is given
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

		if len(supportedCategories) > 0 && !supportedCategories[name] && name != "virtual" && !strings.HasPrefix(name, "virtual-") {
			continue
		}

		catData := CategoryData{Name: name}
		catPath := filepath.Join(repoDir, name)

		inRepo := len(supportedCategories) == 0 || supportedCategories[name]
		mainCats := g2.FetchMainGentooCategories()
		inMain := len(mainCats) == 0 || mainCats[name]

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
				ebuild, err := g2.ParseEbuild(sysFS, filepath.ToSlash(ebuildPath), g2.ParseFull)
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

				vd := VersionData{
					Version:      version,
					Ebuild:       ebuild,
					EbuildRawURL: ebuildRawURL,
					ModTime:      modTime,
				}

				if site.SlotMoves != nil {
					slot := ebuild.Vars["SLOT"]
					if slot != "" {
						for _, sm := range site.SlotMoves {
							if sm.Package == name+"/"+pkgName && sm.Old == slot {
								vd.MovedToSlot = sm.New
								break
							}
						}
					}
				}

				pkgData.Versions = append(pkgData.Versions, vd)
			}

			if len(pkgData.Versions) == 0 {
				continue // No ebuilds, skip package
			}

			pkgData.HighestStableVersion, pkgData.HighestTestingVersion, pkgData.EbuildCount = getHighestVersionsAndCount(pkgData.Versions, site)

			// Sort versions descending
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

			// Compute dominant information
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

			if pkgData.Metadata != nil && len(pkgData.Metadata.LongDescription) > 0 {
				pkgData.DominantDescription = pkgData.Metadata.LongDescription[0].Body
			} else if targetEbuild != nil {
				pkgData.DominantDescription = targetEbuild.Vars["DESCRIPTION"]
			}

			if targetEbuild != nil {
				pkgData.DominantHomepage = targetEbuild.Vars["HOMEPAGE"]
				pkgData.DominantLicense = targetEbuild.Vars["LICENSE"]
			}

			sort.Slice(pkgData.Versions, func(i, j int) bool {
				return pkgData.Versions[i].Version > pkgData.Versions[j].Version
			})

			if remoteURL != "" {
				relPath, _ := filepath.Rel(repoDir, metaPath)
				commitHash, _ := getFileCommit(repoDir, relPath)
				if commitHash != "" {
					pkgData.MetadataRawURL = generateGitHubRawURL(remoteURL, commitHash, relPath)
				}
			}

			// Set ApplicableMirrors for each version
			for i, v := range pkgData.Versions {
				if v.Ebuild != nil {
					applicableMirrors := make(map[string][]string)
					for _, uri := range v.Ebuild.SrcUri {
						if strings.HasPrefix(uri.URL, "mirror://") {
							parts := strings.SplitN(uri.URL[len("mirror://"):], "/", 2)
							if len(parts) > 0 {
								mirrorName := parts[0]
								if mirrors, ok := site.ThirdPartyMirrors[mirrorName]; ok {
									applicableMirrors[mirrorName] = mirrors
								}
							}
						}
					}
					if len(applicableMirrors) > 0 {
						pkgData.Versions[i].ApplicableMirrors = applicableMirrors
					}
				}
			}

			// Read Manifest
			manifestPath := filepath.Join(pkgPath, "Manifest")
			manifest, err := parseManifestFromFS(sysFS, filepath.ToSlash(manifestPath))
			if err == nil {
				pkgData.Manifest = manifest
				pkgData.ManifestData = buildManifestData(manifest, pkgData.Versions, site.ThirdPartyMirrors)
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

			// Assign deprecation data
			pkgStr := pkgData.Category + "/" + pkgData.Name
			for i := range site.Deprecated {
				for _, entry := range site.Deprecated[i].Entries {
					depPkg := entry.Package
					// Handle versions and operators in deprecation package strings (e.g. >=dev-python/autobahn-21)
					// A simple check is if it contains the category/name
					if strings.Contains(depPkg, pkgStr) {
						pkgData.Deprecated = &site.Deprecated[i]
						break
					}
				}
			}

			for i := range site.Masked {
				for _, entry := range site.Masked[i].Entries {
					maskPkg := entry.Package
					if strings.Contains(maskPkg, pkgStr) {
						pkgData.Masked = &site.Masked[i]
						break
					}
				}
			}

			for i, v := range pkgData.Versions {
				for j := range site.Deprecated {
					// Add deprecation note if the package string matches.
					// We match if it contains "category/name" which works for versioned atoms too.
					for _, entry := range site.Deprecated[j].Entries {
						if strings.Contains(entry.Package, pkgStr) {
							pkgData.Versions[i].Deprecated = &site.Deprecated[j]
							break
						}
					}
				}
				for j := range site.Masked {
					for _, entry := range site.Masked[j].Entries {
						if strings.Contains(entry.Package, pkgStr) {
							pkgData.Versions[i].Masked = &site.Masked[j]
							break
						}
					}
				}

				g2PkgData.Versions = append(g2PkgData.Versions, g2.VersionData{
					Version:      v.Version,
					Ebuild:       v.Ebuild,
					EbuildRawURL: v.EbuildRawURL,
					Deprecated:   pkgData.Versions[i].Deprecated,
					Masked:       pkgData.Versions[i].Masked,
				})
			}

			// Add InfoPkg status at the package level
			for j := range site.InfoPkgs {
				// We want to exact match the package string (e.g. "app-shells/bash")
				// with either the full atom or the atom without its slot part (":0").
				atom := site.InfoPkgs[j].PackageAtom
				baseAtom := atom
				if idx := strings.Index(atom, ":"); idx != -1 {
					baseAtom = atom[:idx]
				}
				if baseAtom == pkgStr {
					pkgData.IsInfoPkg = true
					break
				}
			}

			pkgData.LintWarnings = lints.PerformLinting(repoDir, &g2PkgData)

			if len(supportedCategories) > 0 && !inRepo {
				if inMain {
					pkgData.LintWarnings = append(pkgData.LintWarnings, fmt.Sprintf("Warning: category '%s' is not listed in repo's profiles/categories", name))
				} else {
					pkgData.LintWarnings = append(pkgData.LintWarnings, fmt.Sprintf("Error: category '%s' is not listed in repo's profiles/categories or the main gentoo categories list", name))
				}
			} else if len(mainCats) > 0 && !inMain {
				pkgData.LintWarnings = append(pkgData.LintWarnings, fmt.Sprintf("Note: category '%s' is not in the main gentoo categories list", name))
			}

			catData.Packages = append(catData.Packages, pkgData)
		}

		if len(catData.Packages) > 0 {
			if len(supportedCategories) > 0 && !inRepo {
				if inMain {
					log.Printf("Warning: category '%s' is not listed in repo's profiles/categories", name)
				} else {
					log.Printf("Error: category '%s' is not listed in repo's profiles/categories or the main gentoo categories list", name)
				}
			} else if len(mainCats) > 0 && !inMain {
				log.Printf("Note: category '%s' is not in the main gentoo categories list", name)
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

	// Finalize ResolvedDepsJSON for each version
	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			for k := range site.Categories[i].Packages[j].Versions {
				ver := &site.Categories[i].Packages[j].Versions[k]
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					depsMap := map[string][]ResolvedDepNode{}

					types := []string{"DEPEND", "RDEPEND", "BDEPEND", "PDEPEND", "REQUIRED_USE", "LICENSE"}
					for _, depType := range types {
						depStr := ver.Ebuild.Vars[depType]
						if depStr != "" {
							tree := g2.ParseDepTree(depStr)
							var nodes []ResolvedDepNode
							for _, n := range tree.Nodes {
								nodes = append(nodes, resolveDependencies(n, site))
							}
							depsMap[depType] = nodes
						}
					}

					jsonData, _ := json.Marshal(depsMap)
					ver.ResolvedDepsJSON = string(jsonData)
				}
			}
		}
	}

	extractVirtualDeps(site)

	// Parse Eclasses
	eclassDir = filepath.Join(repoDir, "eclass")
	if info, err := fs.Stat(sysFS, filepath.ToSlash(eclassDir)); err == nil && info.IsDir() {
		entries, err := fs.ReadDir(sysFS, filepath.ToSlash(eclassDir))
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".eclass") {
					ebuild, err := g2.ParseEbuild(sysFS, filepath.ToSlash(filepath.Join(eclassDir, e.Name())), g2.ParseFull)
					if err == nil {
						site.Eclasses = append(site.Eclasses, ebuild)
					} else {
						log.Printf("Warning: failed to parse eclass %s: %v", e.Name(), err)
					}
				}
			}
		}
	}
	sort.Slice(site.Eclasses, func(i, j int) bool {
		return site.Eclasses[i].Vars["PN"] < site.Eclasses[j].Vars["PN"]
	})

	return site, nil
}

func extractVirtualDeps(site *SiteData) {
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
				depParts := strings.Split(dep, "/")
				if len(depParts) == 2 {
					depCat := depParts[0]
					depName := depParts[1]
					for k := range site.Categories {
						if site.Categories[k].Name == depCat {
							for l := range site.Categories[k].Packages {
								if site.Categories[k].Packages[l].Name == depName {
									targetPkg := &site.Categories[k].Packages[l]
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
						}
					}
				}
			}
			sort.Strings(pkg.VirtualDeps)

			// Compute Equivalents for each package in VirtualDeps
			for _, dep := range pkg.VirtualDeps {
				depParts := strings.Split(dep, "/")
				if len(depParts) == 2 {
					depCat := depParts[0]
					depName := depParts[1]
					for k := range site.Categories {
						if site.Categories[k].Name == depCat {
							for l := range site.Categories[k].Packages {
								if site.Categories[k].Packages[l].Name == depName {
									targetPkg := &site.Categories[k].Packages[l]
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

func buildManifestData(manifest *g2.Manifest, versions []VersionData, thirdPartyMirrors map[string][]string) []ManifestEntryData {
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

						if strings.HasPrefix(uri.URL, "mirror://") {
							parts := strings.SplitN(uri.URL[len("mirror://"):], "/", 2)
							if len(parts) == 2 {
								mirrorName := parts[0]
								filePath := parts[1]
								if mirrors, ok := thirdPartyMirrors[mirrorName]; ok {
									for _, mirrorURL := range mirrors {
										resolvedURL := mirrorURL
										if !strings.HasSuffix(resolvedURL, "/") {
											resolvedURL += "/"
										}
										resolvedURL += filePath
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

		for _, fname := range fileNames {
			b, err := fs.ReadFile(sysFS, path2.Join(path, fname))
			if err == nil {
				pData.Files[fname] = string(b)
				if fname == "parent" {
					lines := strings.Split(string(b), "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line == "" || strings.HasPrefix(line, "#") {
							continue
						}
						parentRelPath := path2.Clean(path2.Join(relPath, line))
						if !strings.HasPrefix(parentRelPath, "..") {
							pData.Parents = append(pData.Parents, parentRelPath)
						}
					}
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
	Repos               map[string]*SiteData
	DominantDescription string
	DominantHomepage    string
	DominantLicense     string
	ReverseVirtuals     []string
	VirtualDeps         []string
}
type AggProject struct {
	Project  *g2.Project
	Packages []*AggPackage
}

type AggLicense struct {
	Name     string
	Count    int
	Packages []*AggPackage
	Text     string
	Aliases  []string
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

func getRepoEclasses(site *SiteData, aggPackages map[string]*AggPackage) []*AggEclass {
	eclassMap := make(map[string]*AggEclass)
	seenPackages := make(map[string]map[string]bool)

	for _, eclass := range site.DefinedEclasses {
		if _, ok := eclassMap[eclass.Name]; !ok {
			eclassMap[eclass.Name] = &AggEclass{
				Name:  eclass.Name,
				Repos: make(map[string]*SiteData),
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
									Repos: make(map[string]*SiteData),
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

func getRepoUseFlags(site *SiteData, aggPackages map[string]*AggPackage) []*AggUseFlag {
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
	Repos    map[string]*SiteData
	Packages []*AggPackage
}

type AggPackageMove struct {
	Old string
	New string
}

type AggNewsItem struct {
	g2.NewsItem
	RepoName string
}

type AggArch struct {
	Name   string
	Status string
	Repos  []*SiteData
}

type RepoGroup struct {
	Quality string
	Status  string
	Repos   []*SiteData
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

func aggregateGroupedRepos(sites []*SiteData) map[string]*RepoGroup {
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

func aggregateUseExpandDescs(sites []*SiteData) map[string]*g2.UseExpandDesc {
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

func aggregateProjects(sites []*SiteData) map[string]*AggProject {
	aggProjects := make(map[string]*AggProject)
	for _, site := range sites {
		if site.Projects != nil {
			for i := range site.Projects.Projects {
				proj := &site.Projects.Projects[i]
				if _, ok := aggProjects[proj.Email]; !ok {
					aggProjects[proj.Email] = &AggProject{Project: proj}
				}
			}
		}
	}
	return aggProjects
}

func aggregateProfiles(sites []*SiteData) map[string]*g2.AggProfile {
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

func aggregateGlobalNews(sites []*SiteData) []AggNewsItem {
	var globalNews []AggNewsItem
	for _, site := range sites {
		for _, news := range site.News {
			globalNews = append(globalNews, AggNewsItem{
				NewsItem: news,
				RepoName: site.RepoName,
			})
		}
	}
	return globalNews
}

func aggregateArches(sites []*SiteData) map[string]*AggArch {
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

func aggregateMoves(sites []*SiteData) map[string]*AggPackageMove {
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

func aggregateEclasses(sites []*SiteData) map[string]*AggEclass {
	aggEclasses := make(map[string]*AggEclass)
	for _, site := range sites {
		for _, eclass := range site.AggEclasses {
			if _, ok := aggEclasses[eclass.Name]; !ok {
				aggEclasses[eclass.Name] = &AggEclass{
					Name:  eclass.Name,
					Repos: make(map[string]*SiteData),
				}
			}
			for rName, rData := range eclass.Repos {
				aggEclasses[eclass.Name].Repos[rName] = rData
			}
			for _, pkg := range eclass.Packages {
				foundPkg := false
				for _, existingPkg := range aggEclasses[eclass.Name].Packages {
					if existingPkg.Name == pkg.Name && existingPkg.Category == pkg.Category {
						newRepos := make(map[string]*SiteData)
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
					newRepos := make(map[string]*SiteData)
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

func aggregatePackagesAndCategories(sites []*SiteData, aggProjects map[string]*AggProject) (map[string]*AggCategory, map[string]*AggPackage, map[string]*AggLicense, int) {
	aggCategories := make(map[string]*AggCategory)
	aggPackages := make(map[string]*AggPackage)
	aggLicenses := make(map[string]*AggLicense)
	totalPackages := 0

	catPkgMap := make(map[string]map[string]*AggPackage)

	for _, site := range sites {
		for _, cat := range site.Categories {
			if _, ok := aggCategories[cat.Name]; !ok {
				aggCategories[cat.Name] = &AggCategory{Name: cat.Name}
				catPkgMap[cat.Name] = make(map[string]*AggPackage)
			}
			for _, pkg := range cat.Packages {
				pkgKey := cat.Name + "/" + pkg.Name
				if _, ok := aggPackages[pkgKey]; !ok {
					aggPackages[pkgKey] = &AggPackage{Name: pkg.Name, Category: cat.Name, Repos: make(map[string]*SiteData)}
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

func aggregateValidUseExpands(sites []*SiteData) map[string]bool {
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

func prepareAggregatedData(sites []*SiteData) *AggregatedData {
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

func generateGlobalPages(outDir string, tmpl *template.Template, sites []*SiteData, data *AggregatedData, title, version, recentDurationStr string, genInfo GenerationInfo) error {
	// Generate Help Page
	if err := os.MkdirAll(filepath.Join(outDir, "help"), 0755); err != nil {
		return err
	}
	rootNode := &PageNode{Name: title, Path: ""}

	helpNode := &PageNode{Parent: rootNode, Name: "Help", Path: "help"}
	if err := renderPage(filepath.Join(outDir, "help", "index.html"), tmpl, "help.html", GenericPageContext{
		Title:       "Help & Legend",
		BaseURL:     helpNode.BaseURL(),
		Breadcrumbs: helpNode.Breadcrumbs(),
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	// Generate Stats Page
	if err := os.MkdirAll(filepath.Join(outDir, "stats"), 0755); err != nil {
		return err
	}
	statsNode := &PageNode{Parent: rootNode, Name: "Statistics", Path: "stats"}
	if err := renderPage(filepath.Join(outDir, "stats", "index.html"), tmpl, "stats.html", GenericPageContext{
		Title:       "Generation Statistics",
		BaseURL:     statsNode.BaseURL(),
		Breadcrumbs: statsNode.Breadcrumbs(),
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	// 1. Root Dashboard
	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "dashboard.html", GenericPageContext{
		Title:                title,
		BaseURL:              "",
		Repos:                sites,
		GroupedRepos:         data.GroupedRepos,
		GlobalCategories:           data.Categories,
		GlobalPackages:             data.Packages,
		Licenses:             data.Licenses,
		UseFlags:             data.UseFlags,
		Projects:             data.Projects,
		GlobalProfiles:             data.Profiles,
		Arches:               data.Arches,
		Version:              version,
		GenInfo:              genInfo,
		RecentDurationString: recentDurationStr,
		RecentGlobalNews:           data.RecentNews,
		GlobalNews:           data.GlobalNews,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	// 1b. Global News Dashboard
	if len(data.GlobalNews) > 0 {
		if err := os.MkdirAll(filepath.Join(outDir, "news"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "news", "index.html"), tmpl, "news_dashboard.html", GenericPageContext{
			Title:       "News Dashboard",
			BaseURL:     "../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "News"}},
			RecentGlobalNews:  data.RecentNews,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		// Global News Archive
		if err := os.MkdirAll(filepath.Join(outDir, "news", "archive"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "news", "archive", "index.html"), tmpl, "news_archive.html", GenericPageContext{
			Title:       "News Archive",
			BaseURL:     "../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
			GlobalNews:  data.GlobalNews,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		// Global News Articles
		for _, n := range data.GlobalNews {
			newsDir := filepath.Join(outDir, "news", "archive", n.DirName)
			if err := os.MkdirAll(newsDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", newsDir, err)
			}
			if err := renderPage(filepath.Join(newsDir, "index.html"), tmpl, "news_article.html", GenericPageContext{
				Title:       n.Title,
				BaseURL:     "../../../",
				Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
				GlobalNewsItem:    &n,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}

		}
	}

	// 2. Overlays List
	if err := os.MkdirAll(filepath.Join(outDir, "overlays"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "overlays", "index.html"), tmpl, "overlays.html", GenericPageContext{
		Title:       "Overlays",
		BaseURL:     "../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "Overlays"}},
		Repos:       sites,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	return nil
}

func generateCategoryPages(outDir string, tmpl *template.Template, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
	// 3. Global Categories
	if err := os.MkdirAll(filepath.Join(outDir, "categories"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "categories", "index.html"), tmpl, "categories.html", GenericPageContext{
		Title:       "Categories",
		BaseURL:     "../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "Categories"}},
		GlobalCategories:  data.Categories,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, cat := range data.Categories {
		catDirName := sanitizeFilename(cat.Name)
		if catDirName == "" {
			continue
		}
		// NOTE: Sanitize modifies directory generation, but not template linkages,
		// but since null-bytes shouldn't be in valid names anyway, this prevents the filesystem crash.
		catDir := filepath.Join(outDir, "categories", catDirName)
		if err := os.MkdirAll(catDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", catDir, err)
		}

		type TmplPkg struct {
			Name                  string
			ReposList             []*SiteData
			EbuildCount           int
			HighestStableVersion  template.HTML
			HighestTestingVersion template.HTML
			DominantDescription   string
			DominantHomepage      string
			DominantLicense       string
			ReverseVirtuals       []string
		}
		var tmplPkgs []TmplPkg
		for _, p := range cat.Packages {
			var allVersions []VersionData
			for _, r := range p.Repos {
				for _, c := range r.Categories {
					if c.Name == cat.Name {
						for _, pkgData := range c.Packages {
							if pkgData.Name == p.Name {
								allVersions = append(allVersions, pkgData.Versions...)
							}
						}
					}
				}
			}
			hs, ht, count := getHighestVersionsAndCount(allVersions, nil)
			tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: mapToList(p.Repos), EbuildCount: count, HighestStableVersion: hs, HighestTestingVersion: ht, DominantDescription: p.DominantDescription, DominantHomepage: p.DominantHomepage, DominantLicense: p.DominantLicense, ReverseVirtuals: p.ReverseVirtuals})
		}

		if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category.html", GenericPageContext{
			Title:       "Category: " + cat.Name,
			BaseURL:     "../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
			Category:    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generatePackagePages(outDir string, tmpl *template.Template, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
	// Global Moved Packages Pages
	for oldPath, move := range data.Moves {
		parts := strings.Split(oldPath, "/")
		if len(parts) != 2 {
			continue
		}
		oldCat, oldName := parts[0], parts[1]

		pkgExists := false
		for _, p := range data.Packages {
			if p.Category == oldCat && p.Name == oldName {
				pkgExists = true
				break
			}
		}
		if pkgExists {
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

		if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "moved_package.html", GenericPageContext{
			Title:       "Package Moved: " + oldCat + "/" + oldName,
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Packages", URL: "../../"}, {Name: oldCat, URL: "../../../categories/" + oldCat + "/"}, {Name: oldName}},
			OldName:     oldCat + "/" + oldName,
			NewName:     move.New,
			NewURL:      "../../" + newParts[0] + "/" + newParts[1] + "/",
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	// 4. Global Packages
	if err := os.MkdirAll(filepath.Join(outDir, "packages"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "packages", "index.html"), tmpl, "packages.html", GenericPageContext{
		Title:       "Packages",
		BaseURL:     "../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "Packages"}},
		GlobalPackages:    data.Packages,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, pkg := range data.Packages {
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
			if move, ok := data.Moves[pkg.Category+"/"+pkg.Name]; ok {
				newParts := strings.Split(move.New, "/")
				if len(newParts) == 2 {
					movedToName = move.New
					movedToURL = "../../" + newParts[0] + "/" + newParts[1] + "/"
				}
			}

			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "package_picker.html", GenericPageContext{
				Title:       "Package: " + pkg.Category + "/" + pkg.Name,
				BaseURL:     "../../../",
				Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Packages", URL: "../../"}, {Name: pkg.Category, URL: "../../../categories/" + pkg.Category + "/"}, {Name: pkg.Name}},
				GlobalPackage:     pkg,
				MovedToName: movedToName,
				MovedToURL:  movedToURL,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}

		}
	}
	return nil
}

func generateOtherGlobalPages(outDir string, tmpl *template.Template, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
	// Arches
	if err := os.MkdirAll(filepath.Join(outDir, "arches"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "arches", "index.html"), tmpl, "arches.html", GenericPageContext{
		Title:       "Architectures",
		BaseURL:     "../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "Architectures"}},
		Arches:      data.Arches,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, a := range data.Arches {
		archDir := filepath.Join(outDir, "arches", a.Name)
		if err := os.MkdirAll(archDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", archDir, err)
		}

		if err := renderPage(filepath.Join(archDir, "index.html"), tmpl, "arch.html", GenericPageContext{
			Title:       "Architecture: " + a.Name,
			BaseURL:     "../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Architectures", URL: "../../arches/"}, {Name: a.Name}},
			Arch:        a,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	// Profiles
	if len(data.Profiles) > 0 {
		if err := os.MkdirAll(filepath.Join(outDir, "profiles"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "profiles", "index.html"), tmpl, "profiles.html", GenericPageContext{
			Title:       "Profiles",
			BaseURL:     "../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "Profiles"}},
			GlobalProfiles:    data.Profiles,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, p := range data.Profiles {
			profDir := filepath.Join(outDir, "profiles", p.Path)
			if err := os.MkdirAll(profDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", profDir, err)
			}

			relToRoot := "../../" + strings.Repeat("../", strings.Count(p.Path, "/"))

			if err := renderPage(filepath.Join(profDir, "index.html"), tmpl, "profile.html", GenericPageContext{
				Title:       "Profile: " + p.Path,
				BaseURL:     relToRoot,
				Breadcrumbs: []Breadcrumb{{Name: title, URL: relToRoot}, {Name: "Profiles", URL: relToRoot + "profiles/"}, {Name: p.Path}},
				ProfilePath: p.Path,
				ProfileList: p.Repos,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}

	}

	// 5. Global Licenses
	if err := os.MkdirAll(filepath.Join(outDir, "licenses"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "uses"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "uses", "index.html"), tmpl, "uses.html", GenericPageContext{
		Title:       "USE Flags",
		BaseURL:     "../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "USE Flags"}},
		UseFlags:    data.UseFlags,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return err
	}

	for _, f := range data.UseFlags {
		safeName := strings.ReplaceAll(f.Name, "/", "_")
		useDir := filepath.Join(outDir, "uses", safeName)
		if err := os.MkdirAll(useDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", useDir, err)
		}

		if err := renderPage(filepath.Join(useDir, "index.html"), tmpl, "use.html", GenericPageContext{
			Title:       "USE Flag: " + f.Name,
			BaseURL:     "../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "USE Flags", URL: "../"}, {Name: f.Name}},
			GlobalUseFlag:     f,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return err
		}
	}

	if len(data.UseExpandDescs) > 0 {
		if err := os.MkdirAll(filepath.Join(outDir, "uses_expand"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "uses_expand", "index.html"), tmpl, "use_expands.html", GenericPageContext{
			Title:          "USE_EXPAND Flags",
			BaseURL:        "../",
			Breadcrumbs:    []Breadcrumb{{Name: title, URL: "../"}, {Name: "USE_EXPAND Flags"}},
			UseExpandDescs: data.UseExpandDescs,
			Version:        version,
			GenInfo:        genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
		for prefix, desc := range data.UseExpandDescs {
			useExpandDir := filepath.Join(outDir, "uses_expand", prefix)
			if err := os.MkdirAll(useExpandDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", useExpandDir, err)
			}
			if err := renderPage(filepath.Join(useExpandDir, "index.html"), tmpl, "use_expand.html", GenericPageContext{
				Title:         "USE_EXPAND: " + prefix,
				BaseURL:       "../../",
				Breadcrumbs:   []Breadcrumb{{Name: title, URL: "../../"}, {Name: "USE_EXPAND Flags", URL: "../"}, {Name: prefix}},
				UseExpandDesc: desc,
				Version:       version,
				GenInfo:       genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}
	if err := renderPage(filepath.Join(outDir, "licenses", "index.html"), tmpl, "licenses.html", GenericPageContext{
		Title:       "Licenses",
		BaseURL:     "../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "Licenses"}},
		Licenses:    data.Licenses,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, lic := range data.Licenses {
		licDirName := sanitizeFilename(lic.Name)
		if licDirName == "" {
			continue // Skip licenses that sanitize down to nothing
		}
		licDir := filepath.Join(outDir, "licenses", licDirName)
		if err := os.MkdirAll(licDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", licDir, err)
		}

		if err := renderPage(filepath.Join(licDir, "index.html"), tmpl, "license.html", GenericPageContext{
			Title:       "License: " + lic.Name,
			BaseURL:     "../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Licenses", URL: "../"}, {Name: lic.Name}},
			License:     lic,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	// 5b. Global Projects
	if len(data.Projects) > 0 {
		if err := os.MkdirAll(filepath.Join(outDir, "projects"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "projects", "index.html"), tmpl, "projects.html", GenericPageContext{
			Title:       "Projects",
			BaseURL:     "../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../"}, {Name: "Projects"}},
			Projects:    data.Projects,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, proj := range data.Projects {
			projDirName := sanitizeFilename(proj.Project.Email)
			if projDirName == "" {
				continue
			}
			projDir := filepath.Join(outDir, "projects", projDirName)
			if err := os.MkdirAll(projDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", projDir, err)
			}

			type TmplPkg struct {
				Name      string
				Category  string
				ReposList []*SiteData
			}
			var tmplPkgs []TmplPkg
			for _, p := range proj.Packages {
				tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, Category: p.Category, ReposList: mapToList(p.Repos)})
			}

			if err := renderPage(filepath.Join(projDir, "index.html"), tmpl, "project.html", GenericPageContext{
				Title:       "Project: " + proj.Project.Name,
				BaseURL:     "../../",
				Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Projects", URL: "../"}, {Name: proj.Project.Name}},
				Project:     proj,
			Packages:     tmplPkgs, // Legacy any for TmplPkgs
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}

	}

	return nil
}

func generateRepoPages(outDir string, tmpl *template.Template, sites []*SiteData, data *AggregatedData, title, version, recentDurationStr string, genInfo GenerationInfo) error {
	// 6. Repo-Specific Pages

	if err := os.MkdirAll(filepath.Join(outDir, "repos"), 0755); err != nil {
		return fmt.Errorf("creating directory repos/: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(outDir, "repos", "all"), 0755); err != nil {
		return fmt.Errorf("creating directory repos/all: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "repos", "all", "index.html"), tmpl, "overlays.html", GenericPageContext{
		Title:       "All Overlays",
		BaseURL:     "../../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "All Overlays"}},
		Repos:       sites,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	if err := generateRepoGroupPages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}

	for _, site := range sites {
		repoDir := filepath.Join(outDir, "repos", site.RepoName)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", repoDir, err)
		}

		if err := generateRepoUseFlagsPages(repoDir, tmpl, site, title, version, genInfo); err != nil {
			return err
		}

		if err := generateRepoUseExpandsPages(repoDir, tmpl, site, title, version, genInfo); err != nil {
			return err
		}

		if err := generateRepoDeprecatedMaskedInfoPages(repoDir, tmpl, site, title); err != nil {
			return err
		}

		if err := generateRepoMovedPackagesPages(repoDir, tmpl, site, title, version, genInfo); err != nil {
			return err
		}

		cutoffDate := time.Now().AddDate(0, -3, 0)
		var repoRecentNews []g2.NewsItem
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

		if err := generateRepoIndexAndStatsPages(repoDir, tmpl, site, data, title, version, recentDurationStr, genInfo, repoRecentNews); err != nil {
			return err
		}

		if err := generateRepoProfilesPages(repoDir, tmpl, site, title, version, genInfo); err != nil {
			return err
		}

		if err := generateRepoNewsPages(repoDir, tmpl, site, title, version, genInfo, repoRecentNews); err != nil {
			return err
		}

		if err := generateRepoCategoriesPages(repoDir, tmpl, site, title, version, genInfo); err != nil {
			return err
		}

		if err := generateRepoAuthorsPages(repoDir, tmpl, site, title, version, genInfo); err != nil {
			return err
		}

		if err := generateRepoPackagesPages(repoDir, tmpl, site, data, title, version, genInfo); err != nil {
			return err
		}
	}

	return nil
}

func generateRepoGroupPages(outDir string, tmpl *template.Template, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
	for _, group := range data.GroupedRepos {
		groupDirName := group.Quality + "-" + group.Status
		groupDir := filepath.Join(outDir, "repos", groupDirName)
		if err := os.MkdirAll(groupDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", groupDir, err)
		}
		if err := renderPage(filepath.Join(groupDir, "index.html"), tmpl, "repo_group.html", GenericPageContext{
			Title:       "Overlays: " + group.Quality + " - " + group.Status,
			BaseURL:     "../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Overlays: " + group.Quality + " - " + group.Status}},
			Group:       &group,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoUseFlagsPages(repoDir string, tmpl *template.Template, site *SiteData, title, version string, genInfo GenerationInfo) error {
	if len(site.AggUseFlags) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "uses"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "uses", "index.html"), tmpl, "uses.html", GenericPageContext{
			Title:       site.RepoName + " - USE Flags",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "USE Flags"}},
			UseFlags:    site.AggUseFlags,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, f := range site.AggUseFlags {
			safeName := strings.ReplaceAll(f.Name, "/", "_")
			useDir := filepath.Join(repoDir, "uses", safeName)
			if err := os.MkdirAll(useDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", useDir, err)
			}

			if err := renderPage(filepath.Join(useDir, "index.html"), tmpl, "use.html", GenericPageContext{
				Title:       "USE Flag: " + f.Name,
				BaseURL:     "../../../../",
				Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "USE Flags", URL: "../"}, {Name: f.Name}},
				GlobalUseFlag:     f,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateRepoUseExpandsPages(repoDir string, tmpl *template.Template, site *SiteData, title, version string, genInfo GenerationInfo) error {
	if len(site.UseExpandDescs) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "uses_expand"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "uses_expand", "index.html"), tmpl, "repo_use_expands.html", GenericPageContext{
			Title:       site.RepoName + " - USE_EXPAND Flags",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "USE_EXPAND Flags"}},
			Repo:        site,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
		for prefix, desc := range site.UseExpandDescs {
			useExpandDir := filepath.Join(repoDir, "uses_expand", prefix)
			if err := os.MkdirAll(useExpandDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", useExpandDir, err)
			}
			if err := renderPage(filepath.Join(useExpandDir, "index.html"), tmpl, "repo_use_expand.html", GenericPageContext{
				Title:         site.RepoName + " - USE_EXPAND: " + prefix,
				BaseURL:       "../../../../",
				Breadcrumbs:   []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "USE_EXPAND Flags", URL: "../"}, {Name: prefix}},
				Repo:          site,
				UseExpandDesc: desc,
				Version:       version,
				GenInfo:       genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}
	return nil
}

func generateRepoDeprecatedMaskedInfoPages(repoDir string, tmpl *template.Template, site *SiteData, title string) error {
	if len(site.Deprecated) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "deprecated"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "deprecated", "index.html"), tmpl, "repo_deprecated.html", GenericPageContext{
			Title:       site.RepoName + " - Deprecated",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Deprecated Packages"}},
			Repo:        site,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	if len(site.Masked) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "masked"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "masked", "index.html"), tmpl, "repo_masked.html", GenericPageContext{
			Title:       site.RepoName + " - Masked",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Masked Packages"}},
			Repo:        site,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}

	if len(site.InfoVars) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "info_vars"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "info_vars", "index.html"), tmpl, "repo_info_vars.html", GenericPageContext{
			Title:       site.RepoName + " - Info Vars",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Info Vars"}},
			Repo:        site,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	if len(site.InfoPkgs) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "info_pkgs"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "info_pkgs", "index.html"), tmpl, "repo_info_pkgs.html", GenericPageContext{
			Title:       site.RepoName + " - Info Packages",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Info Packages"}},
			Repo:        site,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoMovedPackagesPages(repoDir string, tmpl *template.Template, site *SiteData, title, version string, genInfo GenerationInfo) error {
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

		if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "moved_package.html", GenericPageContext{
			Title:       fmt.Sprintf("%s - %s/%s (Moved)", site.RepoName, oldCat, oldName),
			BaseURL:     "../../../../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../../../"}, {Name: site.RepoName, URL: "../../../../"}, {Name: "Categories", URL: "../../../"}, {Name: oldCat, URL: "../../"}, {Name: oldName}},
			Repo:        site,
			OldName:     oldCat + "/" + oldName,
			NewName:     move.New,
			NewURL:      "../../../" + newParts[0] + "/packages/" + newParts[1] + "/",
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoIndexAndStatsPages(repoDir string, tmpl *template.Template, site *SiteData, data *AggregatedData, title, version, recentDurationStr string, genInfo GenerationInfo, repoRecentNews []g2.NewsItem) error {
	if err := renderPage(filepath.Join(repoDir, "index.html"), tmpl, "repo_index.html", GenericPageContext{
		Title:                 site.RepoName,
		BaseURL:               "../../",
		Breadcrumbs:           []Breadcrumb{{Name: title, URL: "../../"}, {Name: "Overlays", URL: "../../overlays/"}, {Name: site.RepoName}},
		Repo:                  site,
		PackageCount:          site.PackageCount,
		Version:               version,
		GenInfo:               genInfo,
		RecentDurationString:  recentDurationStr,
		RecentRepoNews:            repoRecentNews,
		GlobalCategoriesCount: len(data.Categories),
		GlobalPackagesCount:   data.TotalPackages,
		GlobalLicensesCount:   len(data.Licenses),
		GlobalProfilesCount:   len(data.Profiles),
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(repoDir, "stats"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(repoDir, "stats", "index.html"), tmpl, "repo_stats.html", GenericPageContext{
		Title:       site.RepoName + " - Statistics",
		BaseURL:     "../../../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Overlays", URL: "../../../overlays/"}, {Name: site.RepoName, URL: "../"}, {Name: "Statistics"}},
		Repo:        site,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}
	return nil
}

func generateRepoProfilesPages(repoDir string, tmpl *template.Template, site *SiteData, title, version string, genInfo GenerationInfo) error {
	if len(site.Profiles) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "profiles"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "profiles", "index.html"), tmpl, "repo_profiles.html", GenericPageContext{
			Title:       site.RepoName + " - Profiles",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Profiles"}},
			Repo:        site,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, p := range site.Profiles {
			profDir := filepath.Join(repoDir, "profiles", p.Path)
			if err := os.MkdirAll(profDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", profDir, err)
			}

			relToRoot := "../../../../" + strings.Repeat("../", strings.Count(p.Path, "/"))

			if err := renderPage(filepath.Join(profDir, "index.html"), tmpl, "repo_profile.html", GenericPageContext{
				Title:       site.RepoName + " - Profile: " + p.Path,
				BaseURL:     relToRoot,
				Breadcrumbs: []Breadcrumb{{Name: title, URL: relToRoot}, {Name: site.RepoName, URL: relToRoot + "repos/" + site.RepoName + "/"}, {Name: "Profiles", URL: relToRoot + "repos/" + site.RepoName + "/profiles/"}, {Name: p.Path}},
				RepoName:    site.RepoName,
				ProfilePath: p.Path,
				RepoProfile:     &p,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}

			for fName, fContent := range p.Files {
				if err := renderPage(filepath.Join(profDir, fName+".html"), tmpl, "repo_profile_file.html", GenericPageContext{
					Title:       site.RepoName + " - Profile File: " + fName,
					BaseURL:     relToRoot,
					Breadcrumbs: []Breadcrumb{{Name: title, URL: relToRoot}, {Name: site.RepoName, URL: relToRoot + "repos/" + site.RepoName + "/"}, {Name: "Profiles", URL: relToRoot + "repos/" + site.RepoName + "/profiles/"}, {Name: p.Path, URL: "index.html"}, {Name: fName}},
					RepoName:    site.RepoName,
					ProfilePath: p.Path,
					RepoProfile:     &p,
					FileName:    fName,
					FileContent: fContent,
					Version:     version,
					GenInfo:     genInfo,
				}); err != nil {
					return fmt.Errorf("rendering page: %w", err)
				}
			}
		}
	}
	return nil
}

func generateRepoNewsPages(repoDir string, tmpl *template.Template, site *SiteData, title, version string, genInfo GenerationInfo, repoRecentNews []g2.NewsItem) error {
	if len(site.News) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "news"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "news", "index.html"), tmpl, "news_dashboard.html", GenericPageContext{
			Title:       site.RepoName + " - News Dashboard",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Overlays", URL: "../../../overlays/"}, {Name: site.RepoName, URL: "../"}, {Name: "News"}},
			RecentRepoNews:  repoRecentNews,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		// Repo News Archive
		if err := os.MkdirAll(filepath.Join(repoDir, "news", "archive"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "news", "archive", "index.html"), tmpl, "news_archive.html", GenericPageContext{
			Title:       site.RepoName + " - News Archive",
			BaseURL:     "../../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: "Overlays", URL: "../../../../overlays/"}, {Name: site.RepoName, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
			News:        site.News,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		// Repo News Articles
		for _, n := range site.News {
			newsDir := filepath.Join(repoDir, "news", "archive", n.DirName)
			if err := os.MkdirAll(newsDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", newsDir, err)
			}
			if err := renderPage(filepath.Join(newsDir, "index.html"), tmpl, "news_article.html", GenericPageContext{
				Title:       n.Title,
				BaseURL:     "../../../../../",
				Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../../"}, {Name: "Overlays", URL: "../../../../../overlays/"}, {Name: site.RepoName, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
				RepoNewsItem:    &n,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}
	return nil
}

func generateRepoCategoriesPages(repoDir string, tmpl *template.Template, site *SiteData, title, version string, genInfo GenerationInfo) error {
	if err := os.MkdirAll(filepath.Join(repoDir, "categories"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(repoDir, "categories", "index.html"), tmpl, "categories.html", GenericPageContext{
		Title:       site.RepoName + " - Categories",
		BaseURL:     "../../../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Categories"}},
		RepoCategories:  site.Categories,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	for _, cat := range site.Categories {
		catDirName := sanitizeFilename(cat.Name)
		if catDirName == "" {
			continue
		}
		catDir := filepath.Join(repoDir, "categories", catDirName)
		if err := os.MkdirAll(catDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", catDir, err)
		}

		type TmplPkg struct {
			Name                  string
			ReposList             []*SiteData
			EbuildCount           int
			HighestStableVersion  template.HTML
			HighestTestingVersion template.HTML
			DominantDescription   string
			DominantHomepage      string
			DominantLicense       string
			ReverseVirtuals       []string
		}
		var tmplPkgs []TmplPkg
		for _, p := range cat.Packages {
			tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: []*SiteData{site}, EbuildCount: p.EbuildCount, HighestStableVersion: p.HighestStableVersion, HighestTestingVersion: p.HighestTestingVersion, DominantDescription: p.DominantDescription, DominantHomepage: p.DominantHomepage, DominantLicense: p.DominantLicense, ReverseVirtuals: p.ReverseVirtuals})
		}

		if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category.html", GenericPageContext{
			Title:       "Category: " + cat.Name,
			BaseURL:     "../../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
			Category:    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoAuthorsPages(repoDir string, tmpl *template.Template, site *SiteData, title, version string, genInfo GenerationInfo) error {
	if len(site.Authors) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "authors"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "authors", "index.html"), tmpl, "authors.html", GenericPageContext{
			Title:       site.RepoName + " - Authors",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Authors"}},
			Authors:     site.Authors,
			Repo:        site,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoPackagesPages(repoDir string, tmpl *template.Template, site *SiteData, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
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

	if err := renderPage(filepath.Join(repoDir, "packages", "index.html"), tmpl, "repo_packages.html", GenericPageContext{
		Title:       site.RepoName + " - Packages",
		BaseURL:     "../../../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Packages"}},
		RepoPackages:    repoPkgs,
		Repo:        site,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	aggPackagesMap := make(map[string]*AggPackage)
	for _, p := range data.Packages {
		aggPackagesMap[p.Category+"/"+p.Name] = p
	}
	repoUseFlags := getRepoUseFlags(site, aggPackagesMap)

	if len(site.AggEclasses) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "eclasses"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "eclasses", "index.html"), tmpl, "repo_eclasses.html", GenericPageContext{
			Title:       site.RepoName + " - Eclasses",
			BaseURL:     "../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Eclasses"}},
			Eclasses:    site.AggEclasses,
			Repo:        site,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return err
		}

		for _, ec := range site.AggEclasses {
			safeName := sanitizeFilename(ec.Name)
			if safeName == "" {
				continue
			}
			ecDir := filepath.Join(repoDir, "eclasses", safeName)
			if err := os.MkdirAll(ecDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", ecDir, err)
			}

			if err := renderPage(filepath.Join(ecDir, "index.html"), tmpl, "repo_eclass.html", GenericPageContext{
				Title:       site.RepoName + " - Eclass: " + ec.Name,
				BaseURL:     "../../../../",
				Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "Eclasses", URL: "../"}, {Name: ec.Name}},
				Eclass:      ec,
				Repo:        site,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return err
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(repoDir, "uses"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(repoDir, "uses", "index.html"), tmpl, "repo_uses.html", GenericPageContext{
		Title:       site.RepoName + " - USE Flags",
		BaseURL:     "../../../",
		Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "USE Flags"}},
		UseFlags:    repoUseFlags,
		Repo:        site,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return err
	}

	for _, f := range repoUseFlags {
		safeName := strings.ReplaceAll(f.Name, "/", "_")
		useDir := filepath.Join(repoDir, "uses", safeName)
		if err := os.MkdirAll(useDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", useDir, err)
		}

		if err := renderPage(filepath.Join(useDir, "index.html"), tmpl, "repo_use.html", GenericPageContext{
			Title:       site.RepoName + " - USE Flag: " + f.Name,
			BaseURL:     "../../../../",
			Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "USE Flags", URL: "../"}, {Name: f.Name}},
			GlobalUseFlag:     f,
			Repo:        site,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return err
		}
	}

	for _, pkg := range repoPkgs {
		pkgDir := filepath.Join(repoDir, "categories", pkg.Category, "packages", pkg.Name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", pkgDir, err)
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

		if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "repo_package.html", GenericPageContext{
			Title:         fmt.Sprintf("%s - %s/%s", site.RepoName, pkg.Category, pkg.Name),
			BaseURL:       "../../../../../../",
			Breadcrumbs:   []Breadcrumb{{Name: title, URL: "../../../../../../"}, {Name: site.RepoName, URL: "../../../../"}, {Name: "Categories", URL: "../../../"}, {Name: pkg.Category, URL: "../../"}, {Name: pkg.Name}},
			Repo:          site,
			RepoPackage:       &pkg,
			MovedToName:   movedToName,
			MovedToURL:    movedToURL,
			Version:       version,
			GenInfo:       genInfo,
			ValidLicenses: data.ValidLicenses,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, md := range pkg.ManifestData {
			if len(md.Versions) > 0 {
				mdDir := filepath.Join(pkgDir, "manifest", md.Entry.Filename)
				if err := os.MkdirAll(mdDir, 0755); err != nil {
					return fmt.Errorf("creating directory %s: %w", mdDir, err)
				}

				if err := renderPage(filepath.Join(mdDir, "index.html"), tmpl, "repo_package_manifest.html", GenericPageContext{
					Title:       fmt.Sprintf("%s - %s/%s - Manifest: %s", site.RepoName, pkg.Category, pkg.Name, md.Entry.Filename),
					BaseURL:     "../../../../../../../../",
					Breadcrumbs: []Breadcrumb{{Name: title, URL: "../../../../../../../../"}, {Name: site.RepoName, URL: "../../../../../../"}, {Name: "Categories", URL: "../../../../../"}, {Name: pkg.Category, URL: "../../../../"}, {Name: pkg.Name, URL: "../../"}, {Name: "Manifest"}, {Name: md.Entry.Filename}},
					Repo:        site,
					RepoPackage:     &pkg,
					Manifest:    &md,
					Version:     version,
					GenInfo:     genInfo,
				}); err != nil {
					return fmt.Errorf("rendering page: %w", err)
				}
			}
		}
		for _, v := range pkg.Versions {
			versionStr := v.Version
			if v.Ebuild != nil && v.Ebuild.Vars != nil && v.Ebuild.Vars["PV"] != "" {
				versionStr = v.Ebuild.Vars["PV"]
			}

			ebuildDir := filepath.Join(pkgDir, "ebuild", versionStr)
			if err := os.MkdirAll(ebuildDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", ebuildDir, err)
			}

			var filteredManifest []ManifestEntryData
			if pkg.Manifest != nil {
				for _, md := range pkg.ManifestData {
					for _, mv := range md.Versions {
						if mv == v.Version || mv == versionStr {
							filteredManifest = append(filteredManifest, md)
							break
						}
					}
				}
			}

			if err := renderPage(filepath.Join(ebuildDir, "index.html"), tmpl, "ebuild_details.html", GenericPageContext{
				Title:            fmt.Sprintf("%s - %s/%s-%s", site.RepoName, pkg.Category, pkg.Name, versionStr),
				BaseURL:          "../../../../../../../../",
				Breadcrumbs:      []Breadcrumb{{Name: title, URL: "../../../../../../../../"}, {Name: site.RepoName, URL: "../../../../../../"}, {Name: "Categories", URL: "../../../../../"}, {Name: pkg.Category, URL: "../../../../"}, {Name: "Packages", URL: "../../../"}, {Name: pkg.Name, URL: "../../"}, {Name: "Ebuild", URL: "../"}, {Name: versionStr}},
				Repo:             site,
				RepoPackage:          &pkg,
				VersionData:      &v,
				FilteredManifest: filteredManifest,
				Version:          version,
				GenInfo:          genInfo,
				ValidLicenses:    data.ValidLicenses,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}
	return nil
}

func generateSite(outDir string, sites []*SiteData, recentDuration time.Duration, recentDurationStr string, genInfo GenerationInfo) error {
	log.Printf("Starting site generation (v%s) with %d repos to %s", version, len(sites), outDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	for _, site := range sites {
		populatePkgUseFlags(site)
	}

	// Generate search index
	if err := generateSearchIndex(outDir, sites); err != nil {
		log.Printf("Warning: failed to generate search index: %v", err)
	}

	tmpl, err := GetSiteTemplates()
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	for _, site := range sites {
		aggPackagesMap := make(map[string]*AggPackage)
		for _, cat := range site.Categories {
			for _, pkg := range cat.Packages {
				pkgKey := cat.Name + "/" + pkg.Name
				aggPackagesMap[pkgKey] = &AggPackage{
					Name:                pkg.Name,
					Category:            cat.Name,
					Repos:               map[string]*SiteData{site.RepoName: site},
					DominantDescription: pkg.DominantDescription,
					DominantHomepage:    pkg.DominantHomepage,
					DominantLicense:     pkg.DominantLicense,
					ReverseVirtuals:     pkg.ReverseVirtuals,
					VirtualDeps:         pkg.VirtualDeps,
				}
			}
		}
		site.AggEclasses = getRepoEclasses(site, aggPackagesMap)
	}

	// Prepare Immutable Render Context
	data := prepareAggregatedData(sites)

	// Title
	title := "Gentoo Packages"
	if len(sites) == 1 {
		title = sites[0].Title
	}

	// Render Phases
	if err := generateGlobalPages(outDir, tmpl, sites, data, title, version, recentDurationStr, genInfo); err != nil {
		return err
	}

	if err := generateCategoryPages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}

	if err := generatePackagePages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}

	if err := generateOtherGlobalPages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}

	if err := generateRepoPages(outDir, tmpl, sites, data, title, version, recentDurationStr, genInfo); err != nil {
		return err
	}

	totalNodes := len(data.Categories) + data.TotalPackages + len(data.Profiles) + len(data.GlobalNews) + len(data.Moves) + len(data.Eclasses)
	for _, pkg := range data.Packages {
		for _, repoSite := range pkg.Repos {
			for _, cat := range repoSite.Categories {
				if cat.Name == pkg.Category {
					for _, repoPkg := range cat.Packages {
						if repoPkg.Name == pkg.Name {
							totalNodes += len(repoPkg.Versions)
						}
					}
				}
			}
		}
	}

	log.Printf("[DONE] Site generation complete. Total nodes generated: %d", totalNodes)

	return nil
}

func renderPage(path string, tmpl *template.Template, name string, data interface{}) error {
	log.Printf("Rendering page %s using template %s", path, name)

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

func (cfg *MainArgConfig) cmdSiteRemote(repositoriesFile string, outDir string, recentDuration time.Duration, recentDurationStr string, fastGit bool, useZip bool, concurrency int, retries int, continueOnError bool, persistentDir string, reposConfPath string) error {
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
						Name: s.Name,
						Sources: []g2.RepositorySource{{Type: "git", Text: syncURI}},
					})
				} else if loc != "" {
					repos.Repositories = append(repos.Repositories, g2.Repository{
						Name: s.Name,
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

	if persistentDir != "" {
		tmpDir = persistentDir
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("creating persistent dir: %w", err)
		}
		cleanup = func() {}
	} else {
		var err error
		tmpDir, err = os.MkdirTemp("", "g2-sitegen-*")
		if err != nil {
			return fmt.Errorf("creating temp dir: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(tmpDir) }
	}
	defer cleanup()

	var allSites []*SiteData
	var allSitesMu sync.Mutex
	var lastLogTime time.Time

	var processedRepos int
	var totalCategories int
	var totalPackages int
	var totalPackageVersions int

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
			if err := FetchRepo(ctx, gitUrl, repoPath, useZip, persistentDir != "", retries); err != nil {
				log.Printf("Failed to fetch %s: %v", repo.Name, err)
				if !continueOnError {
					return fmt.Errorf("fetching %s: %w", repo.Name, err)
				}
				return nil
			}
			checkoutTime := time.Since(t0)
      freeSpace, err := getFreeSpace(repoPath)
      procMem := getProcessMemUsage()

      var memStats runtime.MemStats
      runtime.ReadMemStats(&memStats)

      if err == nil {
        log.Printf(
          "[DONE] Finished fetching repository %s in %s. Free space: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
          repo.Name,
          checkoutTime,
          float64(freeSpace)/(1024*1024),
          float64(procMem)/(1024*1024),
          float64(memStats.Alloc)/(1024*1024),
        )
      } else {
        log.Printf(
          "[DONE] Finished fetching repository %s in %s. Process Memory: %.2f MB. Go Alloc: %.2f MB",
          repo.Name,
          checkoutTime,
          float64(procMem)/(1024*1024),
          float64(memStats.Alloc)/(1024*1024),
        )
      }

      log.Printf("[START] Parsing repository: %s", repo.Name)
      
			size, err := getDirSize(repoPath)
			var gitSize string
			if err == nil {
				gitSize = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
			}

			repoCopy := repo

			t1 := time.Now()
			siteData, err := parseRepo(os.DirFS(repoPath), ".", repo.Name, fastGit, &repoCopy, SourceURL(gitUrl))
			if err != nil {
				log.Printf("Failed to parse repo %s: %v", repo.Name, err)
				return nil
			}
			processTime := time.Since(t1)
			freeSpaceAfter, err := getFreeSpace(repoPath)
			procMemAfter := getProcessMemUsage()

			var memStatsAfter runtime.MemStats
			runtime.ReadMemStats(&memStatsAfter)

			nodeCount := 0
			if siteData != nil {
				nodeCount = len(siteData.Categories) + siteData.PackageCount + len(siteData.Profiles) + len(siteData.News) + len(siteData.Moves) + len(siteData.DefinedEclasses)
				for _, cat := range siteData.Categories {
					for _, pkg := range cat.Packages {
						nodeCount += len(pkg.Versions)
					}
				}
			}

			if err == nil {
				log.Printf(
					"[DONE] Finished parsing repository %s in %s. Free space: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
					repo.Name,
					processTime,
					float64(freeSpaceAfter)/(1024*1024),
					float64(procMemAfter)/(1024*1024),
					float64(memStatsAfter.Alloc)/(1024*1024),
					nodeCount,
				)
			} else {
				log.Printf(
					"[DONE] Finished parsing repository %s in %s. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
					repo.Name,
					processTime,
					float64(procMemAfter)/(1024*1024),
					float64(memStatsAfter.Alloc)/(1024*1024),
					nodeCount,
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
	// Sort the resulting sites alphabetically by RepoName for deterministic ordering
	sort.Slice(allSites, func(i, j int) bool {
		return allSites[i].RepoName < allSites[j].RepoName
	})

	finalMemUsage := getProcessMemUsage()
	log.Printf("--------------------------------------------------")
	log.Printf("[FINAL SUMMARY] Repository Processing Complete")
	log.Printf("Total Repositories:      %d", len(allSites))
	log.Printf("Total Categories:        %d", totalCategories)
	log.Printf("Total Packages:          %d", totalPackages)
	log.Printf("Total Package Versions:  %d", totalPackageVersions)
	log.Printf("Final Memory Usage:      %.2f MB", float64(finalMemUsage)/(1024*1024))
	log.Printf("--------------------------------------------------")

	log.Printf("Generating integrated site (v%s) for %d repos", version, len(allSites))
	if err := generateSite(outDir, allSites, recentDuration, recentDurationStr, GenerationInfo{}); err != nil {
		return fmt.Errorf("generating integrated site: %w", err)
	}

	log.Println("Remote site generation complete.")
	return nil
}

func mapToList(m map[string]*SiteData) []*SiteData {
	var l []*SiteData
	for _, v := range m {
		l = append(l, v)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].RepoName < l[j].RepoName })
	return l
}
