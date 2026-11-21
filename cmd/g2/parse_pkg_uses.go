package main

import (
	"github.com/arran4/g2"
	"sort"
)

// AggregateUseFlags processes a list of g2.SiteData and updates an aggregate map of UseFlags,
// and returns the sorted global UseFlags list. It also aggregates per-repo USE flags inside g2.SiteData.
func AggregateUseFlags(sites []*g2.SiteData, aggPackages map[string]*AggPackage) ([]*AggUseFlag, map[string]*AggUseFlag) {
	for _, site := range sites {
		if site == nil {
			continue
		}
		repoFlags, _ := aggregateUseFlagsWithProvenance([]*g2.SiteData{site}, aggPackages)
		site.AggUseFlags = repoFlags
	}
	return aggregateUseFlagsWithProvenance(sites, aggPackages)
}

func populatePkgUseFlags(site *g2.SiteData) {
	globalDescs := make(map[string]string)
	if site.UseDesc != nil {
		for k, v := range site.UseDesc.Flags {
			globalDescs[k] = v
		}
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
