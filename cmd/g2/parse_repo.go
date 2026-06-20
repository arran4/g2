package main

import (
	"encoding/json"
	"fmt"

	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

// resolveRemoteURL determines the best remote URL to use for the repository.
func resolveRemoteURL(repoDir string, repoInfo *g2.Repository, remoteURL string) string {
	isUselessURL := func(u string) bool {
		u = strings.TrimSpace(u)
		return u == "" || u == "." || u == ".." || u == "./" || u == "../"
	}

	if !isUselessURL(remoteURL) {
		return remoteURL
	}

	// 1. Git origin URL
	gitURL, err := getGitOriginURL(repoDir)
	if err == nil && !isUselessURL(gitURL) {
		return gitURL
	}

	// 2. RepoInfo Sources
	if repoInfo != nil && len(repoInfo.Sources) > 0 {
		for _, src := range repoInfo.Sources {
			if !isUselessURL(src.Text) {
				return src.Text
			}
		}
	}

	// 3. RepoInfo Homepage
	if repoInfo != nil && !isUselessURL(repoInfo.Homepage) {
		return repoInfo.Homepage
	}

	return remoteURL
}

// parseRepoProfilesDir extracts information from the profiles directory such as layout, licenses, policies, masks, and architectures.
func parseRepoProfilesDir(sysFS fs.FS, repoDir string, site *g2.SiteData) {
	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	if f, err := sysFS.Open(filepath.ToSlash(layoutConfPath)); err == nil {
		_ = f.Close()
		if lc, err := parseLayoutConfFromFS(sysFS, filepath.ToSlash(layoutConfPath)); err == nil {
			site.LayoutConf = lc
		} else {
			log.Printf("Warning: failed to parse layout.conf: %v", err)
		}
	}

	licenseGroupsPath := filepath.Join(repoDir, "profiles", "license_groups")
	if f, err := sysFS.Open(filepath.ToSlash(licenseGroupsPath)); err == nil {
		groups, err := g2.ParseLicenseGroups(f)
		_ = f.Close()
		if err != nil {
			log.Printf("Warning: failed to parse license_groups: %v", err)
		} else {
			licenseMapping := make(map[string][]string)
			for group, licenses := range groups {
				for _, lic := range licenses {
					licenseMapping[lic] = append(licenseMapping[lic], group)
				}
			}
			site.LicenseMapping = licenseMapping
		}
	}

	qaPolicyPath := filepath.Join(repoDir, "metadata", "qa-policy.conf")
	if f, err := sysFS.Open(filepath.ToSlash(qaPolicyPath)); err == nil {
		defer func() { _ = f.Close() }()
		if qa, err := g2.ParseQAPolicyFromReader(f); err == nil {
			site.QAPolicy = qa
		} else {
			log.Printf("Warning: failed to parse qa-policy.conf: %v", err)
		}
	}

	licensesDir := filepath.Join(repoDir, "licenses")
	if entries, err := fs.ReadDir(sysFS, filepath.ToSlash(licensesDir)); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				site.ProvidedLicenses = append(site.ProvidedLicenses, entry.Name())
			}
		}
	}

	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "use.desc"))); err == nil {
		defer func() { _ = f.Close() }()
		if ud, err := g2.ParseUseDesc(f); err == nil {
			site.UseDesc = ud
		}
	}

	if descs, err := g2.ParseUseExpandDescFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "desc"))); err == nil {
		site.UseExpandDescs = descs
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse use expand desc: %v", err)
	}

	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "use.local.desc"))); err == nil {
		defer func() { _ = f.Close() }()
		if uld, err := g2.ParseUseLocalDesc(f); err == nil {
			site.UseLocalDesc = uld
		}
	}

	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "arch.list"))); err == nil {
		defer func() { _ = f.Close() }()
		if al, err := g2.ParseArchList(f); err == nil {
			site.ArchList = al
		}
	}

	if f, err := sysFS.Open(filepath.ToSlash(filepath.Join(repoDir, "profiles", "arches.desc"))); err == nil {
		defer func() { _ = f.Close() }()
		if ad, err := g2.ParseArchesDesc(f); err == nil {
			site.ArchesDesc = ad
		}
	}

	if parsedDeprecated, err := g2.ParsePackageDeprecatedFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "package.deprecated"))); err == nil {
		site.Deprecated = parsedDeprecated
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse package.deprecated: %v", err)
	}

	if parsedMasked, err := g2.ParsePackageMaskedFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "package.mask"))); err == nil {
		site.Masked = parsedMasked
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse package.mask: %v", err)
	}

	if tm, err := g2.ParseThirdPartyMirrorsFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "thirdpartymirrors"))); err == nil {
		site.ThirdPartyMirrors = tm
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse thirdpartymirrors: %v", err)
	}

	if parsedInfoVars, err := g2.ParseInfoVarsFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "info_vars"))); err == nil {
		site.InfoVars = parsedInfoVars
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse info_vars: %v", err)
	}

	if parsedInfoPkgs, err := g2.ParseInfoPkgsFS(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "info_pkgs"))); err == nil {
		site.InfoPkgs = parsedInfoPkgs
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to parse info_pkgs: %v", err)
	}
}

// parseRepoNews reads the news directory and parses out all news items for the repository.
func parseRepoNews(sysFS fs.FS, repoDir string, site *g2.SiteData) {
	newsDir := filepath.Join(repoDir, "metadata", "news")
	entries, err := fs.ReadDir(sysFS, filepath.ToSlash(newsDir))
	if err != nil {
		return
	}

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

	sort.Slice(site.News, func(i, j int) bool {
		return site.News[i].Posted.After(site.News[j].Posted)
	})
}

// parseRepoAuthors extracts author information from the metadata/AUTHORS file and sets the raw URL.
func parseRepoAuthors(repoDir string, site *g2.SiteData, remoteURL string) {
	authorsFile, err := os.Open(filepath.Join(repoDir, "metadata", "AUTHORS"))
	if err == nil {
		if authors, err := g2.ParseAuthors(authorsFile); err == nil {
			site.Authors = authors
			if remoteURL != "" {
				if commitHash, err := getFileCommit(repoDir, "metadata/AUTHORS"); err == nil && commitHash != "" {
					site.AuthorsURL = generateGitHubRawURL(remoteURL, commitHash, "metadata/AUTHORS")
				}
			}
		} else {
			log.Printf("Warning: failed to parse metadata/AUTHORS: %v", err)
		}
		_ = authorsFile.Close()
	}
}

// parseRepoCategoriesAndPackages recursively crawls and parses the repo's categories, packages, and their ebuilds.
func parseRepoCategoriesAndPackages(sysFS fs.FS, repoDir string, repoName string, fastGit bool, remoteURL string, site *g2.SiteData) error {
	supportedCategories := make(map[string]bool)
	if categoriesBytes, err := fs.ReadFile(sysFS, filepath.ToSlash(filepath.Join(repoDir, "profiles", "categories"))); err == nil {
		for _, line := range strings.Split(string(categoriesBytes), "\n") {
			cat := strings.TrimSpace(line)
			if cat != "" && !strings.HasPrefix(cat, "#") {
				supportedCategories[cat] = true
			}
		}
	}

	deprecatedMap := make(map[string]*g2.PackageDeprecated)
	for i := range site.Deprecated {
		for _, entry := range site.Deprecated[i].Entries {
			pkgName := g2.ExtractPackageNameFromDep(entry.Package)
			if pkgName != "" {
				deprecatedMap[pkgName] = &site.Deprecated[i]
			}
		}
	}

	maskedMap := make(map[string]*g2.PackageMasked)
	for i := range site.Masked {
		for _, entry := range site.Masked[i].Entries {
			pkgName := g2.ExtractPackageNameFromDep(entry.Package)
			if pkgName != "" {
				maskedMap[pkgName] = &site.Masked[i]
			}
		}
	}

	slotMovesMap := make(map[string][]g2.PackageSlotMove)
	if site.SlotMoves != nil {
		for _, sm := range site.SlotMoves {
			slotMovesMap[sm.Package] = append(slotMovesMap[sm.Package], sm)
		}
	}

	infoPkgsMap := make(map[string]bool)
	for j := range site.InfoPkgs {
		atom := site.InfoPkgs[j].PackageAtom
		baseAtom := atom
		if idx := strings.Index(atom, ":"); idx != -1 {
			baseAtom = atom[:idx]
		}
		infoPkgsMap[baseAtom] = true
	}

	entries, err := fs.ReadDir(sysFS, filepath.ToSlash(repoDir))
	if err != nil {
		return fmt.Errorf("reading repo dir: %w", err)
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

		catData := g2.CategoryData{Name: name}
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
			pkgData := g2.PackageData{
				Name:     pkgName,
				Category: name,
			}

			pkgStr := name + "/" + pkgName

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
					log.Printf("Warning: parsing ebuild %s in repo %s: %v", ebuildPath, repoName, err)
					continue
				}

				for _, w := range ebuild.ParseWarnings {
					log.Printf("Warning: parsing ebuild %s in repo %s: %v", ebuildPath, repoName, w)
				}

				version := ""
				if ebuild.Vars != nil {
					version = ebuild.Vars["PV"]
				}
				if version == "" {
					vars := g2.ParseEbuildVariables(file.Name())
					if vars != nil {
						version = vars["PV"]
					}
				}

				var ebuildRawURL string
				relPath, _ := filepath.Rel(repoDir, ebuildPath)
				if remoteURL != "" {
					if commitHash, _ := getFileCommit(repoDir, relPath); commitHash != "" {
						ebuildRawURL = generateGitHubRawURL(remoteURL, commitHash, relPath)
					}
				}

				modTime := getFileModTime(repoDir, relPath, fastGit)
				if modTime.After(pkgData.ModTime) {
					pkgData.ModTime = modTime
				}

				vd := g2.VersionData{
					Version:      version,
					Ebuild:       ebuild,
					EbuildRawURL: ebuildRawURL,
					ModTime:      modTime,
				}

				if slot := ebuild.Vars["SLOT"]; slot != "" {
					if moves, ok := slotMovesMap[pkgStr]; ok {
						for _, sm := range moves {
							if sm.Old == slot {
								vd.MovedToSlot = sm.New
								break
							}
						}
					}
				}

				pkgData.Versions = append(pkgData.Versions, vd)
			}

			if len(pkgData.Versions) == 0 {
				continue
			}

			pkgData.HighestStableVersion, pkgData.HighestTestingVersion, pkgData.SnapshotVersion, pkgData.EbuildCount = getHighestVersionsAndCount(pkgData.Versions, site)

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

			var highestUnmasked *g2.Ebuild
			var highestMasked *g2.Ebuild
			for _, v := range pkgData.Versions {
				if v.Ebuild == nil || v.Ebuild.Vars == nil {
					continue
				}
				isMasked := true
				for _, p := range strings.Fields(v.Ebuild.Vars["KEYWORDS"]) {
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
				if commitHash, _ := getFileCommit(repoDir, relPath); commitHash != "" {
					pkgData.MetadataRawURL = generateGitHubRawURL(remoteURL, commitHash, relPath)
				}
			}

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

			manifestPath := filepath.Join(pkgPath, "Manifest")
			manifest, err := parseManifestFromFS(sysFS, filepath.ToSlash(manifestPath))
			if err == nil {
				pkgData.Manifest = manifest
				pkgData.ManifestData = buildManifestData(manifest, pkgData.Versions, site.ThirdPartyMirrors)
			}

			filesDirPath := filepath.Join(pkgPath, "files")
			if info, err := fs.Stat(sysFS, filepath.ToSlash(filesDirPath)); err == nil && info.IsDir() {
				fileEntries, err := fs.ReadDir(sysFS, filepath.ToSlash(filesDirPath))
				if err == nil {
					for _, fe := range fileEntries {
						if !fe.IsDir() {
							fd := g2.FileData{
								Name: fe.Name(),
								Path: filepath.Join(filesDirPath, fe.Name()),
							}
							if remoteURL != "" {
								relPath, _ := filepath.Rel(repoDir, fd.Path)
								if commitHash, _ := getFileCommit(repoDir, relPath); commitHash != "" {
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

			if dep, ok := deprecatedMap[pkgStr]; ok {
				pkgData.Deprecated = dep
			}

			if mask, ok := maskedMap[pkgStr]; ok {
				pkgData.Masked = mask
			}

			for i, v := range pkgData.Versions {
				pkgData.Versions[i].Deprecated = pkgData.Deprecated
				pkgData.Versions[i].Masked = pkgData.Masked

				g2PkgData.Versions = append(g2PkgData.Versions, g2.VersionData{
					Version:      v.Version,
					Ebuild:       v.Ebuild,
					EbuildRawURL: v.EbuildRawURL,
					Deprecated:   pkgData.Versions[i].Deprecated,
					Masked:       pkgData.Versions[i].Masked,
				})
			}

			if infoPkgsMap[pkgStr] {
				pkgData.IsInfoPkg = true
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

			sort.Slice(catData.Packages, func(i, j int) bool {
				return catData.Packages[i].Name < catData.Packages[j].Name
			})
			site.Categories = append(site.Categories, catData)
		}
	}

	sort.Slice(site.Categories, func(i, j int) bool {
		return site.Categories[i].Name < site.Categories[j].Name
	})

	pkgMap := make(map[string]bool)
	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			pkgMap[site.Categories[i].Packages[j].Category+"/"+site.Categories[i].Packages[j].Name] = true
		}
	}

	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			for k := range site.Categories[i].Packages[j].Versions {
				ver := &site.Categories[i].Packages[j].Versions[k]
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					depsMap := map[string][]ResolvedDepNode{}
					for _, depType := range []string{"DEPEND", "RDEPEND", "BDEPEND", "PDEPEND", "REQUIRED_USE", "LICENSE"} {
						if depStr := ver.Ebuild.Vars[depType]; depStr != "" {
							tree := g2.ParseDepTree(depStr)
							var nodes []ResolvedDepNode
							for _, n := range tree.Nodes {
								nodes = append(nodes, resolveDependencies(n, pkgMap))
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

	return nil
}

// parseRepoEclasses parses .eclass files in the eclass directory.
func parseRepoEclasses(sysFS fs.FS, repoDir string, site *g2.SiteData) {
	eclassDir := filepath.Join(repoDir, "eclass")
	if info, err := fs.Stat(sysFS, filepath.ToSlash(eclassDir)); err == nil && info.IsDir() {
		entries, err := fs.ReadDir(sysFS, filepath.ToSlash(eclassDir))
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".eclass") {
					ebuild, err := g2.ParseEbuild(sysFS, filepath.ToSlash(filepath.Join(eclassDir, e.Name())), g2.ParseFull)
					if err == nil {
						site.Eclasses = append(site.Eclasses, ebuild)
						for _, w := range ebuild.ParseWarnings {
							log.Printf("Warning: parsing eclass %s in repo %s: %v", e.Name(), site.RepoName, w)
						}
					} else {
						log.Printf("Warning: failed to parse eclass %s in repo %s: %v", e.Name(), site.RepoName, err)
					}
				}
			}
		}
	}
	sort.Slice(site.Eclasses, func(i, j int) bool {
		return site.Eclasses[i].Vars["PN"] < site.Eclasses[j].Vars["PN"]
	})
}
