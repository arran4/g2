package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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

func (cfg *MainArgConfig) cmdSite(args []string) error {
	fs := flag.NewFlagSet("site", flag.ExitOnError)
	outDir := fs.String("out", "site_out", "Output directory for the generated site")
	repoDir := fs.String("repo", ".", "Repository root directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	log.Printf("Generating site from %s into %s", *repoDir, *outDir)

	siteData, err := parseRepo(*repoDir)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	if err := generateSite(*outDir, siteData); err != nil {
		return fmt.Errorf("generating site: %w", err)
	}

	log.Println("Site generation complete.")
	return nil
}

func parseRepo(repoDir string) (*SiteData, error) {
	site := &SiteData{
		Title: "Gentoo Packages",
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

	tmpl := template.Must(template.New("layout").Parse(layoutTemplate))
	tmpl = template.Must(tmpl.New("index").Parse(indexTemplate))
	tmpl = template.Must(tmpl.New("category").Parse(categoryTemplate))
	tmpl = template.Must(tmpl.New("package").Parse(packageTemplate))

	// Generate index
	if err := renderPage(filepath.Join(outDir, "index.html"), tmpl, "index", map[string]interface{}{
		"Title":      site.Title,
		"Categories": site.Categories,
	}); err != nil {
		return err
	}

	for _, cat := range site.Categories {
		catDir := filepath.Join(outDir, cat.Name)
		if err := os.MkdirAll(catDir, 0755); err != nil {
			return err
		}

		// Generate category index
		if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category", map[string]interface{}{
			"Title":    fmt.Sprintf("%s - %s", site.Title, cat.Name),
			"Category": cat,
		}); err != nil {
			return err
		}

		for _, pkg := range cat.Packages {
			pkgDir := filepath.Join(catDir, pkg.Name)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return err
			}

			// Generate package index
			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "package", map[string]interface{}{
				"Title":   fmt.Sprintf("%s - %s/%s", site.Title, cat.Name, pkg.Name),
				"Package": pkg,
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

	return tmpl.ExecuteTemplate(f, "layout", data)
}
