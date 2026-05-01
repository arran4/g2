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

var pkgRegex = regexp.MustCompile(`([a-zA-Z0-9_][a-zA-Z0-9_\-\+]*\/[a-zA-Z0-9_][a-zA-Z0-9_\-\+]+)`)

func generateSearchData(outDir, outZip string, sites []*g2.SiteData, maxChunkSizeOverride ...int) error {
	var documents []SearchDocument
	docID := 0

	dependedBy := make(map[string]map[string]bool)
	rdependedBy := make(map[string]map[string]bool)

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
						sL := licenseStr
						for len(sL) > 0 {
							for len(sL) > 0 && sL[0] <= ' ' {
								sL = sL[1:]
							}
							if sL == "" {
								break
							}
							end := 0
							for end < len(sL) && sL[end] > ' ' {
								end++
							}
							l := sL[:end]
							if l != "||" && l != "(" && l != ")" && !(len(l) > 0 && l[0] == '?') {
								licenses = append(licenses, l)
								if site.LicenseMapping != nil {
									if aliases, ok := site.LicenseMapping[l]; ok {
										licenses = append(licenses, aliases...)
									}
								}
							}
							sL = sL[end:]
						}

						keywordStr := ver.Ebuild.Vars["KEYWORDS"]
						sK := keywordStr
						for len(sK) > 0 {
							for len(sK) > 0 && sK[0] <= ' ' {
								sK = sK[1:]
							}
							if sK == "" {
								break
							}
							end := 0
							for end < len(sK) && sK[end] > ' ' {
								end++
							}
							kw := sK[:end]
							keywords = append(keywords, kw)
							arch := kw
							if len(arch) > 0 && arch[0] == '~' {
								arch = arch[1:]
							}
							if len(arch) > 0 && arch[0] == '-' {
								arch = arch[1:]
							}
							if len(arch) > 0 {
								arches = append(arches, arch)
							}
							sK = sK[end:]
						}

						iuseStr := ver.Ebuild.Vars["IUSE"]
						sI := iuseStr
						for len(sI) > 0 {
							for len(sI) > 0 && sI[0] <= ' ' {
								sI = sI[1:]
							}
							if sI == "" {
								break
							}
							end := 0
							for end < len(sI) && sI[end] > ' ' {
								end++
							}
							u := sI[:end]
							if len(u) > 0 && (u[0] == '+' || u[0] == '-') {
								u = u[1:]
							}
							uses = append(uses, u)
							sI = sI[end:]
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

					for i := range uses {
						uses[i] = strings.ToLower(uses[i])
					}
					for i := range urls {
						urls[i] = strings.ToLower(urls[i])
					}
					for i := range useDescriptions {
						useDescriptions[i] = strings.ToLower(useDescriptions[i])
					}

					searchText := strings.ToLower(fmt.Sprintf("%s %s %s %s %s", fullName, desc, strings.Join(uses, " "), strings.Join(urls, " "), strings.Join(useDescriptions, " ")))

					for i := range licenses {
						licenses[i] = strings.ToLower(licenses[i])
					}
					for i := range keywords {
						keywords[i] = strings.ToLower(keywords[i])
					}
					for i := range arches {
						arches[i] = strings.ToLower(arches[i])
					}
					for i := range depends {
						depends[i] = strings.ToLower(depends[i])
					}
					for i := range rdepends {
						rdepends[i] = strings.ToLower(rdepends[i])
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

					catNameLower := strings.ToLower(cat.Name)
					pkgNameLower := strings.ToLower(pkg.Name)
					fullNameLower := strings.ToLower(fullName)
					overlayNameLower := strings.ToLower(overlayName)
					descLower := strings.ToLower(desc)

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
						Overlay:         overlayNameLower,
						Category:        catNameLower,
						Package:         pkgNameLower,
						FullName:        fullNameLower,
						Version:         verStr,
						Description:     descLower,
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

	// Build inverted index mapping token -> []int (doc IDs)
	invertedIndex := make(map[string][]int)
	for _, doc := range documents {
		tokens := tokenizeDocument(doc)
		for _, t := range tokens {
			if len(invertedIndex[t]) > 0 && invertedIndex[t][len(invertedIndex[t])-1] == doc.ID {
				continue
			}
			invertedIndex[t] = append(invertedIndex[t], doc.ID)
		}
	}

	// Partition the inverted index
	// bucket -> map[token][]int
	partitionedIndex := make(map[string]map[string][]int)
	for token, docIDs := range invertedIndex {
		bucket := getBucket(token)
		if partitionedIndex[bucket] == nil {
			partitionedIndex[bucket] = make(map[string][]int)
		}
		partitionedIndex[bucket][token] = docIDs
	}

	// Output individual docs directly.
	// Ensure dataFiles is initialized so it doesn't serialize to null for old clients.
	dataFiles := make([]string, 0)

	if outZip != "" {
		f, err := os.Create(outZip)
		if err != nil {
			return fmt.Errorf("creating search zip file: %w", err)
		}
		defer func() { _ = f.Close() }()

		z := zip.NewWriter(f)
		defer func() { _ = z.Close() }()

		// Write individual doc files
		for _, doc := range documents {
			docsWriter, err := z.Create(fmt.Sprintf("data/docs/%d.json", doc.ID))
			if err != nil {
				return fmt.Errorf("creating %d.json in zip: %w", doc.ID, err)
			}
			encoder := json.NewEncoder(docsWriter)
			if err := encoder.Encode(doc); err != nil {
				return fmt.Errorf("encoding search doc %d: %w", doc.ID, err)
			}
		}

		// Write partitioned index files
		for bucket, tokenMap := range partitionedIndex {
			// Bucket string, e.g. "py" -> index/p/y/py.json
			if len(bucket) < 1 {
				continue
			}
			p1 := string(bucket[0])
			p2 := ""
			if len(bucket) > 1 {
				p2 = string(bucket[1])
			} else {
				p2 = "_"
			}
			indexPath := fmt.Sprintf("data/index/%s/%s/%s.json", p1, p2, bucket)
			idxWriter, err := z.Create(indexPath)
			if err != nil {
				return fmt.Errorf("creating index file %s in zip: %w", indexPath, err)
			}
			encoder := json.NewEncoder(idxWriter)
			if err := encoder.Encode(tokenMap); err != nil {
				return fmt.Errorf("encoding index %s: %w", bucket, err)
			}
		}

		manifest := SearchManifest{
			DocumentCount: len(documents),
			DataFiles:     dataFiles,
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
		docsDir := filepath.Join(dataDir, "docs")
		if err := os.MkdirAll(docsDir, 0755); err != nil {
			return fmt.Errorf("creating search docs directory: %w", err)
		}

		for _, doc := range documents {
			dataFile := fmt.Sprintf("%d.json", doc.ID)
			dataFilePath := filepath.Join(docsDir, dataFile)

			f, err := os.Create(dataFilePath)
			if err != nil {
				return fmt.Errorf("creating doc file %d: %w", doc.ID, err)
			}
			encoder := json.NewEncoder(f)
			err = encoder.Encode(doc)
			_ = f.Close()
			if err != nil {
				return fmt.Errorf("encoding search doc %d: %w", doc.ID, err)
			}
		}

		// Write partitioned index files
		for bucket, tokenMap := range partitionedIndex {
			if len(bucket) < 1 {
				continue
			}
			p1 := string(bucket[0])
			p2 := ""
			if len(bucket) > 1 {
				p2 = string(bucket[1])
			} else {
				p2 = "_"
			}
			indexPath := filepath.Join(dataDir, "index", p1, p2)
			if err := os.MkdirAll(indexPath, 0755); err != nil {
				return fmt.Errorf("creating search index directory: %w", err)
			}
			indexFile := filepath.Join(indexPath, fmt.Sprintf("%s.json", bucket))
			f, err := os.Create(indexFile)
			if err != nil {
				return fmt.Errorf("creating index file %s: %w", indexFile, err)
			}
			encoder := json.NewEncoder(f)
			err = encoder.Encode(tokenMap)
			_ = f.Close()
			if err != nil {
				return fmt.Errorf("encoding search index %s: %w", bucket, err)
			}
		}

		manifest := SearchManifest{
			DocumentCount: len(documents),
			DataFiles:     dataFiles,
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

func generateSearchIndex(outDir string, sites []*g2.SiteData) error {
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

func getBucket(token string) string {
	val := token
	if idx := strings.Index(token, ":"); idx != -1 {
		val = token[idx+1:]
	}
	t := strings.ToLower(val)
	var cleaned []rune
	for _, r := range t {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cleaned = append(cleaned, r)
		}
	}
	if len(cleaned) == 0 {
		return "_"
	}
	if len(cleaned) == 1 {
		return string(cleaned[0]) + "_"
	}
	return string(cleaned[0:2])
}

func tokenizeDocument(doc SearchDocument) []string {
	seen := make(map[string]bool)
	var tokens []string

	addToken := func(t string) {
		t = strings.ToLower(t)
		if t == "" {
			return
		}
		if !seen[t] {
			seen[t] = true
			tokens = append(tokens, t)
		}
	}

	// Text fields
	words := strings.Fields(doc.SearchText)
	for _, w := range words {
		// Just basic stripping of common punctuation from ends could be useful,
		// but since earlier code didn't do much stripping, we just add the word.
		addToken(w)
	}
	// Fullname split by /
	parts := strings.Split(doc.FullName, "/")
	for _, p := range parts {
		addToken(p)
	}

	// Specific fields
	if doc.Overlay != "" {
		addToken("overlay:" + doc.Overlay)
	}
	if doc.Category != "" {
		addToken("category:" + doc.Category)
	}
	for _, url := range doc.Urls {
		addToken("url:" + url)
	}
	for _, a := range doc.Arches {
		addToken("arch:" + a)
	}
	for _, k := range doc.Keywords {
		addToken("keyword:" + k)
	}
	if doc.Mask != "" {
		addToken("mask:" + doc.Mask)
	}
	for _, l := range doc.Licenses {
		addToken("license:" + l)
	}
	for _, d := range doc.Depends {
		addToken("depends:" + d)
	}
	for _, d := range doc.Rdepends {
		addToken("rdepends:" + d)
	}
	for _, d := range doc.Bdepends {
		addToken("bdepends:" + d)
	}
	for _, d := range doc.Pdepends {
		addToken("pdepends:" + d)
	}
	for _, d := range doc.DependedBy {
		addToken("depended:" + d)
	}
	for _, d := range doc.RdependedBy {
		addToken("rdepended:" + d)
	}
	for _, m := range doc.ManifestFiles {
		addToken("manifestfile:" + m)
	}
	if doc.EAPI != "" {
		addToken("eapi:" + doc.EAPI)
	}
	if doc.Slot != "" {
		addToken("slot:" + doc.Slot)
	}
	for _, i := range doc.Inherits {
		addToken("inherit:" + i)
	}
	for _, u := range doc.Uses {
		addToken("use:" + u)
	}
	if doc.Version != "" {
		addToken("version:" + doc.Version)
	}

	return tokens
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
