package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (cfg *MainArgConfig) cmdSite(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing subcommand for site (e.g., serve)")
	}
	subcmd := args[0]

	switch subcmd {
	case "serve":
		return cfg.cmdSiteServe(args[1:])
	default:
		return fmt.Errorf("unknown site subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) cmdSiteServe(args []string) error {
	fs := flag.NewFlagSet("site serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "Port to run the site server on")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	// Determine if location is a single overlay or we need to fall back to /var/db/repos
	var sites []*SiteData

	if isOverlayDir(location) {
		log.Printf("Parsing local overlay at %s", location)
		siteData, err := parseRepo(location, "Gentoo Packages")
		if err != nil {
			return fmt.Errorf("parsing repo %s: %w", location, err)
		}
		sites = append(sites, siteData)
	} else {
		dbReposPath := "/var/db/repos"
		log.Printf("Location %s is not an overlay, checking %s", location, dbReposPath)

		entries, err := os.ReadDir(dbReposPath)
		if err != nil {
			return fmt.Errorf("could not read %s: %w", dbReposPath, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(dbReposPath, entry.Name())
			if isOverlayDir(repoPath) {
				log.Printf("Parsing repository %s", entry.Name())
				siteData, err := parseRepo(repoPath, entry.Name())
				if err != nil {
					log.Printf("Warning: failed to parse repo %s: %v", entry.Name(), err)
					continue
				}
				sites = append(sites, siteData)
			}
		}

		if len(sites) == 0 {
			return fmt.Errorf("no valid repositories found in %s", dbReposPath)
		}
	}

	log.Printf("Pre-calculating site data for %d repositories", len(sites))
	handler, err := newSiteServer(sites)
	if err != nil {
		return fmt.Errorf("initializing site server: %w", err)
	}

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting live site server at http://localhost%s", addr)

	return http.ListenAndServe(addr, handler)
}

func isOverlayDir(dir string) bool {
	// A basic check to see if a directory looks like a Gentoo overlay.
	// We'll check for profiles/repo_name or just profiles directory.
	profilesDir := filepath.Join(dir, "profiles")
	info, err := os.Stat(profilesDir)
	return err == nil && info.IsDir()
}

type SiteServer struct {
	tmpl             *template.Template
	Title            string
	Sites            []*SiteData
	AggCategories    []*AggCategory
	AggPackages      []*AggPackage
	AggLicenses      []*AggLicense
	GlobalUpdates    []FeedItem

	// Mappings for faster lookup
	CatMap           map[string]*AggCategory
	PkgMap           map[string]*AggPackage
	LicMap           map[string]*AggLicense
	RepoMap          map[string]*SiteData
}

func newSiteServer(sites []*SiteData) (*SiteServer, error) {
	tmpl, err := template.ParseFS(siteTemplates, "sitegen_templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	server := &SiteServer{
		tmpl:    tmpl,
		Sites:   sites,
		CatMap:  make(map[string]*AggCategory),
		PkgMap:  make(map[string]*AggPackage),
		LicMap:  make(map[string]*AggLicense),
		RepoMap: make(map[string]*SiteData),
	}

	// Similar aggregation logic to generateSite
	aggCategories := make(map[string]*AggCategory)
	aggPackages := make(map[string]*AggPackage)
	aggLicenses := make(map[string]*AggLicense)

	for _, site := range sites {
		server.RepoMap[site.RepoName] = site
		for _, cat := range site.Categories {
			if _, ok := aggCategories[cat.Name]; !ok {
				aggCategories[cat.Name] = &AggCategory{Name: cat.Name, Packages: make(map[string]*AggPackage)}
			}
			for _, pkg := range cat.Packages {
				pkgKey := cat.Name + "/" + pkg.Name
				if _, ok := aggPackages[pkgKey]; !ok {
					aggPackages[pkgKey] = &AggPackage{Name: pkg.Name, Category: cat.Name, Repos: make(map[string]*SiteData)}
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

	for _, c := range aggCategories { server.AggCategories = append(server.AggCategories, c) }
	sort.Slice(server.AggCategories, func(i, j int) bool { return server.AggCategories[i].Name < server.AggCategories[j].Name })

	for _, p := range aggPackages { server.AggPackages = append(server.AggPackages, p) }
	sort.Slice(server.AggPackages, func(i, j int) bool {
		if server.AggPackages[i].Category == server.AggPackages[j].Category { return server.AggPackages[i].Name < server.AggPackages[j].Name }
		return server.AggPackages[i].Category < server.AggPackages[j].Category
	})

	for _, l := range aggLicenses { server.AggLicenses = append(server.AggLicenses, l) }
	sort.Slice(server.AggLicenses, func(i, j int) bool { return server.AggLicenses[i].Name < server.AggLicenses[j].Name })

	server.CatMap = aggCategories
	server.PkgMap = aggPackages
	server.LicMap = aggLicenses

	server.Title = "Gentoo Packages"
	if len(sites) == 1 {
		server.Title = sites[0].Title
	}

	var globalFeedItems []FeedItem
	for _, pkg := range server.AggPackages {
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
	server.GlobalUpdates = globalFeedItems

	return server, nil
}

func mapToList(m map[string]*SiteData) []*SiteData {
	var l []*SiteData
	for _, v := range m {
		l = append(l, v)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].RepoName < l[j].RepoName })
	return l
}

func (s *SiteServer) renderPageHTTP(w http.ResponseWriter, name string, data map[string]interface{}) {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data["Content"] = template.HTML(buf.String())
	if err := s.tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering layout: %v", err)
	}
}

func (s *SiteServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, "/index.html")
	parts := strings.Split(path, "/")
	if path == "" {
		parts = []string{}
	}

	// 1. Root Dashboard
	if len(parts) == 0 {
		s.renderPageHTTP(w, "dashboard.html", map[string]interface{}{
			"Title":      s.Title,
			"BaseURL":    "",
			"Repos":      s.Sites,
			"Categories": s.AggCategories,
			"Packages":   s.AggPackages,
			"Licenses":   s.AggLicenses,
			"Updates":    s.GlobalUpdates,
			"Version":    version,
		})
		return
	}

	// Helper for base URL
	baseURL := ""
	for i := 0; i < len(parts); i++ {
		baseURL += "../"
	}

	// Route based on first part
	switch parts[0] {
	case "overlays":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "overlays.html", map[string]interface{}{
				"Title":       "Overlays",
				"BaseURL":     baseURL,
				"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Overlays"}},
				"Repos":       s.Sites,
				"Version":     version,
			})
			return
		}

	case "categories":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "categories.html", map[string]interface{}{
				"Title":       "Categories",
				"BaseURL":     baseURL,
				"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Categories"}},
				"Categories":  s.AggCategories,
				"Version":     version,
			})
			return
		} else if len(parts) == 2 {
			catName := parts[1]
			cat, ok := s.CatMap[catName]
			if !ok {
				http.NotFound(w, r)
				return
			}

			var catPkgs []*AggPackage
			for _, p := range cat.Packages { catPkgs = append(catPkgs, p) }
			sort.Slice(catPkgs, func(i, j int) bool { return catPkgs[i].Name < catPkgs[j].Name })

			type TmplPkg struct {
				Name string
				ReposList []*SiteData
			}
			var tmplPkgs []TmplPkg
			for _, p := range catPkgs {
				tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: mapToList(p.Repos)})
			}

			s.renderPageHTTP(w, "category.html", map[string]interface{}{
				"Title":       "Category: " + cat.Name,
				"BaseURL":     baseURL,
				"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
				"Category":    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
				"Version":     version,
			})
			return
		}

	case "packages":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "packages.html", map[string]interface{}{
				"Title":       "Packages",
				"BaseURL":     baseURL,
				"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Packages"}},
				"Packages":    s.AggPackages,
				"Version":     version,
			})
			return
		} else if len(parts) == 3 {
			catName := parts[1]
			pkgName := parts[2]
			pkgKey := catName + "/" + pkgName

			pkg, ok := s.PkgMap[pkgKey]
			if !ok {
				http.NotFound(w, r)
				return
			}

			reposList := mapToList(pkg.Repos)

			if len(reposList) == 1 {
				targetURL := fmt.Sprintf("%srepos/%s/categories/%s/packages/%s/", baseURL, reposList[0].RepoName, pkg.Category, pkg.Name)
				http.Redirect(w, r, targetURL, http.StatusFound)
				return
			} else {
				s.renderPageHTTP(w, "package_picker.html", map[string]interface{}{
					"Title":       "Package: " + pkg.Category + "/" + pkg.Name,
					"BaseURL":     baseURL,
					"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Packages", URL: "../../"}, {Name: pkg.Category}, {Name: pkg.Name}},
					"Package":     map[string]interface{}{"Category": pkg.Category, "Name": pkg.Name, "ReposList": reposList},
					"Version":     version,
				})
				return
			}
		}

	case "licenses":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "licenses.html", map[string]interface{}{
				"Title":       "Licenses",
				"BaseURL":     baseURL,
				"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Licenses"}},
				"Licenses":    s.AggLicenses,
				"Version":     version,
			})
			return
		} else if len(parts) == 2 {
			licName := parts[1]
			lic, ok := s.LicMap[licName]
			if !ok {
				http.NotFound(w, r)
				return
			}

			type TmplPkg struct {
				Name string
				Category string
				ReposList []*SiteData
			}
			var tmplPkgs []TmplPkg
			for _, p := range lic.Packages {
				tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, Category: p.Category, ReposList: mapToList(p.Repos)})
			}

			s.renderPageHTTP(w, "license.html", map[string]interface{}{
				"Title":       "License: " + lic.Name,
				"BaseURL":     baseURL,
				"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Licenses", URL: "../"}, {Name: lic.Name}},
				"License":     map[string]interface{}{"Name": lic.Name, "Packages": tmplPkgs, "Text": lic.Text},
				"Version":     version,
			})
			return
		}

	case "repos":
		if len(parts) >= 2 {
			repoName := parts[1]
			site, ok := s.RepoMap[repoName]
			if !ok {
				http.NotFound(w, r)
				return
			}

			if len(parts) == 2 {
				var repoFeedItems []FeedItem
				pkgCount := 0
				for _, cat := range site.Categories {
					pkgCount += len(cat.Packages)
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

				s.renderPageHTTP(w, "repo_index.html", map[string]interface{}{
					"Title":       site.RepoName,
					"BaseURL":     baseURL,
					"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Overlays", URL: baseURL + "overlays/"}, {Name: site.RepoName}},
					"Repo":        site,
					"PackageCount": pkgCount,
					"Updates":     repoFeedItems,
					"Version":     version,
				})
				return
			}

			if len(parts) >= 3 {
				switch parts[2] {
				case "categories":
					if len(parts) == 3 {
						s.renderPageHTTP(w, "categories.html", map[string]interface{}{
							"Title":       site.RepoName + " - Categories",
							"BaseURL":     baseURL,
							"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Categories"}},
							"Categories":  site.Categories,
							"Version":     version,
						})
						return
					} else if len(parts) == 4 {
						catName := parts[3]
						var catData *CategoryData
						for _, c := range site.Categories {
							if c.Name == catName {
								catData = &c
								break
							}
						}
						if catData == nil {
							http.NotFound(w, r)
							return
						}

						type TmplPkg struct {
							Name string
							ReposList []*SiteData
						}
						var tmplPkgs []TmplPkg
						for _, p := range catData.Packages {
							tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: []*SiteData{site}})
						}

						s.renderPageHTTP(w, "category.html", map[string]interface{}{
							"Title":       "Category: " + catData.Name,
							"BaseURL":     baseURL,
							"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: catData.Name}},
							"Category":    map[string]interface{}{"Name": catData.Name, "Packages": tmplPkgs},
							"Version":     version,
						})
						return
					} else if len(parts) == 6 && parts[4] == "packages" {
						catName := parts[3]
						pkgName := parts[5]

						var pkgData *PackageData
						for _, c := range site.Categories {
							if c.Name == catName {
								for _, p := range c.Packages {
									if p.Name == pkgName {
										pkgData = &p
										break
									}
								}
							}
						}
						if pkgData == nil {
							http.NotFound(w, r)
							return
						}

						s.renderPageHTTP(w, "repo_package.html", map[string]interface{}{
							"Title":       fmt.Sprintf("%s - %s/%s", site.RepoName, pkgData.Category, pkgData.Name),
							"BaseURL":     baseURL,
							"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../../../../"}, {Name: "Categories", URL: "../../../"}, {Name: pkgData.Category}, {Name: pkgData.Name}},
							"Repo":        site,
							"Package":     *pkgData,
							"Version":     version,
						})
						return
					}

				case "packages":
					if len(parts) == 3 {
						var repoPkgs []PackageData
						for _, c := range site.Categories { repoPkgs = append(repoPkgs, c.Packages...) }
						sort.Slice(repoPkgs, func(i, j int) bool {
							if repoPkgs[i].Category == repoPkgs[j].Category { return repoPkgs[i].Name < repoPkgs[j].Name }
							return repoPkgs[i].Category < repoPkgs[j].Category
						})

						s.renderPageHTTP(w, "repo_packages.html", map[string]interface{}{
							"Title":       site.RepoName + " - Packages",
							"BaseURL":     baseURL,
							"Breadcrumbs": []Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Packages"}},
							"Packages":    repoPkgs,
							"Repo":        site,
							"Version":     version,
						})
						return
					}
				}
			}
		}
	}

	http.NotFound(w, r)
}
