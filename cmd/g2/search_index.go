package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"

	"github.com/arran4/g2/templates"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arran4/g2"
)

type SearchDocument struct {
	ID              int      `json:"id"`
	Overlay         string   `json:"overlay"`
	Category        string   `json:"category"`
	Package         string   `json:"package"`
	FullName        string   `json:"full_name"`
	Version         string   `json:"version"`
	VersionSortKey  string   `json:"version_sort_key"`
	Description     string   `json:"description"`
	Urls            []string `json:"urls"`
	Licenses        []string `json:"licenses"`
	EAPI            string   `json:"eapi"`
	Slot            string   `json:"slot"`
	Inherits        []string `json:"inherits"`
	Uses            []string `json:"uses"`
	UseDescriptions []string `json:"use_descriptions"`
	Keywords        []string `json:"keywords"`
	Arches          []string `json:"arches"`
	Mask            string   `json:"mask"` // "none", "soft", "hard"
	Depends         []string `json:"depends"`
	Rdepends        []string `json:"rdepends"`
	Bdepends        []string `json:"bdepends"`
	Pdepends        []string `json:"pdepends"`
	DependedBy      []string `json:"depended_by"`
	RdependedBy     []string `json:"rdepended_by"`
	RawDepends      string   `json:"raw_depends"`
	RawRdepends     string   `json:"raw_rdepends"`
	RawBdepends     string   `json:"raw_bdepends"`
	RawPdepends     string   `json:"raw_pdepends"`
	RawRequiredUse  string   `json:"raw_required_use"`
	ManifestFiles   []string `json:"manifest_files"`
	SearchText      string   `json:"search_text"`
	PageURL         string   `json:"page_url"`
}

type SearchManifest struct {
	DocumentCount int      `json:"document_count"`
	DataFiles     []string `json:"data_files"`
}

func generateSearchData(outDir, outZip string, sites []*SiteData) error {
	var documents []SearchDocument
	docID := 0

	dependedBy := make(map[string]map[string]bool)
	rdependedBy := make(map[string]map[string]bool)

	pkgRegex := regexp.MustCompile(`([a-zA-Z0-9_][a-zA-Z0-9_\-\+]*\/[a-zA-Z0-9_][a-zA-Z0-9_\-\+]+)`)

	for _, site := range sites {
		overlayName := site.RepoName
		for _, cat := range site.Categories {
			for _, pkg := range cat.Packages {
				fullName := cat.Name + "/" + pkg.Name

				for _, ver := range pkg.Versions {
					docID++

					verStr := ver.Version
					eapi := ""
					slot := "0"
					desc := ""
					var urls []string
					var licenses []string
					var keywords []string
					var inherits []string
					var uses []string
					var useDescriptions []string
					var depends []string
					var rdepends []string
					var bdepends []string
					var pdepends []string
					var rawDepends, rawRdepends, rawBdepends, rawPdepends, rawRequiredUse string
					var arches []string

					if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
						if v := ver.Ebuild.Vars["PV"]; v != "" {
							verStr = v
						}
						eapi = ver.Ebuild.Vars["EAPI"]
						slot = ver.Ebuild.Vars["SLOT"]
						desc = ver.Ebuild.Vars["DESCRIPTION"]

						homepage := ver.Ebuild.Vars["HOMEPAGE"]
						urls = append(urls, strings.Fields(homepage)...)

						licenseStr := ver.Ebuild.Vars["LICENSE"]
						for _, l := range strings.Fields(licenseStr) {
							if l != "||" && l != "(" && l != ")" && !strings.HasPrefix(l, "?") {
								licenses = append(licenses, l)
								if site.LicenseMapping != nil {
									if aliases, ok := site.LicenseMapping[l]; ok {
										licenses = append(licenses, aliases...)
									}
								}
							}
						}

						keywordStr := ver.Ebuild.Vars["KEYWORDS"]
						for _, kw := range strings.Fields(keywordStr) {
							keywords = append(keywords, kw)
							arch := strings.TrimPrefix(kw, "~")
							arch = strings.TrimPrefix(arch, "-")
							if arch != "" {
								arches = append(arches, arch)
							}
						}

						iuseStr := ver.Ebuild.Vars["IUSE"]
						for _, u := range strings.Fields(iuseStr) {
							u = strings.TrimPrefix(u, "+")
							u = strings.TrimPrefix(u, "-")
							uses = append(uses, u)
						}

						if d := ver.Ebuild.Vars["DEPEND"]; d != "" {
							rawDepends = d
							matches := pkgRegex.FindAllString(d, -1)
							for _, m := range matches {
								depends = append(depends, m)
								if dependedBy[m] == nil {
									dependedBy[m] = make(map[string]bool)
								}
								dependedBy[m][fullName] = true
							}
						}

						if d := ver.Ebuild.Vars["RDEPEND"]; d != "" {
							rawRdepends = d
							matches := pkgRegex.FindAllString(d, -1)
							for _, m := range matches {
								rdepends = append(rdepends, m)
								if rdependedBy[m] == nil {
									rdependedBy[m] = make(map[string]bool)
								}
								rdependedBy[m][fullName] = true
							}
						}

						if d := ver.Ebuild.Vars["BDEPEND"]; d != "" {
							rawBdepends = d
							matches := pkgRegex.FindAllString(d, -1)
							bdepends = append(bdepends, matches...)
						}

						if d := ver.Ebuild.Vars["PDEPEND"]; d != "" {
							rawPdepends = d
							matches := pkgRegex.FindAllString(d, -1)
							pdepends = append(pdepends, matches...)
						}

						if d := ver.Ebuild.Vars["REQUIRED_USE"]; d != "" {
							rawRequiredUse = d
						}

						if inh := ver.Ebuild.Vars["INHERITED"]; inh != "" {
							inherits = strings.Fields(inh)
						}
					}

					for _, pUse := range pkg.PkgUseFlags {
						if pUse.Versions[verStr] != "" {
							useDescriptions = append(useDescriptions, pUse.Desc)
						}
					}

					depends = deduplicateStrings(depends)
					rdepends = deduplicateStrings(rdepends)
					bdepends = deduplicateStrings(bdepends)
					pdepends = deduplicateStrings(pdepends)
					licenses = deduplicateStrings(licenses)
					keywords = deduplicateStrings(keywords)
					arches = deduplicateStrings(arches)
					uses = deduplicateStrings(uses)
					urls = deduplicateStrings(urls)
					useDescriptions = deduplicateStrings(useDescriptions)

					mask := "none"
					allMasked := true
					allTesting := true
					for _, kw := range keywords {
						if !strings.HasPrefix(kw, "-") && !strings.HasPrefix(kw, "~") {
							allMasked = false
							allTesting = false
						} else if strings.HasPrefix(kw, "~") {
							allMasked = false
						}
					}
					if len(keywords) > 0 {
						if allMasked {
							mask = "hard"
						} else if allTesting {
							mask = "soft"
						}
					}

					searchText := strings.ToLower(fmt.Sprintf("%s %s %s %s %s", fullName, desc, strings.Join(uses, " "), strings.Join(urls, " "), strings.Join(useDescriptions, " ")))

					var manifestFiles []string
					for _, m := range pkg.ManifestData {
						for _, mv := range m.Versions {
							if mv == ver.Version || mv == verStr {
								manifestFiles = append(manifestFiles, m.Entry.Filename)
							}
						}
					}

					doc := SearchDocument{
						ID:              docID,
						Overlay:         overlayName,
						Category:        cat.Name,
						Package:         pkg.Name,
						FullName:        fullName,
						Version:         verStr,
						Description:     desc,
						Urls:            urls,
						Licenses:        licenses,
						EAPI:            eapi,
						Slot:            slot,
						Inherits:        inherits,
						Uses:            uses,
						UseDescriptions: useDescriptions,
						Keywords:        keywords,
						Arches:          arches,
						Mask:            mask,
						Depends:         depends,
						Rdepends:        rdepends,
						Bdepends:        bdepends,
						Pdepends:        pdepends,
						RawDepends:      rawDepends,
						RawRdepends:     rawRdepends,
						RawBdepends:     rawBdepends,
						RawPdepends:     rawPdepends,
						RawRequiredUse:  rawRequiredUse,
						DependedBy:      []string{},
						RdependedBy:     []string{},
						ManifestFiles:   manifestFiles,
						SearchText:      searchText,
						PageURL:         fmt.Sprintf("../repos/%s/categories/%s/packages/%s/ebuild/%s/index.html", site.RepoName, cat.Name, pkg.Name, verStr),
					}

					doc.VersionSortKey = g2.PadVersionTokens(verStr)

					documents = append(documents, doc)
				}
			}
		}
	}

	for i := range documents {
		if deps, ok := dependedBy[documents[i].FullName]; ok {
			for dep := range deps {
				documents[i].DependedBy = append(documents[i].DependedBy, dep)
			}
			sort.Strings(documents[i].DependedBy)
		}
		if deps, ok := rdependedBy[documents[i].FullName]; ok {
			for dep := range deps {
				documents[i].RdependedBy = append(documents[i].RdependedBy, dep)
			}
			sort.Strings(documents[i].RdependedBy)
		}
	}

	manifest := SearchManifest{
		DocumentCount: len(documents),
		DataFiles:     []string{"docs-0.json"},
	}

	if outZip != "" {
		f, err := os.Create(outZip)
		if err != nil {
			return fmt.Errorf("creating search zip file: %w", err)
		}
		defer func() { _ = f.Close() }()

		z := zip.NewWriter(f)
		defer func() { _ = z.Close() }()

		// Create data dir
		docsWriter, err := z.Create("data/docs-0.json")
		if err != nil {
			return fmt.Errorf("creating docs-0.json in zip: %w", err)
		}
		encoder := json.NewEncoder(docsWriter)
		if err := encoder.Encode(documents); err != nil {
			return fmt.Errorf("encoding search docs: %w", err)
		}

		manifestWriter, err := z.Create("data/manifest.json")
		if err != nil {
			return fmt.Errorf("creating manifest.json in zip: %w", err)
		}
		mEncoder := json.NewEncoder(manifestWriter)
		if err := mEncoder.Encode(manifest); err != nil {
			return fmt.Errorf("encoding search manifest: %w", err)
		}
		return nil
	}

	if outDir != "" {
		dataDir := filepath.Join(outDir, "data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("creating search data directory: %w", err)
		}

		dataFile := "docs-0.json"
		dataFilePath := filepath.Join(dataDir, dataFile)

		f, err := os.Create(dataFilePath)
		if err != nil {
			return fmt.Errorf("creating docs data file: %w", err)
		}
		defer func() { _ = f.Close() }()

		encoder := json.NewEncoder(f)
		if err := encoder.Encode(documents); err != nil {
			return fmt.Errorf("encoding search docs: %w", err)
		}

		manifestPath := filepath.Join(dataDir, "manifest.json")
		mf, err := os.Create(manifestPath)
		if err != nil {
			return fmt.Errorf("creating search manifest: %w", err)
		}
		defer func() { _ = mf.Close() }()

		mEncoder := json.NewEncoder(mf)
		if err := mEncoder.Encode(manifest); err != nil {
			return fmt.Errorf("encoding search manifest: %w", err)
		}
	}

	return nil
}

func generateSearchIndex(outDir string, sites []*SiteData) error {
	searchDir := filepath.Join(outDir, "search")
	dataDir := filepath.Join(searchDir, "data")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating search data directory: %w", err)
	}

	if err := generateSearchData(searchDir, "", sites); err != nil {
		return fmt.Errorf("generating search data: %w", err)
	}

	jsFiles := []string{"search_parser.js", "search.js", "search_ui.js"}
	for _, jsFile := range jsFiles {
		content, err := templates.SiteFS.ReadFile("site/" + jsFile)
		if err != nil {
			return fmt.Errorf("reading template js %s: %w", jsFile, err)
		}
		destPath := filepath.Join(searchDir, jsFile)
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("writing search js file %s: %w", jsFile, err)
		}
	}

	tmpl, err := GetSiteTemplates()
	if err != nil {
		return fmt.Errorf("loading templates for search: %w", err)
	}

	if err := renderPage(filepath.Join(searchDir, "index.html"), tmpl, "search.html", map[string]interface{}{
		"Title":       "Search Overlays",
		"BaseURL":     "../",
		"Breadcrumbs": []g2.Breadcrumb{{Name: "Search"}},
		"Version":     version,
	}); err != nil {
		return fmt.Errorf("rendering search index page: %w", err)
	}

	return nil
}

func deduplicateStrings(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}
	seen := make(map[string]bool)
	var res []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			res = append(res, v)
		}
	}
	return res
}
