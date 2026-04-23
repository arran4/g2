package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arran4/g2"
	"golang.org/x/sync/errgroup"
	"sort"
)

func generateGlobalPages(outDir string, tmpl *template.Template, sites []*g2.SiteData, data *AggregatedData, title, version, recentDurationStr string, genInfo GenerationInfo) error {
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
		GlobalCategories:     data.Categories,
		GlobalPackages:       data.Packages,
		Licenses:             data.Licenses,
		UseFlags:             data.UseFlags,
		Projects:             data.Projects,
		GlobalProfiles:       data.Profiles,
		Arches:               data.Arches,
		Version:              version,
		GenInfo:              genInfo,
		RecentDurationString: recentDurationStr,
		RecentGlobalNews:     data.RecentNews,
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
			Title:            "News Dashboard",
			BaseURL:          "../",
			Breadcrumbs:      []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "News"}},
			RecentGlobalNews: data.RecentNews,
			Version:          version,
			GenInfo:          genInfo,
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
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
				Title:          n.Title,
				BaseURL:        "../../../",
				Breadcrumbs:    []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
				GlobalNewsItem: &n,
				Version:        version,
				GenInfo:        genInfo,
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
		Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "Overlays"}},
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
		Title:            "Categories",
		BaseURL:          "../",
		Breadcrumbs:      []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "Categories"}},
		GlobalCategories: data.Categories,
		Version:          version,
		GenInfo:          genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	g := new(errgroup.Group)
	g.SetLimit(10)
	for _, cat := range data.Categories {
		cat := cat
		g.Go(func() error {
			catDirName := sanitizeFilename(cat.Name)
			if catDirName == "" {
				return nil
			}
			// NOTE: Sanitize modifies directory generation, but not template linkages,
			// but since null-bytes shouldn't be in valid names anyway, this prevents the filesystem crash.
			catDir := filepath.Join(outDir, "categories", catDirName)
			if err := os.MkdirAll(catDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", catDir, err)
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
				Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
				Category:    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
			return nil
		})
	}
	return g.Wait()
}

func generatePackagePages(outDir string, tmpl *template.Template, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
	// Global Moved Packages Pages
	pkgMap := make(map[string]bool)
	for _, p := range data.Packages {
		pkgMap[p.Category+"/"+p.Name] = true
	}

	gMoves := new(errgroup.Group)
	gMoves.SetLimit(10)
	for oldPath, move := range data.Moves {
		oldPath, move := oldPath, move
		gMoves.Go(func() error {
			parts := strings.Split(oldPath, "/")
			if len(parts) != 2 {
				return nil
			}
			oldCat, oldName := parts[0], parts[1]

			if pkgMap[oldPath] {
				return nil // skip if a package now exists at this location
			}

			newParts := strings.Split(move.New, "/")
			if len(newParts) != 2 {
				return nil
			}

			pkgDir := filepath.Join(outDir, "packages", oldCat, oldName)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", pkgDir, err)
			}

			rootNode := &PageNode{Name: title, Path: ""}
			packagesNode := &PageNode{Parent: rootNode, Name: "Packages", Path: "packages"}
			catNode := &PageNode{Parent: packagesNode, Name: oldCat, Path: "packages/" + oldCat}
			pkgNode := &PageNode{Parent: catNode, Name: oldName, Path: "packages/" + oldCat + "/" + oldName}

			ctx := pkgNode.Context("Package Moved: "+oldCat+"/"+oldName, version, genInfo)
			ctx.OldName = oldCat + "/" + oldName
			ctx.NewName = move.New
			ctx.NewURL = "../../" + newParts[0] + "/" + newParts[1] + "/"

			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "moved_package.html", ctx); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
			return nil
		})
	}
	if err := gMoves.Wait(); err != nil {
		return err
	}

	// 4. Global Packages
	if err := os.MkdirAll(filepath.Join(outDir, "packages"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "packages", "index.html"), tmpl, "packages.html", GenericPageContext{
		Title:          "Packages",
		BaseURL:        "../",
		Breadcrumbs:    []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "Packages"}},
		GlobalPackages: data.Packages,
		Version:        version,
		GenInfo:        genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	gPkgs := new(errgroup.Group)
	gPkgs.SetLimit(10)
	for _, pkg := range data.Packages {
		pkg := pkg
		gPkgs.Go(func() error {
			pkgDir := filepath.Join(outDir, "packages", pkg.Category, pkg.Name)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", pkgDir, err)
			}

			reposList := mapToList(pkg.Repos)

			if len(reposList) == 1 {
				targetURL := fmt.Sprintf("../../../repos/%s/categories/%s/packages/%s/", reposList[0].RepoName, pkg.Category, pkg.Name)
				if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "redirect.html", GenericPageContext{
					TargetURL: targetURL,
				}); err != nil {
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

				rootNode := &PageNode{Name: title, Path: ""}
				packagesNode := &PageNode{Parent: rootNode, Name: "Packages", Path: "packages"}
				catNode := &PageNode{Parent: packagesNode, Name: pkg.Category, Path: "packages/" + pkg.Category}
				pkgNode := &PageNode{Parent: catNode, Name: pkg.Name, Path: "packages/" + pkg.Category + "/" + pkg.Name}

				ctx := pkgNode.Context("Package: "+pkg.Category+"/"+pkg.Name, version, genInfo)
				ctx.GlobalPackage = pkg
				ctx.MovedToName = movedToName
				ctx.MovedToURL = movedToURL

				if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "package_picker.html", ctx); err != nil {
					return fmt.Errorf("rendering page: %w", err)
				}
			}
			return nil
		})
	}
	return gPkgs.Wait()
}

func generateOtherGlobalPages(outDir string, tmpl *template.Template, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
	// Arches
	if err := os.MkdirAll(filepath.Join(outDir, "arches"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(outDir, "arches", "index.html"), tmpl, "arches.html", GenericPageContext{
		Title:       "Architectures",
		BaseURL:     "../",
		Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "Architectures"}},
		Arches:      data.Arches,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	gArches := new(errgroup.Group)
	gArches.SetLimit(10)
	for _, a := range data.Arches {
		a := a
		gArches.Go(func() error {
			archDir := filepath.Join(outDir, "arches", a.Name)
			if err := os.MkdirAll(archDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", archDir, err)
			}

			if err := renderPage(filepath.Join(archDir, "index.html"), tmpl, "arch.html", GenericPageContext{
				Title:       "Architecture: " + a.Name,
				BaseURL:     "../../",
				Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "Architectures", URL: "../../arches/"}, {Name: a.Name}},
				Arch:        a,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
			return nil
		})
	}
	if err := gArches.Wait(); err != nil {
		return err
	}

	// Profiles
	if len(data.Profiles) > 0 {
		if err := os.MkdirAll(filepath.Join(outDir, "profiles"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(outDir, "profiles", "index.html"), tmpl, "profiles.html", GenericPageContext{
			Title:          "Profiles",
			BaseURL:        "../",
			Breadcrumbs:    []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "Profiles"}},
			GlobalProfiles: data.Profiles,
			Version:        version,
			GenInfo:        genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		gProfiles := new(errgroup.Group)
		gProfiles.SetLimit(10)
		for _, p := range data.Profiles {
			p := p
			gProfiles.Go(func() error {
				profDir := filepath.Join(outDir, "profiles", p.Path)
				if err := os.MkdirAll(profDir, 0755); err != nil {
					return fmt.Errorf("creating directory %s: %w", profDir, err)
				}

				relToRoot := "../../" + strings.Repeat("../", strings.Count(p.Path, "/"))

				if err := renderPage(filepath.Join(profDir, "index.html"), tmpl, "profile.html", GenericPageContext{
					Title:       "Profile: " + p.Path,
					BaseURL:     relToRoot,
					Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: relToRoot}, {Name: "Profiles", URL: relToRoot + "profiles/"}, {Name: p.Path}},
					ProfilePath: p.Path,
					ProfileList: p.Repos,
					Version:     version,
					GenInfo:     genInfo,
				}); err != nil {
					return fmt.Errorf("rendering page: %w", err)
				}
				return nil
			})
		}
		if err := gProfiles.Wait(); err != nil {
			return err
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
		Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "USE Flags"}},
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
			Title:         "USE Flag: " + f.Name,
			BaseURL:       "../../",
			Breadcrumbs:   []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "USE Flags", URL: "../"}, {Name: f.Name}},
			GlobalUseFlag: f,
			Version:       version,
			GenInfo:       genInfo,
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
			Breadcrumbs:    []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "USE_EXPAND Flags"}},
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
				Breadcrumbs:   []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "USE_EXPAND Flags", URL: "../"}, {Name: prefix}},
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
		Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "Licenses"}},
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "Licenses", URL: "../"}, {Name: lic.Name}},
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../"}, {Name: "Projects"}},
			Projects:    data.Projects,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		gProjects := new(errgroup.Group)
		gProjects.SetLimit(10)
		for _, proj := range data.Projects {
			proj := proj
			gProjects.Go(func() error {
				projDirName := sanitizeFilename(proj.Project.Email)
				if projDirName == "" {
					return nil
				}
				projDir := filepath.Join(outDir, "projects", projDirName)
				if err := os.MkdirAll(projDir, 0755); err != nil {
					return fmt.Errorf("creating directory %s: %w", projDir, err)
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

				if err := renderPage(filepath.Join(projDir, "index.html"), tmpl, "project.html", GenericPageContext{
					Title:       "Project: " + proj.Project.Name,
					BaseURL:     "../../",
					Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "Projects", URL: "../"}, {Name: proj.Project.Name}},
					Project:     proj,
					Packages:    tmplPkgs, // Legacy any for TmplPkgs
					Version:     version,
					GenInfo:     genInfo,
				}); err != nil {
					return fmt.Errorf("rendering page: %w", err)
				}
				return nil
			})
		}
		if err := gProjects.Wait(); err != nil {
			return err
		}

	}

	return nil
}

func generateRepoPages(outDir string, tmpl *template.Template, sites []*g2.SiteData, data *AggregatedData, title, version, recentDurationStr string, genInfo GenerationInfo) error {
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
		Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "All Overlays"}},
		Repos:       sites,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	if err := generateRepoGroupPages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}

	g := new(errgroup.Group)
	g.SetLimit(10)

	for _, site := range sites {
		site := site // loop variable capture
		g.Go(func() error {
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
			return nil
		})
	}

	return g.Wait()
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "Overlays: " + group.Quality + " - " + group.Status}},
			Group:       &group,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoUseFlagsPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title, version string, genInfo GenerationInfo) error {
	if site.AggUseFlags != nil && len(site.AggUseFlags.([]*AggUseFlag)) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "uses"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "uses", "index.html"), tmpl, "uses.html", GenericPageContext{
			Title:       site.RepoName + " - USE Flags",
			BaseURL:     "../../../",
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "USE Flags"}},
			UseFlags:    site.AggUseFlags.([]*AggUseFlag),
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}

		for _, f := range site.AggUseFlags.([]*AggUseFlag) {
			safeName := strings.ReplaceAll(f.Name, "/", "_")
			useDir := filepath.Join(repoDir, "uses", safeName)
			if err := os.MkdirAll(useDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", useDir, err)
			}

			if err := renderPage(filepath.Join(useDir, "index.html"), tmpl, "use.html", GenericPageContext{
				Title:         "USE Flag: " + f.Name,
				BaseURL:       "../../../../",
				Breadcrumbs:   []g2.Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "USE Flags", URL: "../"}, {Name: f.Name}},
				GlobalUseFlag: f,
				Version:       version,
				GenInfo:       genInfo,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateRepoUseExpandsPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title, version string, genInfo GenerationInfo) error {
	if len(site.UseExpandDescs) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "uses_expand"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "uses_expand", "index.html"), tmpl, "repo_use_expands.html", GenericPageContext{
			Title:       site.RepoName + " - USE_EXPAND Flags",
			BaseURL:     "../../../",
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "USE_EXPAND Flags"}},
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
				Breadcrumbs:   []g2.Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "USE_EXPAND Flags", URL: "../"}, {Name: prefix}},
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

func generateRepoDeprecatedMaskedInfoPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title string) error {
	if len(site.Deprecated) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "deprecated"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "deprecated", "index.html"), tmpl, "repo_deprecated.html", GenericPageContext{
			Title:       site.RepoName + " - Deprecated",
			BaseURL:     "../../../",
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Deprecated Packages"}},
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Masked Packages"}},
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Info Vars"}},
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Info Packages"}},
			Repo:        site,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoMovedPackagesPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title, version string, genInfo GenerationInfo) error {
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

		rootNode := &PageNode{Name: title, Path: ""}
		packagesNode := &PageNode{Parent: rootNode, Name: "Packages", Path: "packages"}
		catNode := &PageNode{Parent: packagesNode, Name: oldCat, Path: "packages/" + oldCat}
		pkgNode := &PageNode{Parent: catNode, Name: oldName, Path: "packages/" + oldCat + "/" + oldName}

		ctx := pkgNode.Context("Package Moved: "+oldCat+"/"+oldName, version, genInfo)
		ctx.OldName = oldCat + "/" + oldName
		ctx.NewName = move.New
		ctx.NewURL = "../../" + newParts[0] + "/" + newParts[1] + "/"

		if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "moved_package.html", ctx); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoIndexAndStatsPages(repoDir string, tmpl *template.Template, site *g2.SiteData, data *AggregatedData, title, version, recentDurationStr string, genInfo GenerationInfo, repoRecentNews []g2.NewsItem) error {
	if err := renderPage(filepath.Join(repoDir, "index.html"), tmpl, "repo_index.html", GenericPageContext{
		Title:                 site.RepoName,
		BaseURL:               "../../",
		Breadcrumbs:           []g2.Breadcrumb{{Name: title, URL: "../../"}, {Name: "Overlays", URL: "../../overlays/"}, {Name: site.RepoName}},
		Repo:                  site,
		PackageCount:          site.PackageCount,
		Version:               version,
		GenInfo:               genInfo,
		RecentDurationString:  recentDurationStr,
		RecentRepoNews:        repoRecentNews,
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
		Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Overlays", URL: "../../../overlays/"}, {Name: site.RepoName, URL: "../"}, {Name: "Statistics"}},
		Repo:        site,
		Version:     version,
		GenInfo:     genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}
	return nil
}

func generateRepoProfilesPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title, version string, genInfo GenerationInfo) error {
	if len(site.Profiles) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "profiles"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "profiles", "index.html"), tmpl, "repo_profiles.html", GenericPageContext{
			Title:       site.RepoName + " - Profiles",
			BaseURL:     "../../../",
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Profiles"}},
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
				Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: relToRoot}, {Name: site.RepoName, URL: relToRoot + "repos/" + site.RepoName + "/"}, {Name: "Profiles", URL: relToRoot + "repos/" + site.RepoName + "/profiles/"}, {Name: p.Path}},
				RepoName:    site.RepoName,
				ProfilePath: p.Path,
				RepoProfile: &p,
				Version:     version,
				GenInfo:     genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}

			for fName, fContent := range p.Files {
				if err := renderPage(filepath.Join(profDir, fName+".html"), tmpl, "repo_profile_file.html", GenericPageContext{
					Title:       site.RepoName + " - Profile File: " + fName,
					BaseURL:     relToRoot,
					Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: relToRoot}, {Name: site.RepoName, URL: relToRoot + "repos/" + site.RepoName + "/"}, {Name: "Profiles", URL: relToRoot + "repos/" + site.RepoName + "/profiles/"}, {Name: p.Path, URL: "index.html"}, {Name: fName}},
					RepoName:    site.RepoName,
					ProfilePath: p.Path,
					RepoProfile: &p,
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

func generateRepoNewsPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title, version string, genInfo GenerationInfo, repoRecentNews []g2.NewsItem) error {
	if len(site.News) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "news"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "news", "index.html"), tmpl, "news_dashboard.html", GenericPageContext{
			Title:          site.RepoName + " - News Dashboard",
			BaseURL:        "../../../",
			Breadcrumbs:    []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: "Overlays", URL: "../../../overlays/"}, {Name: site.RepoName, URL: "../"}, {Name: "News"}},
			RecentRepoNews: repoRecentNews,
			Version:        version,
			GenInfo:        genInfo,
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
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../../"}, {Name: "Overlays", URL: "../../../../overlays/"}, {Name: site.RepoName, URL: "../../"}, {Name: "News", URL: "../"}, {Name: "Archive"}},
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
				Title:        n.Title,
				BaseURL:      "../../../../../",
				Breadcrumbs:  []g2.Breadcrumb{{Name: title, URL: "../../../../../"}, {Name: "Overlays", URL: "../../../../../overlays/"}, {Name: site.RepoName, URL: "../../../"}, {Name: "News", URL: "../../"}, {Name: "Archive", URL: "../"}, {Name: n.Title}},
				RepoNewsItem: &n,
				Version:      version,
				GenInfo:      genInfo,
			}); err != nil {
				return fmt.Errorf("rendering page: %w", err)
			}
		}
	}
	return nil
}

func generateRepoCategoriesPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title, version string, genInfo GenerationInfo) error {
	if err := os.MkdirAll(filepath.Join(repoDir, "categories"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	if err := renderPage(filepath.Join(repoDir, "categories", "index.html"), tmpl, "categories.html", GenericPageContext{
		Title:          site.RepoName + " - Categories",
		BaseURL:        "../../../",
		Breadcrumbs:    []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Categories"}},
		RepoCategories: site.Categories,
		Version:        version,
		GenInfo:        genInfo,
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
			tmplPkgs = append(tmplPkgs, TmplPkg{Name: p.Name, ReposList: []*g2.SiteData{site}, EbuildCount: p.EbuildCount, HighestStableVersion: p.HighestStableVersion.(template.HTML), HighestTestingVersion: p.HighestTestingVersion.(template.HTML), DominantDescription: p.DominantDescription, DominantHomepage: p.DominantHomepage, DominantLicense: p.DominantLicense, ReverseVirtuals: p.ReverseVirtuals})
		}

		if err := renderPage(filepath.Join(catDir, "index.html"), tmpl, "category.html", GenericPageContext{
			Title:       "Category: " + cat.Name,
			BaseURL:     "../../../../",
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "Categories", URL: "../"}, {Name: cat.Name}},
			Category:    map[string]interface{}{"Name": cat.Name, "Packages": tmplPkgs},
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return fmt.Errorf("rendering page: %w", err)
		}
	}
	return nil
}

func generateRepoAuthorsPages(repoDir string, tmpl *template.Template, site *g2.SiteData, title, version string, genInfo GenerationInfo) error {
	if len(site.Authors) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "authors"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "authors", "index.html"), tmpl, "authors.html", GenericPageContext{
			Title:       site.RepoName + " - Authors",
			BaseURL:     "../../../",
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Authors"}},
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

func generateRepoPackagesPages(repoDir string, tmpl *template.Template, site *g2.SiteData, data *AggregatedData, title, version string, genInfo GenerationInfo) error {
	if err := os.MkdirAll(filepath.Join(repoDir, "packages"), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
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

	if err := renderPage(filepath.Join(repoDir, "packages", "index.html"), tmpl, "repo_packages.html", GenericPageContext{
		Title:        site.RepoName + " - Packages",
		BaseURL:      "../../../",
		Breadcrumbs:  []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Packages"}},
		RepoPackages: repoPkgs,
		Repo:         site,
		Version:      version,
		GenInfo:      genInfo,
	}); err != nil {
		return fmt.Errorf("rendering page: %w", err)
	}

	aggPackagesMap := make(map[string]*AggPackage)
	for _, p := range data.Packages {
		aggPackagesMap[p.Category+"/"+p.Name] = p
	}
	repoUseFlags := getRepoUseFlags(site, aggPackagesMap)

	if site.AggEclasses != nil && len(site.AggEclasses.([]*AggEclass)) > 0 {
		if err := os.MkdirAll(filepath.Join(repoDir, "eclasses"), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		if err := renderPage(filepath.Join(repoDir, "eclasses", "index.html"), tmpl, "repo_eclasses.html", GenericPageContext{
			Title:       site.RepoName + " - Eclasses",
			BaseURL:     "../../../",
			Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "Eclasses"}},
			Eclasses:    site.AggEclasses.([]*AggEclass),
			Repo:        site,
			Version:     version,
			GenInfo:     genInfo,
		}); err != nil {
			return err
		}

		for _, ec := range site.AggEclasses.([]*AggEclass) {
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
				Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "Eclasses", URL: "../"}, {Name: ec.Name}},
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
		Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../"}, {Name: site.RepoName, URL: "../"}, {Name: "USE Flags"}},
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
			Title:         site.RepoName + " - USE Flag: " + f.Name,
			BaseURL:       "../../../../",
			Breadcrumbs:   []g2.Breadcrumb{{Name: title, URL: "../../../../"}, {Name: site.RepoName, URL: "../../"}, {Name: "USE Flags", URL: "../"}, {Name: f.Name}},
			GlobalUseFlag: f,
			Repo:          site,
			Version:       version,
			GenInfo:       genInfo,
		}); err != nil {
			return err
		}
	}

	gPkgs := new(errgroup.Group)
	gPkgs.SetLimit(10)
	for _, pkg := range repoPkgs {
		pkg := pkg
		gPkgs.Go(func() error {
			pkgDir := filepath.Join(repoDir, "categories", pkg.Category, "packages", pkg.Name)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", pkgDir, err)
			}

			var movedToName, movedToURL string
			if move, ok := data.Moves[pkg.Category+"/"+pkg.Name]; ok {
				newParts := strings.Split(move.New, "/")
				if len(newParts) == 2 {
					movedToName = move.New
					movedToURL = "../../../" + newParts[0] + "/packages/" + newParts[1] + "/"
				}
			}

			if err := renderPage(filepath.Join(pkgDir, "index.html"), tmpl, "repo_package.html", GenericPageContext{
				Title:         fmt.Sprintf("%s - %s/%s", site.RepoName, pkg.Category, pkg.Name),
				BaseURL:       "../../../../../../",
				Breadcrumbs:   []g2.Breadcrumb{{Name: title, URL: "../../../../../../"}, {Name: site.RepoName, URL: "../../../../"}, {Name: "Categories", URL: "../../../"}, {Name: pkg.Category, URL: "../../"}, {Name: pkg.Name}},
				Repo:          site,
				RepoPackage:   &pkg,
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
						Breadcrumbs: []g2.Breadcrumb{{Name: title, URL: "../../../../../../../../"}, {Name: site.RepoName, URL: "../../../../../../"}, {Name: "Categories", URL: "../../../../../"}, {Name: pkg.Category, URL: "../../../../"}, {Name: pkg.Name, URL: "../../"}, {Name: "Manifest"}, {Name: md.Entry.Filename}},
						Repo:        site,
						RepoPackage: &pkg,
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

				var filteredManifest []g2.ManifestEntryData
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
					Breadcrumbs:      []g2.Breadcrumb{{Name: title, URL: "../../../../../../../../"}, {Name: site.RepoName, URL: "../../../../../../"}, {Name: "Categories", URL: "../../../../../"}, {Name: pkg.Category, URL: "../../../../"}, {Name: "Packages", URL: "../../../"}, {Name: pkg.Name, URL: "../../"}, {Name: "Ebuild", URL: "../"}, {Name: versionStr}},
					Repo:             site,
					RepoPackage:      &pkg,
					VersionData:      &v,
					FilteredManifest: filteredManifest,
					Version:          version,
					GenInfo:          genInfo,
					ValidLicenses:    data.ValidLicenses,
				}); err != nil {
					return fmt.Errorf("rendering page: %w", err)
				}
			}
			return nil
		})
	}
	return gPkgs.Wait()
}

func generateSite(outDir string, sites []*g2.SiteData, recentDuration time.Duration, recentDurationStr string, genInfo GenerationInfo) error {
	if genInfo.Profiler == nil {
		genInfo.Profiler = NewProfiler(false, "")
	}
	defer func() {
		if err := genInfo.Profiler.Write(); err != nil {
			log.Printf("Error writing profile: %v", err)
		}
	}()

	defer genInfo.Profiler.Track("Total Generation")()

	log.Printf("Starting site generation (v%s) with %d repos to %s", version, len(sites), outDir)
	stepMkdir := genInfo.Profiler.Track("Create Output Directory")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	stepMkdir()

	stepPopulate := genInfo.Profiler.Track("Populate Package Use Flags")
	for _, site := range sites {
		populatePkgUseFlags(site)
	}
	stepPopulate()

	// Generate search index
	stepSearchIndex := genInfo.Profiler.Track("Generate Search Index")
	if err := generateSearchIndex(outDir, sites); err != nil {
		log.Printf("Warning: failed to generate search index: %v", err)
	}
	stepSearchIndex()

	stepTemplates := genInfo.Profiler.Track("Load Templates")
	tmpl, err := GetSiteTemplates()
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}
	stepTemplates()

	for _, site := range sites {
		aggPackagesMap := make(map[string]*AggPackage)
		for _, cat := range site.Categories {
			for _, pkg := range cat.Packages {
				pkgKey := cat.Name + "/" + pkg.Name
				aggPackagesMap[pkgKey] = &AggPackage{
					Name:                pkg.Name,
					Category:            cat.Name,
					Repos:               map[string]*g2.SiteData{site.RepoName: site},
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
	log.Printf("[PHASE] Rendering global pages...")
	stepGlobalPages := genInfo.Profiler.Track("Render Global Pages")
	if err := generateGlobalPages(outDir, tmpl, sites, data, title, version, recentDurationStr, genInfo); err != nil {
		return err
	}
	stepGlobalPages()

	log.Printf("[PHASE] Rendering %d category pages...", len(data.Categories))
	stepCategoryPages := genInfo.Profiler.Track("Render Category Pages")
	if err := generateCategoryPages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}
	stepCategoryPages()

	log.Printf("[PHASE] Rendering package pages...")
	stepPackagePages := genInfo.Profiler.Track("Render Package Pages")
	if err := generatePackagePages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}
	stepPackagePages()

	log.Printf("[PHASE] Rendering other global pages...")
	stepOtherGlobalPages := genInfo.Profiler.Track("Render Other Global Pages")
	if err := generateOtherGlobalPages(outDir, tmpl, data, title, version, genInfo); err != nil {
		return err
	}
	stepOtherGlobalPages()

	log.Printf("[PHASE] Rendering %d repository pages...", len(sites))
	stepRepositoryPages := genInfo.Profiler.Track("Render Repository Pages")
	if err := generateRepoPages(outDir, tmpl, sites, data, title, version, recentDurationStr, genInfo); err != nil {
		return err
	}
	stepRepositoryPages()

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
