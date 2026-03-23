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
	Categories []CategoryData
}

type CategoryData struct {
	Name     string
	Packages []PackageData
}

type PackageData struct {
	Name     string
	Category string
	Versions []VersionData
	Metadata interface{}
	Manifest *g2.Manifest
}

type VersionData struct {
	Version string
	Ebuild  *g2.Ebuild
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
		cleanup = func() { os.RemoveAll(tmpDir) }

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

func parseRepo(repoDir string, title string) (*SiteData, error) {
	site := &SiteData{
		Title: title,
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

				pkgData.Versions = append(pkgData.Versions, VersionData{
					Version: version,
					Ebuild:  ebuild,
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
				pkgData.Metadata = metadata
			}

			// Read Manifest
			manifestPath := filepath.Join(pkgPath, "Manifest")
			manifest, err := g2.ParseManifest(manifestPath)
			if err == nil {
				pkgData.Manifest = manifest
			}

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

	// Generate index
	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "index.html", map[string]interface{}{
		"Title":      site.Title,
		"Categories": site.Categories,
		"Version":    "v1",
	}); err != nil {
		return err
	}

	for _, cat := range site.Categories {
		catDir := filepath.Join(outDir, cat.Name)
		if err := os.MkdirAll(catDir, 0755); err != nil {
			return err
		}

		// Generate category index
		if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category.html", map[string]interface{}{
			"Title":    fmt.Sprintf("%s - %s", site.Title, cat.Name),
			"Category": cat,
			"Version":  "v1",
		}); err != nil {
			return err
		}

		for _, pkg := range cat.Packages {
			pkgDir := filepath.Join(catDir, pkg.Name)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return err
			}

			// Generate package index
			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "package.html", map[string]interface{}{
				"Title":   fmt.Sprintf("%s - %s/%s", site.Title, cat.Name, pkg.Name),
				"Package": pkg,
				"Version": "v1",
			}); err != nil {
				return err
			}
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
	defer f.Close()

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
		defer resp.Body.Close()
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
	defer os.RemoveAll(tmpDir)

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
		"Version":    "v1",
	}); err != nil {
		return err
	}

	log.Println("Remote site generation complete.")
	return nil
}
