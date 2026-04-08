package main

import (
	"fmt"
	"github.com/arran4/g2"
	"sort"
	"strings"
)

// AggregateUseFlags processes a list of g2.SiteData and updates an aggregate map of UseFlags,
// and returns the sorted global UseFlags list. It also aggregates per-repo USE flags inside g2.SiteData.
func AggregateUseFlags(sites []*g2.SiteData, aggPackages map[string]*AggPackage) ([]*AggUseFlag, map[string]*AggUseFlag) {
	globalAggUseFlags := make(map[string]*AggUseFlag)

	for _, site := range sites {
		repoAggUseFlags := make(map[string]*AggUseFlag)

		for _, pkg := range site.Categories {
			for _, p := range pkg.Packages {
				pkgKey := p.Category + "/" + p.Name

				if p.Metadata != nil {
					pkgMd := p.Metadata
					if pkgMd != nil {
						for _, useBlock := range pkgMd.Use {
							for _, flag := range useBlock.Flags {
								// Global
								if _, ok := globalAggUseFlags[flag.Name]; !ok {
									globalAggUseFlags[flag.Name] = &AggUseFlag{
										Name:          flag.Name,
										LocalDescs:    make(map[string]string),
										MetadataDescs: make(map[string]string),
									}
								}
								// Global logic
								foundPkgGlobal := false
								for _, pkgObj := range globalAggUseFlags[flag.Name].Packages {
									if pkgObj.Name == p.Name && pkgObj.Category == p.Category {
										foundPkgGlobal = true
										break
									}
								}
								if !foundPkgGlobal && aggPackages != nil {
									if ap, ok := aggPackages[pkgKey]; ok {
										globalAggUseFlags[flag.Name].Packages = append(globalAggUseFlags[flag.Name].Packages, ap)
										globalAggUseFlags[flag.Name].Count++
									}
								}

								if flag.Text != "" {
									globalAggUseFlags[flag.Name].MetadataDescs[pkgKey] = flag.Text
								}

								// Repo
								if _, ok := repoAggUseFlags[flag.Name]; !ok {
									repoAggUseFlags[flag.Name] = &AggUseFlag{
										Name:          flag.Name,
										LocalDescs:    make(map[string]string),
										MetadataDescs: make(map[string]string),
									}
								}

								// Repo logic
								foundPkgRepo := false
								for _, pkgObj := range repoAggUseFlags[flag.Name].Packages {
									if pkgObj.Name == p.Name && pkgObj.Category == p.Category {
										foundPkgRepo = true
										break
									}
								}
								if !foundPkgRepo && aggPackages != nil {
									if ap, ok := aggPackages[pkgKey]; ok {
										repoAggUseFlags[flag.Name].Packages = append(repoAggUseFlags[flag.Name].Packages, ap)
										repoAggUseFlags[flag.Name].Count++
									}
								}

								if flag.Text != "" {
									repoAggUseFlags[flag.Name].MetadataDescs[pkgKey] = flag.Text
								}
							}
						}
					}
				}

				for _, ver := range p.Versions {
					if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
						// Extract IUSE flags
						iuse := ver.Ebuild.Vars["IUSE"]
						if iuse != "" {
							parsedFlags := parseIUSEFlagsFunc(iuse)
							for _, flagObj := range parsedFlags {
								flag := flagObj.Name
								// Global
								if _, ok := globalAggUseFlags[flag]; !ok {
									globalAggUseFlags[flag] = &AggUseFlag{
										Name:          flag,
										LocalDescs:    make(map[string]string),
										MetadataDescs: make(map[string]string),
									}
								}
								// Repo
								if _, ok := repoAggUseFlags[flag]; !ok {
									repoAggUseFlags[flag] = &AggUseFlag{
										Name:          flag,
										LocalDescs:    make(map[string]string),
										MetadataDescs: make(map[string]string),
									}
								}

								// Global logic
								foundPkgGlobal := false
								for _, pkgObj := range globalAggUseFlags[flag].Packages {
									if pkgObj.Name == p.Name && pkgObj.Category == p.Category {
										foundPkgGlobal = true
										break
									}
								}
								if !foundPkgGlobal && aggPackages != nil {
									if ap, ok := aggPackages[pkgKey]; ok {
										globalAggUseFlags[flag].Packages = append(globalAggUseFlags[flag].Packages, ap)
										globalAggUseFlags[flag].Count++
									}
								}

								// Repo logic
								foundPkgRepo := false
								for _, pkgObj := range repoAggUseFlags[flag].Packages {
									if pkgObj.Name == p.Name && pkgObj.Category == p.Category {
										foundPkgRepo = true
										break
									}
								}
								if !foundPkgRepo && aggPackages != nil {
									if ap, ok := aggPackages[pkgKey]; ok {
										repoAggUseFlags[flag].Packages = append(repoAggUseFlags[flag].Packages, ap)
										repoAggUseFlags[flag].Count++
									}
								}
							}
						}

						// Extract REQUIRED_USE flags
						requiredUse := ver.Ebuild.Vars["REQUIRED_USE"]
						if requiredUse != "" {
							parsedFlags := parseIUSEFlagsFunc(requiredUse)
							for _, flagObj := range parsedFlags {
								flag := flagObj.Name
								if flag == "(" || flag == ")" || flag == "||" || flag == "^^" || flag == "??" || strings.HasSuffix(flag, "?") {
									continue
								}
								flag = strings.TrimPrefix(flag, "!") // remove negations

								// Global
								if _, ok := globalAggUseFlags[flag]; !ok {
									globalAggUseFlags[flag] = &AggUseFlag{
										Name:          flag,
										LocalDescs:    make(map[string]string),
										MetadataDescs: make(map[string]string),
									}
								}
								// Repo
								if _, ok := repoAggUseFlags[flag]; !ok {
									repoAggUseFlags[flag] = &AggUseFlag{
										Name:          flag,
										LocalDescs:    make(map[string]string),
										MetadataDescs: make(map[string]string),
									}
								}

								// Global
								foundPkgGlobal := false
								for _, pkgObj := range globalAggUseFlags[flag].Packages {
									if pkgObj.Name == p.Name && pkgObj.Category == p.Category {
										foundPkgGlobal = true
										break
									}
								}
								if !foundPkgGlobal && aggPackages != nil {
									if ap, ok := aggPackages[pkgKey]; ok {
										globalAggUseFlags[flag].Packages = append(globalAggUseFlags[flag].Packages, ap)
										globalAggUseFlags[flag].Count++
									}
								}

								// Repo
								foundPkgRepo := false
								for _, pkgObj := range repoAggUseFlags[flag].Packages {
									if pkgObj.Name == p.Name && pkgObj.Category == p.Category {
										foundPkgRepo = true
										break
									}
								}
								if !foundPkgRepo && aggPackages != nil {
									if ap, ok := aggPackages[pkgKey]; ok {
										repoAggUseFlags[flag].Packages = append(repoAggUseFlags[flag].Packages, ap)
										repoAggUseFlags[flag].Count++
									}
								}
							}
						}
					}
				}
			}
		}

		if site.UseDesc != nil {
			for flag, desc := range site.UseDesc.Flags {
				// Global
				if _, ok := globalAggUseFlags[flag]; !ok {
					globalAggUseFlags[flag] = &AggUseFlag{
						Name:          flag,
						LocalDescs:    make(map[string]string),
						MetadataDescs: make(map[string]string),
					}
				}
				globalAggUseFlags[flag].GlobalDesc = desc

				// Repo
				if _, ok := repoAggUseFlags[flag]; !ok {
					repoAggUseFlags[flag] = &AggUseFlag{
						Name:          flag,
						LocalDescs:    make(map[string]string),
						MetadataDescs: make(map[string]string),
					}
				}
				repoAggUseFlags[flag].GlobalDesc = desc
			}
		}

		if site.UseExpandDescs != nil {
			for prefix, desc := range site.UseExpandDescs {
				for suffix, text := range desc.Flags {
					flagName := prefix + "_" + suffix
					if aggFlag, ok := globalAggUseFlags[flagName]; ok {
						if aggFlag.GlobalDesc == "" {
							aggFlag.GlobalDesc = text
						}
					}
					if aggFlag, ok := repoAggUseFlags[flagName]; ok {
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
					// Global
					if aggFlag, ok := globalAggUseFlags[flag]; ok {
						aggFlag.LocalDescs[pkg] = desc
					}
					// Repo
					if aggFlag, ok := repoAggUseFlags[flag]; ok {
						aggFlag.LocalDescs[pkg] = desc
					}
				}
			}
		}

		var sortedRepoUseFlags []*AggUseFlag
		for _, flag := range repoAggUseFlags {
			if flag.GlobalDesc == "" && len(flag.Packages) > 0 {
				flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' is used in ebuilds/metadata but is not defined in profiles/use.desc", flag.Name))
			}
			if flag.GlobalDesc != "" && len(flag.Packages) == 0 {
				flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' is defined in use.desc but is never used by any package", flag.Name))
			}

			for _, pkg := range flag.Packages {
				pkgKey := pkg.Category + "/" + pkg.Name
				hasLocal := flag.LocalDescs[pkgKey] != ""
				hasMetadata := flag.MetadataDescs[pkgKey] != ""

				if !hasMetadata && !hasLocal && flag.GlobalDesc == "" {
					flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' used by %s but has no description in metadata.xml, use.local.desc or use.desc", flag.Name, pkgKey))
				} else if !hasMetadata && flag.GlobalDesc == "" {
					flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' used by %s but not documented in its metadata.xml", flag.Name, pkgKey))
				}
			}
			sortedRepoUseFlags = append(sortedRepoUseFlags, flag)
		}
		sort.Slice(sortedRepoUseFlags, func(i, j int) bool { return sortedRepoUseFlags[i].Name < sortedRepoUseFlags[j].Name })
		site.AggUseFlags = sortedRepoUseFlags
	}

	var sortedUseFlags []*AggUseFlag
	for _, flag := range globalAggUseFlags {
		if flag.GlobalDesc == "" && len(flag.Packages) > 0 {
			flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' is used in ebuilds/metadata but is not defined in profiles/use.desc", flag.Name))
		}
		if flag.GlobalDesc != "" && len(flag.Packages) == 0 {
			flag.Warnings = append(flag.Warnings, fmt.Sprintf("Warning: USE flag '%s' is defined in use.desc but is never used by any package", flag.Name))
		}

		for _, pkg := range flag.Packages {
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

	return sortedUseFlags, globalAggUseFlags
}

func populatePkgUseFlags(site *g2.SiteData) {
	globalDescs := make(map[string]string)
	if site.UseDesc != nil {
		globalDescs = site.UseDesc.Flags
	}

	localDescs := make(map[string]map[string]string)
	if site.UseExpandDescs != nil {
		for prefix, desc := range site.UseExpandDescs {
			for suffix, text := range desc.Flags {
				flagName := prefix + "_" + suffix
				globalDescs[flagName] = text
			}
		}
	}
	if site.UseLocalDesc != nil {
		localDescs = site.UseLocalDesc.Flags
	}

	// Pre-compute the most common local and metadata descriptions globally per flag.
	mostCommonLocal := make(map[string]string)
	mostCommonMetadata := make(map[string]string)

	localCounts := make(map[string]map[string]int)
	metadataCounts := make(map[string]map[string]int)

	if site.UseLocalDesc != nil {
		for _, flags := range site.UseLocalDesc.Flags {
			for flag, desc := range flags {
				if localCounts[flag] == nil {
					localCounts[flag] = make(map[string]int)
				}
				localCounts[flag][desc]++
			}
		}
	}

	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			pkg := &site.Categories[i].Packages[j]
			if pkg.Metadata != nil {
				for _, block := range pkg.Metadata.Use {
					for _, f := range block.Flags {
						if f.Text != "" {
							if metadataCounts[f.Name] == nil {
								metadataCounts[f.Name] = make(map[string]int)
							}
							metadataCounts[f.Name][f.Text]++
						}
					}
				}
			}
		}
	}

	for flag, descCounts := range localCounts {
		maxCount := 0
		maxDesc := ""
		for desc, count := range descCounts {
			if count > maxCount {
				maxCount = count
				maxDesc = desc
			} else if count == maxCount && desc < maxDesc { // break ties
				maxDesc = desc
			}
		}
		mostCommonLocal[flag] = maxDesc
	}

	for flag, descCounts := range metadataCounts {
		maxCount := 0
		maxDesc := ""
		for desc, count := range descCounts {
			if count > maxCount {
				maxCount = count
				maxDesc = desc
			} else if count == maxCount && desc < maxDesc { // break ties
				maxDesc = desc
			}
		}
		mostCommonMetadata[flag] = maxDesc
	}

	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			pkg := &site.Categories[i].Packages[j]
			pkgKey := pkg.Category + "/" + pkg.Name

			flagsMap := make(map[string]*g2.PkgUseFlag)

			for _, ver := range pkg.Versions {
				vName := ver.Version
				if ver.Ebuild != nil && ver.Ebuild.Vars != nil && ver.Ebuild.Vars["PV"] != "" {
					vName = ver.Ebuild.Vars["PV"]
				}

				if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					iuse := ver.Ebuild.Vars["IUSE"]
					if iuse != "" {
						parsed := parseIUSEFlagsFunc(iuse)
						for _, f := range parsed {
							if _, ok := flagsMap[f.Name]; !ok {
								flagsMap[f.Name] = &g2.PkgUseFlag{
									Name:     f.Name,
									Versions: make(map[string]string),
								}
							}

							symbol := "✓"
							switch f.ConditionStr {
							case "Default: Enabled (+)":
								symbol = "⊕"
							case "Default: Disabled (-)":
								symbol = "⊖"
							}

							flagsMap[f.Name].Versions[vName] = symbol
						}
					}
				}
			}

			var pkgFlags []g2.PkgUseFlag
			for name, flag := range flagsMap {
				desc := ""
				source := ""

				if pkg.Metadata != nil {
					for _, block := range pkg.Metadata.Use {
						for _, f := range block.Flags {
							if f.Name == name {
								desc = f.Text
								source = "metadata.xml"
								break
							}
						}
					}
				}

				if desc == "" {
					if localFlags, ok := localDescs[pkgKey]; ok {
						if ld, ok := localFlags[name]; ok {
							desc = ld
							source = "use.local.desc"
						}
					}
				}

				if desc == "" {
					if gd, ok := globalDescs[name]; ok {
						desc = gd
						source = "use.desc"
					}
				}

				if desc == "" {
					if mcl, ok := mostCommonLocal[name]; ok && mcl != "" {
						desc = mcl
						source = "most common use.local.desc"
					}
				}

				if desc == "" {
					if mcm, ok := mostCommonMetadata[name]; ok && mcm != "" {
						desc = mcm
						source = "most common metadata.xml"
					}
				}

				flag.Desc = desc
				flag.Source = source

				for _, ver := range pkg.Versions {
					vName := ver.Version
					if ver.Ebuild != nil && ver.Ebuild.Vars != nil && ver.Ebuild.Vars["PV"] != "" {
						vName = ver.Ebuild.Vars["PV"]
					}
					if _, ok := flag.Versions[vName]; !ok {
						flag.Versions[vName] = "✗"
					}
				}

				pkgFlags = append(pkgFlags, *flag)
			}

			sort.Slice(pkgFlags, func(a, b int) bool { return pkgFlags[a].Name < pkgFlags[b].Name })
			pkg.PkgUseFlags = pkgFlags
		}
	}
}
