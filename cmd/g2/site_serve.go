package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/arran4/g2"
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
	concurrency := fs.Int("concurrency", 4, "Maximum number of concurrent repository fetches/parses")
	smartMode := fs.Bool("smart-mode", true, "Use smart mode for parsing repositories in memory without disk access")
	reposConfOpt := fs.String("repos-conf", "", "Path to repos.conf file or directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	// Determine if location is a single overlay or we need to fall back to repos.conf / /var/db/repos
	var sites []*g2.SiteData

limit := *concurrency
	if limit <= 0 {
		limit = 10
	}

	memManager := NewMemoryManager()
	freeMem, err := getFreeMemory()
	var defaultAlloc uint64 = 0
	if err == nil && limit > 0 {
		defaultAlloc = freeMem / 2 / uint64(limit)
	}
	if isOverlayDir(location) {
		log.Printf("Parsing local overlay at %s", location)
		siteData, err := parseRepo(os.DirFS(location), ".", "Gentoo Packages", false, nil)
		if err != nil {
			return fmt.Errorf("parsing repo %s: %w", location, err)
		}
		sites = append(sites, siteData)
	} else {
		var repoPaths []string
		var repoNames []string

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
					if loc != "" {
						repoPaths = append(repoPaths, loc)
						repoNames = append(repoNames, s.Name)
					}
				}
			}
			log.Printf("Location %s is not an overlay, checking %d repos from %s", location, len(repoPaths), *reposConfOpt)
		} else {
			dbReposPath := "/var/db/repos"
			log.Printf("Location %s is not an overlay, checking %s", location, dbReposPath)

			entries, err := os.ReadDir(dbReposPath)
			if err == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					repoPaths = append(repoPaths, filepath.Join(dbReposPath, entry.Name()))
					repoNames = append(repoNames, entry.Name())
				}
			} else {
				return fmt.Errorf("could not read %s: %w", dbReposPath, err)
			}
		}

		var sitesMu sync.Mutex
		g, _ := errgroup.WithContext(context.Background())
		if *smartMode {
			log.Printf("Smart mode enabled for site serve, using in-memory parsing")
		}
		if *concurrency > 0 {
			g.SetLimit(*concurrency)
			log.Printf("Starting concurrent repository processing with %d concurrency limit", *concurrency)
		} else {
			log.Printf("Starting concurrent repository processing with unbounded concurrency")
		}

		for i, repoPath := range repoPaths {
			repoPath := repoPath
			repoName := repoNames[i]

			if isOverlayDir(repoPath) {
				g.Go(func() error {
					log.Printf("[START] Parsing repository %s", repoName)
					var siteData *g2.SiteData
					var err error
					func() {
						memManager.Acquire(defaultAlloc)
						defer memManager.Release(defaultAlloc)
						siteData, err = parseRepo(os.DirFS(repoPath), ".", repoName, false, nil)
					}()
					if err != nil {
						log.Printf("Warning: failed to parse repo %s: %v", repoName, err)
						return nil // Don't fail entire group
					}

					freeSpace, err := getFreeSpace(repoPath)
					appFreeSpace, appErr := getFreeSpace(".")
					tmpFreeSpace, tmpErr := getFreeSpace(os.TempDir())
					freeMem, memErr := getFreeMemory()
					if err == nil && appErr == nil && tmpErr == nil && memErr == nil {
						log.Printf("[DONE] Finished parsing repository %s. Free space (Repo/App/Tmp): %.2f/%.2f/%.2f MB. Free memory: %.2f MB", repoName, float64(freeSpace)/(1024*1024), float64(appFreeSpace)/(1024*1024), float64(tmpFreeSpace)/(1024*1024), float64(freeMem)/(1024*1024))
					} else if err == nil && appErr == nil && memErr == nil {
						log.Printf("[DONE] Finished parsing repository %s. Free space (Repo/App): %.2f/%.2f MB. Free memory: %.2f MB", repoName, float64(freeSpace)/(1024*1024), float64(appFreeSpace)/(1024*1024), float64(freeMem)/(1024*1024))
					} else if err == nil && memErr == nil {
						log.Printf("[DONE] Finished parsing repository %s. Free space: %.2f MB. Free memory: %.2f MB", repoName, float64(freeSpace)/(1024*1024), float64(freeMem)/(1024*1024))
					} else if err == nil && appErr == nil && tmpErr == nil {
						log.Printf("[DONE] Finished parsing repository %s. Free space (Repo/App/Tmp): %.2f/%.2f/%.2f MB", repoName, float64(freeSpace)/(1024*1024), float64(appFreeSpace)/(1024*1024), float64(tmpFreeSpace)/(1024*1024))
					} else if err == nil && appErr == nil {
						log.Printf("[DONE] Finished parsing repository %s. Free space (Repo/App): %.2f/%.2f MB", repoName, float64(freeSpace)/(1024*1024), float64(appFreeSpace)/(1024*1024))
					} else if err == nil {
						log.Printf("[DONE] Finished parsing repository %s. Free space: %.2f MB", repoName, float64(freeSpace)/(1024*1024))
					} else if memErr == nil {
						log.Printf("[DONE] Finished parsing repository %s. Free memory: %.2f MB", repoName, float64(freeMem)/(1024*1024))
					} else {
						log.Printf("[DONE] Finished parsing repository %s", repoName)
					}

					if *concurrency == 1 {
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

						var memStats runtime.MemStats
						runtime.ReadMemStats(&memStats)
						log.Printf("[STATS] Heap Objects: %d, Alloc: %.2f MB, Total Alloc: %.2f MB, Sys: %.2f MB, NumGC: %d",
							memStats.HeapObjects,
							float64(memStats.Alloc)/(1024*1024),
							float64(memStats.TotalAlloc)/(1024*1024),
							float64(memStats.Sys)/(1024*1024),
							memStats.NumGC,
						)
					}

					sitesMu.Lock()
					sites = append(sites, siteData)
					sitesMu.Unlock()
					return nil
				})
			}
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("concurrent repository processing failed: %w", err)
		}

		sort.Slice(sites, func(i, j int) bool {
			return sites[i].Title < sites[j].Title
		})

		if len(sites) == 0 {
			return fmt.Errorf("no valid repositories found from given configuration or default path")
		}
	}

	log.Printf("Pre-calculating site data (v%s) for %d repositories", version, len(sites))
	genInfo := GenerationInfo{Args: cfg.Args}
	handler, err := newSiteServer(sites, genInfo)
	if err != nil {
		return fmt.Errorf("initializing site server: %w", err)
	}

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting live site server (v%s) at http://localhost%s", version, addr)

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
	GenInfo       GenerationInfo
	tmpl          *template.Template
	Title         string
	Sites         []*g2.SiteData
	AggCategories []*AggCategory
	AggPackages   []*AggPackage
	AggLicenses   []*AggLicense
	AggProjects   []*AggProject

	// Mappings for faster lookup
	CatMap      map[string]*AggCategory
	PkgMap      map[string]*AggPackage
	LicMap      map[string]*AggLicense
	UseMap      map[string]*AggUseFlag
	AggUseFlags []*AggUseFlag
	ProjMap     map[string]*AggProject
	RepoMap     map[string]*g2.SiteData
}

type ParsedIUSEFlag struct {
	Name         string
	Conditional  bool
	ConditionStr string
}

func parseIUSEFlagsFunc(iuse string) []ParsedIUSEFlag {
	var flags []ParsedIUSEFlag
	parts := strings.Fields(iuse)
	for _, part := range parts {
		if part == "" || part == "." || part == "/" || part == ".." {
			if part != "" {
				log.Printf("Warning: filtered out invalid use flag: %q", part)
			}
			continue
		}
		name := strings.TrimPrefix(part, "+")
		name = strings.TrimPrefix(name, "-")
		cond := ""
		if strings.HasPrefix(part, "+") {
			cond = "Default: Enabled (+)"
		} else if strings.HasPrefix(part, "-") {
			cond = "Default: Disabled (-)"
		}
		flags = append(flags, ParsedIUSEFlag{
			Name:         name,
			Conditional:  cond != "",
			ConditionStr: cond,
		})
	}
	return flags
}

func newSiteServer(sites []*g2.SiteData, genInfo GenerationInfo) (*SiteServer, error) {
	for _, site := range sites {
		populatePkgUseFlags(site)
	}

	tmpl, err := GetSiteTemplates()
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	server := &SiteServer{
		tmpl:    tmpl,
		Sites:   sites,
		CatMap:  make(map[string]*AggCategory),
		PkgMap:  make(map[string]*AggPackage),
		LicMap:  make(map[string]*AggLicense),
		ProjMap: make(map[string]*AggProject),
		RepoMap: make(map[string]*g2.SiteData),
		GenInfo: genInfo,
	}

	// Similar aggregation logic to generateSite
	aggCategories := make(map[string]*AggCategory)
	aggPackages := make(map[string]*AggPackage)
	aggLicenses := make(map[string]*AggLicense)
	aggProjects := make(map[string]*AggProject)

	catPkgMap := make(map[string]map[string]*AggPackage)

	for _, site := range sites {
		if site.Projects != nil {
			for i := range site.Projects.Projects {
				proj := &site.Projects.Projects[i]
				if _, ok := aggProjects[proj.Email]; !ok {
					aggProjects[proj.Email] = &AggProject{Project: proj}
				}
			}
		}
		server.RepoMap[site.RepoName] = site
		for _, cat := range site.Categories {
			if _, ok := aggCategories[cat.Name]; !ok {
				aggCategories[cat.Name] = &AggCategory{Name: cat.Name}
				catPkgMap[cat.Name] = make(map[string]*AggPackage)
			}
			for _, pkg := range cat.Packages {
				pkgKey := cat.Name + "/" + pkg.Name
				if _, ok := aggPackages[pkgKey]; !ok {
					aggPackages[pkgKey] = &AggPackage{Name: pkg.Name, Category: cat.Name, Repos: make(map[string]*g2.SiteData)}
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
						lic := ver.Ebuild.Vars["LICENSE"]
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

	for catName, pkgs := range catPkgMap {
		var sortedPkgs []*AggPackage
		for _, p := range pkgs {
			sortedPkgs = append(sortedPkgs, p)
		}
		sort.Slice(sortedPkgs, func(i, j int) bool { return sortedPkgs[i].Name < sortedPkgs[j].Name })
		aggCategories[catName].Packages = sortedPkgs
	}

	for _, c := range aggCategories {
		server.AggCategories = append(server.AggCategories, c)
	}
	sort.Slice(server.AggCategories, func(i, j int) bool { return server.AggCategories[i].Name < server.AggCategories[j].Name })

	for _, p := range aggPackages {
		server.AggPackages = append(server.AggPackages, p)
	}
	sort.Slice(server.AggPackages, func(i, j int) bool {
		if server.AggPackages[i].Category == server.AggPackages[j].Category {
			return server.AggPackages[i].Name < server.AggPackages[j].Name
		}
		return server.AggPackages[i].Category < server.AggPackages[j].Category
	})

	sortedUseFlags, aggUseFlags := AggregateUseFlags(sites, aggPackages)

	for _, l := range aggLicenses {
		sort.Strings(l.Aliases)
		server.AggLicenses = append(server.AggLicenses, l)
	}
	sort.Slice(server.AggLicenses, func(i, j int) bool { return server.AggLicenses[i].Name < server.AggLicenses[j].Name })

	for _, p := range aggProjects {
		server.AggProjects = append(server.AggProjects, p)
	}
	sort.Slice(server.AggProjects, func(i, j int) bool { return server.AggProjects[i].Project.Name < server.AggProjects[j].Project.Name })

	server.CatMap = aggCategories
	server.PkgMap = aggPackages
	server.LicMap = aggLicenses
	server.ProjMap = aggProjects
	server.UseMap = aggUseFlags
	server.AggUseFlags = sortedUseFlags

	server.Title = "Gentoo Packages"
	if len(sites) == 1 {
		server.Title = sites[0].Title
	}

	return server, nil
}

func (s *SiteServer) renderPageHTTP(w http.ResponseWriter, name string, data map[string]interface{}) {
	log.Printf("Serving page using template %s", name)
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "layout_header.html", data); err != nil {
		errWrapped := fmt.Errorf("executing header template for %s: %w", name, err)
		log.Printf("Error: %v", errWrapped)
		http.Error(w, errWrapped.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		errWrapped := fmt.Errorf("executing template %s: %w", name, err)
		log.Printf("Error: %v", errWrapped)
		http.Error(w, errWrapped.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.tmpl.ExecuteTemplate(&buf, "layout_footer.html", data); err != nil {
		errWrapped := fmt.Errorf("executing footer template for %s: %w", name, err)
		log.Printf("Error: %v", errWrapped)
		http.Error(w, errWrapped.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("Error writing response for %s: %v", name, err)
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
			"GlobalCategories": s.AggCategories,
			"GlobalPackages":   s.AggPackages,
			"Licenses":   s.AggLicenses,
			"UseFlags":   s.AggUseFlags,
			"Projects":   s.AggProjects,
			"Profiles":   []interface{}{},
			"Version":    version,
			"GenInfo":    s.GenInfo,
		})
		return
	}

	// Helper for base URL
	baseURL := strings.Repeat("../", len(parts))

	// Route based on first part
	switch parts[0] {
	case "overlays":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "overlays.html", map[string]interface{}{
				"Title":       "Overlays",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Overlays"}},
				"Repos":       s.Sites,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		}

	case "categories":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "categories.html", map[string]interface{}{
				"Title":       "Categories",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Categories"}},
				"GlobalCategories":  s.AggCategories,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		} else if len(parts) == 2 {
			catName := parts[1]
			cat, ok := s.CatMap[catName]
			if !ok {
				http.NotFound(w, r)
				return
			}

			type TmplPkg struct {
				Name                  string
				ReposList             []*g2.SiteData
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
				var allVersions []g2.VersionData
				for _, r := range p.Repos {
					for _, c := range r.Categories {
						if c.Name == catName {
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

			s.renderPageHTTP(w, "category.html", map[string]interface{}{
				"Title":       "Category: " + cat.Name,
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
				"Category":    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		}

	case "packages":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "packages.html", map[string]interface{}{
				"Title":       "Packages",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Packages"}},
				"Packages":    s.AggPackages,
				"Version":     version,
				"GenInfo":     s.GenInfo,
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
					"Title":         "Package: " + pkg.Category + "/" + pkg.Name,
					"BaseURL":       baseURL,
					"Breadcrumbs":   []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Packages", URL: "../../"}, {Name: pkg.Category, URL: "../../../categories/" + pkg.Category + "/"}, {Name: pkg.Name}},
					"GlobalPackage": pkg,
					"Version":       version,
					"GenInfo":       s.GenInfo,
				})
				return
			}
		}

	case "uses":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "uses.html", map[string]interface{}{
				"Title":       "USE Flags",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "USE Flags"}},
				"UseFlags":    s.AggUseFlags,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		} else if len(parts) == 2 {
			flagName := parts[1]
			flag, ok := s.UseMap[flagName]
			if !ok {
				http.NotFound(w, r)
				return
			}
			s.renderPageHTTP(w, "use.html", map[string]interface{}{
				"Title":       "USE Flag: " + flag.Name,
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "USE Flags", URL: "../"}, {Name: flag.Name}},
				"UseFlag":     flag,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		}

	case "licenses":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "licenses.html", map[string]interface{}{
				"Title":       "Licenses",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Licenses"}},
				"Licenses":    s.AggLicenses,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		} else if len(parts) == 2 {
			licNameSlug := parts[1]
			var lic *AggLicense
			for _, l := range s.AggLicenses {
				if sanitizeFilename(l.Name) == licNameSlug {
					lic = l
					break
				}
			}
			if lic == nil {
				http.NotFound(w, r)
				return
			}

			s.renderPageHTTP(w, "license.html", map[string]interface{}{
				"Title":       "License: " + lic.Name,
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Licenses", URL: "../"}, {Name: lic.Name}},
				"License":     lic,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		}

	case "projects":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "projects.html", map[string]interface{}{
				"Title":       "Projects",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Projects"}},
				"Projects":    s.AggProjects,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		} else if len(parts) == 2 {
			projEmail := parts[1]
			proj, ok := s.ProjMap[projEmail]
			if !ok {
				http.NotFound(w, r)
				return
			}

			type TmplPkg struct {
				Name      string
				Category  string
				ReposList []*g2.SiteData
			}
			var tmplPkgs []TmplPkg
			for _, p := range proj.Packages {
				tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, Category: p.Category, ReposList: mapToList(p.Repos)})
			}

			s.renderPageHTTP(w, "project.html", map[string]interface{}{
				"Title":       "Project: " + proj.Project.Name,
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Projects", URL: "../"}, {Name: proj.Project.Name}},
				"Project":     proj,
				"Packages":    tmplPkgs,
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		}

	case "stats":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "stats.html", map[string]interface{}{
				"Title":       "Generation Statistics",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Statistics"}},
				"Version":     version,
				"GenInfo":     s.GenInfo,
			})
			return
		}

	case "help":
		if len(parts) == 1 {
			s.renderPageHTTP(w, "help.html", map[string]interface{}{
				"Title":       "Help & Legend",
				"BaseURL":     baseURL,
				"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Help"}},
				"Version":     version,
				"GenInfo":     s.GenInfo,
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
				pkgCount := 0
				for _, cat := range site.Categories {
					pkgCount += len(cat.Packages)
				}

				s.renderPageHTTP(w, "repo_index.html", map[string]interface{}{
					"Title":                 site.RepoName,
					"BaseURL":               baseURL,
					"Breadcrumbs":           []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: "Overlays", URL: baseURL + "overlays/"}, {Name: site.RepoName}},
					"Repo":                  site,
					"PackageCount":          site.PackageCount,
					"Version":               version,
					"GenInfo":               s.GenInfo,
					"GlobalCategoriesCount": len(s.AggCategories),
					"GlobalPackagesCount":   len(s.AggPackages),
					"GlobalLicensesCount":   len(s.AggLicenses),
					"GlobalProfilesCount":   0,
				})
				return
			}

			if len(parts) >= 3 {
				switch parts[2] {
				case "stats":
					if len(parts) == 3 {
						s.renderPageHTTP(w, "repo_stats.html", map[string]interface{}{
							"Title":       site.RepoName + " - Statistics",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Statistics"}},
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					}
				case "masked":
					if len(parts) == 3 {
						s.renderPageHTTP(w, "repo_masked.html", map[string]interface{}{
							"Title":       site.RepoName + " - Masked",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Masked Packages"}},
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					}
				case "deprecated":
					if len(parts) == 3 {
						s.renderPageHTTP(w, "repo_deprecated.html", map[string]interface{}{
							"Title":       site.RepoName + " - Deprecated",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Deprecated Packages"}},
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					}
				case "info_pkgs":
					if len(parts) == 3 {
						s.renderPageHTTP(w, "repo_info_pkgs.html", map[string]interface{}{
							"Title":       site.RepoName + " - Info Packages",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Info Packages"}},
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					}
				case "categories":
					if len(parts) == 3 {
						s.renderPageHTTP(w, "categories.html", map[string]interface{}{
							"Title":       site.RepoName + " - Categories",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Categories"}},
							"RepoCategories":  site.Categories,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					} else if len(parts) == 4 {
						catName := parts[3]
						var catData *g2.CategoryData
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
							Name                  string
							ReposList             []*g2.SiteData
							EbuildCount           int
							HighestStableVersion  template.HTML
							HighestTestingVersion template.HTML
							DominantDescription   string
							DominantHomepage      string
							DominantLicense       string
							ReverseVirtuals       []string
						}
						var tmplPkgs []TmplPkg
						for _, p := range catData.Packages {
							tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: []*g2.SiteData{site}, EbuildCount: p.EbuildCount, HighestStableVersion: p.HighestStableVersion.(template.HTML), HighestTestingVersion: p.HighestTestingVersion.(template.HTML), DominantDescription: p.DominantDescription, DominantHomepage: p.DominantHomepage, DominantLicense: p.DominantLicense, ReverseVirtuals: p.ReverseVirtuals})
						}

						s.renderPageHTTP(w, "category.html", map[string]interface{}{
							"Title":       "Category: " + catData.Name,
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: catData.Name}},
							"Category":    map[string]interface{}{"Name": catData.Name, "Packages": tmplPkgs},
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					} else if len(parts) >= 6 && parts[4] == "packages" {
						catName := parts[3]
						pkgName := parts[5]

						var pkgData *g2.PackageData
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

						validLicenses := make(map[string]bool)
						for _, lic := range s.AggLicenses {
							validLicenses[lic.Name] = true
						}

						if len(parts) == 6 {
							s.renderPageHTTP(w, "repo_package.html", map[string]interface{}{
								"Title":   fmt.Sprintf("%s - %s/%s", site.RepoName, pkgData.Category, pkgData.Name),
								"BaseURL": baseURL,
								"Breadcrumbs": []g2.Breadcrumb{
									{Name: s.Title, URL: baseURL},
									{Name: site.RepoName, URL: "../../../../"},
									{Name: "Categories", URL: "../../../"},
									{Name: pkgData.Category},
									{Name: pkgData.Name},
								},
								"Repo":          site,
								"Package":       *pkgData,
								"Version":       version,
								"GenInfo":       s.GenInfo,
								"ValidLicenses": validLicenses,
							})
							return
						} else if len(parts) == 8 && parts[6] == "ebuild" {
							versionName := strings.TrimSuffix(parts[7], ".html")

							var versionData *g2.VersionData
							for _, v := range pkgData.Versions {
								if v.Version == versionName || (v.Ebuild != nil && v.Ebuild.Vars != nil && v.Ebuild.Vars["PV"] == versionName) {
									versionData = &v
									break
								}
							}
							if versionData == nil {
								http.NotFound(w, r)
								return
							}

							var filteredManifest []g2.ManifestEntryData
							for _, md := range pkgData.ManifestData {
								for _, v := range md.Versions {
									if v == versionData.Version || (versionData.Ebuild != nil && versionData.Ebuild.Vars != nil && v == versionData.Ebuild.Vars["PV"]) {
										filteredManifest = append(filteredManifest, md)
										break
									}
								}
							}

							s.renderPageHTTP(w, "ebuild_details.html", map[string]interface{}{
								"Title":   fmt.Sprintf("%s - %s/%s-%s", site.RepoName, pkgData.Category, pkgData.Name, versionName),
								"BaseURL": baseURL,
								"Breadcrumbs": []g2.Breadcrumb{
									{Name: s.Title, URL: baseURL},
									{Name: site.RepoName, URL: "../../../../../../"},
									{Name: "Categories", URL: "../../../../../"},
									{Name: pkgData.Category, URL: "../../../../"},
									{Name: "Packages", URL: "../../../"},
									{Name: pkgData.Name, URL: "../../"},
									{Name: "Ebuild"},
									{Name: versionName},
								},
								"Repo":             site,
								"Package":          *pkgData,
								"g2.VersionData":   *versionData,
								"FilteredManifest": filteredManifest,
								"Version":          version,
								"GenInfo":          s.GenInfo,
								"ValidLicenses":    validLicenses,
							})
							return
						} else if len(parts) == 8 && parts[6] == "manifest" {
							manifestName := strings.TrimSuffix(parts[7], ".html")

							var targetMD *g2.ManifestEntryData
							for _, md := range pkgData.ManifestData {
								if md.Entry.Filename == manifestName {
									targetMD = &md
									break
								}
							}
							if targetMD == nil {
								http.NotFound(w, r)
								return
							}

							s.renderPageHTTP(w, "repo_package_manifest.html", map[string]interface{}{
								"Title":   fmt.Sprintf("%s - %s/%s-Manifest-%s", site.RepoName, pkgData.Category, pkgData.Name, manifestName),
								"BaseURL": baseURL,
								"Breadcrumbs": []g2.Breadcrumb{
									{Name: s.Title, URL: baseURL},
									{Name: site.RepoName, URL: "../../../../../../"},
									{Name: "Categories", URL: "../../../../../"},
									{Name: pkgData.Category, URL: "../../../../"},
									{Name: "Packages", URL: "../../../"},
									{Name: pkgData.Name, URL: "../../"},
									{Name: "Manifest"},
									{Name: manifestName},
								},
								"Repo":     site,
								"Package":  *pkgData,
								"Manifest": *targetMD,
								"Version":  version,
								"GenInfo":  s.GenInfo,
							})
							return
						}

					}

				case "packages":
					if len(parts) == 3 {
						var repoPkgs []g2.PackageData
						for _, c := range site.Categories {
							repoPkgs = append(repoPkgs, c.Packages...)
						}
						sort.Slice(repoPkgs, func(i, j int) bool {
							if repoPkgs[i].Category == repoPkgs[j].Category {
								return repoPkgs[i].Name < repoPkgs[j].Name
							}
							return repoPkgs[i].Category < repoPkgs[j].Category
						})

						s.renderPageHTTP(w, "repo_packages.html", map[string]interface{}{
							"Title":       site.RepoName + " - Packages",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Packages"}},
							"Packages":    repoPkgs,
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					}

				case "profiles":
					if len(parts) == 3 {
						s.renderPageHTTP(w, "repo_profiles.html", map[string]interface{}{
							"Title":       site.RepoName + " - Profiles",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "Profiles"}},
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					} else if len(parts) >= 4 {
						// Extract profile path which might have multiple slashes
						// URL: repos/overlay1/profiles/default/linux/amd64/17.0
						// Or URL: repos/overlay1/profiles/default/linux/amd64/17.0/make.defaults.html

						// Reconstruct the remaining parts
						remaining := strings.Join(parts[3:], "/")

						// Check if it's a file request ending in .html
						var requestedFile string
						var profilePath string

						if strings.HasSuffix(remaining, ".html") {
							// E.g., default/linux/amd64/17.0/make.defaults.html
							// Split at the last slash
							lastSlash := strings.LastIndex(remaining, "/")
							if lastSlash != -1 {
								profilePath = remaining[:lastSlash]
								requestedFile = strings.TrimSuffix(remaining[lastSlash+1:], ".html")
							} else {
								profilePath = ""
								requestedFile = strings.TrimSuffix(remaining, ".html")
							}
						} else {
							profilePath = remaining
						}

						var targetProfile *g2.ProfileData
						for i, p := range site.Profiles {
							if p.Path == profilePath {
								targetProfile = &site.Profiles[i]
								break
							}
						}

						if targetProfile == nil {
							http.NotFound(w, r)
							return
						}

						if requestedFile != "" {
							fileContent, exists := targetProfile.Files[requestedFile]
							if !exists {
								http.NotFound(w, r)
								return
							}

							s.renderPageHTTP(w, "repo_profile_file.html", map[string]interface{}{
								"Title":       site.RepoName + " - Profile File: " + requestedFile,
								"BaseURL":     baseURL,
								"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: baseURL + "repos/" + site.RepoName + "/"}, {Name: "Profiles", URL: baseURL + "repos/" + site.RepoName + "/profiles/"}, {Name: targetProfile.Path, URL: "index.html"}, {Name: requestedFile}},
								"RepoName":    site.RepoName,
								"ProfilePath": targetProfile.Path,
								"Profile":     targetProfile,
								"FileName":    requestedFile,
								"FileContent": fileContent,
								"Version":     version,
								"GenInfo":     s.GenInfo,
							})
							return
						} else {
							s.renderPageHTTP(w, "repo_profile.html", map[string]interface{}{
								"Title":       site.RepoName + " - Profile: " + targetProfile.Path,
								"BaseURL":     baseURL,
								"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: baseURL + "repos/" + site.RepoName + "/"}, {Name: "Profiles", URL: baseURL + "repos/" + site.RepoName + "/profiles/"}, {Name: targetProfile.Path}},
								"RepoName":    site.RepoName,
								"ProfilePath": targetProfile.Path,
								"Profile":     targetProfile,
								"Version":     version,
								"GenInfo":     s.GenInfo,
							})
							return
						}
					}

				case "uses":
					repoUseFlags := getRepoUseFlags(site, s.PkgMap)

					if len(parts) == 3 {
						s.renderPageHTTP(w, "repo_uses.html", map[string]interface{}{
							"Title":       site.RepoName + " - USE Flags",
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../"}, {Name: "USE Flags"}},
							"UseFlags":    repoUseFlags,
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					} else if len(parts) == 4 {
						flagName := parts[3]
						var flag *AggUseFlag
						for _, f := range repoUseFlags {
							if f.Name == flagName {
								flag = f
								break
							}
						}
						if flag == nil {
							http.NotFound(w, r)
							return
						}

						s.renderPageHTTP(w, "repo_use.html", map[string]interface{}{
							"Title":       site.RepoName + " - USE Flag: " + flag.Name,
							"BaseURL":     baseURL,
							"Breadcrumbs": []g2.Breadcrumb{{Name: s.Title, URL: baseURL}, {Name: site.RepoName, URL: "../../"}, {Name: "USE Flags", URL: "../"}, {Name: flag.Name}},
							"UseFlag":     flag,
							"Repo":        site,
							"Version":     version,
							"GenInfo":     s.GenInfo,
						})
						return
					}
				}
			}
		}
	}

	http.NotFound(w, r)
}
