package main

import (
	"sort"
)

func populatePkgUseFlags(site *SiteData) {
	globalDescs := make(map[string]string)
	if site.UseDesc != nil {
		globalDescs = site.UseDesc.Flags
	}

	localDescs := make(map[string]map[string]string)
	if site.UseLocalDesc != nil {
		localDescs = site.UseLocalDesc.Flags
	}

	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			pkg := &site.Categories[i].Packages[j]
			pkgKey := pkg.Category + "/" + pkg.Name

			flagsMap := make(map[string]*PkgUseFlag)

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
								flagsMap[f.Name] = &PkgUseFlag{
									Name: f.Name,
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

			var pkgFlags []PkgUseFlag
			for name, flag := range flagsMap {
				desc := ""
				if pkg.Metadata != nil {
					for _, block := range pkg.Metadata.Use {
						for _, f := range block.Flags {
							if f.Name == name {
								desc = f.Text
								break
							}
						}
					}
				}

				if desc == "" {
					if localFlags, ok := localDescs[pkgKey]; ok {
						if ld, ok := localFlags[name]; ok {
							desc = ld
						}
					}
				}

				if desc == "" {
					if gd, ok := globalDescs[name]; ok {
						desc = gd
					}
				}

				flag.Desc = desc

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
