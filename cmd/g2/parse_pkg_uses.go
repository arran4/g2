package main

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arran4/g2"
)

// isValidUseFlagName validates a Gentoo USE flag identifier per PMS section 3.1.4:
// non-empty, starts with ASCII alphanumeric, followed by [A-Za-z0-9+_@-].
func isValidUseFlagName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if i == 0 {
			if !isAlphaNum {
				return false
			}
			continue
		}
		if !isAlphaNum && c != '+' && c != '_' && c != '@' && c != '-' {
			return false
		}
	}
	return true
}

// extractRequiredUseFlags tokenizes REQUIRED_USE expressions and returns valid USE flag identifiers,
// ignoring boolean operators, parentheses, and conditional suffixes, and logging malformed tokens.
func extractRequiredUseFlags(requiredUse, repoName, pkgKey string) []string {
	cleaned := strings.NewReplacer("(", " ", ")", " ").Replace(requiredUse)
	var flags []string
	for tok := range strings.FieldsSeq(cleaned) {
		if tok == "" || tok == "||" || tok == "^^" || tok == "??" || tok == "!" || strings.HasSuffix(tok, "?") {
			continue
		}
		flag := strings.TrimPrefix(tok, "!")
		if !isValidUseFlagName(flag) {
			log.Printf("Warning: ignoring invalid USE flag %q in repo %q package %s (source: REQUIRED_USE)", tok, repoName, pkgKey)
			continue
		}
		flags = append(flags, flag)
	}
	return flags
}

// useFlagRouteSegment validates and normalizes a USE flag name into a safe single route directory segment.
func useFlagRouteSegment(name string) (string, error) {
	if !isValidUseFlagName(name) {
		return "", fmt.Errorf("invalid USE flag identifier %q", name)
	}
	safe := strings.ReplaceAll(name, "/", "_")
	if safe == "" || safe == "." || safe == ".." || strings.ContainsAny(safe, "/\\\x00") {
		return "", fmt.Errorf("unsafe USE flag route segment %q from %q", safe, name)
	}
	return safe, nil
}

// reserveUseFlagDest validates the USE flag route segment and ensures it never overwrites
// the parent uses/index.html or collides with another USE flag destination.
func reserveUseFlagDest(seen map[string]string, usesDir, flagName string) (string, error) {
	safeName, err := useFlagRouteSegment(flagName)
	if err != nil {
		return "", err
	}
	dest := filepath.Clean(filepath.Join(usesDir, safeName, "index.html"))
	norm := filepath.ToSlash(dest)
	if existing, exists := seen[norm]; exists {
		return "", fmt.Errorf("duplicate USE flag output destination %q: flag %q collides with %q", norm, flagName, existing)
	}
	seen[norm] = "use flag: " + flagName
	return dest, nil
}

// AggregateUseFlags processes a list of g2.SiteData and updates an aggregate map of UseFlags,
// and returns the sorted global UseFlags list. It also aggregates per-repo USE flags inside g2.SiteData.
func AggregateUseFlags(sites []*g2.SiteData, aggPackages map[string]*AggPackage) ([]*AggUseFlag, map[string]*AggUseFlag) {
	globalAggUseFlags := make(map[string]*AggUseFlag)
	globalSeenPkgs := make(map[string]map[string]bool)

	addGlobalPkg := func(flag string, pkgKey string) *AggUseFlag {
		aggFlag, ok := globalAggUseFlags[flag]
		if !ok {
			aggFlag = &AggUseFlag{
				Name:          flag,
				LocalDescs:    make(map[string]string),
				MetadataDescs: make(map[string]string),
			}
			globalAggUseFlags[flag] = aggFlag
			globalSeenPkgs[flag] = make(map[string]bool)
		} else if globalSeenPkgs[flag] == nil {
			globalSeenPkgs[flag] = make(map[string]bool)
		}
		if !globalSeenPkgs[flag][pkgKey] && aggPackages != nil {
			if ap, ok := aggPackages[pkgKey]; ok {
				aggFlag.Packages = append(aggFlag.Packages, ap)
				aggFlag.Count++
				globalSeenPkgs[flag][pkgKey] = true
			}
		}
		return aggFlag
	}

	for _, site := range sites {
		repoAggUseFlags := make(map[string]*AggUseFlag)
		repoSeenPkgs := make(map[string]map[string]bool)

		addRepoPkg := func(flag string, pkgKey string) *AggUseFlag {
			aggFlag, ok := repoAggUseFlags[flag]
			if !ok {
				aggFlag = &AggUseFlag{
					Name:          flag,
					LocalDescs:    make(map[string]string),
					MetadataDescs: make(map[string]string),
				}
				repoAggUseFlags[flag] = aggFlag
				repoSeenPkgs[flag] = make(map[string]bool)
			} else if repoSeenPkgs[flag] == nil {
				repoSeenPkgs[flag] = make(map[string]bool)
			}
			if !repoSeenPkgs[flag][pkgKey] && aggPackages != nil {
				if ap, ok := aggPackages[pkgKey]; ok {
					aggFlag.Packages = append(aggFlag.Packages, ap)
					aggFlag.Count++
					repoSeenPkgs[flag][pkgKey] = true
				}
			}
			return aggFlag
		}

		for _, pkg := range site.Categories {
			for _, p := range pkg.Packages {
				pkgKey := p.Category + "/" + p.Name

				if p.Metadata != nil {
					pkgMd := p.Metadata
					for _, useBlock := range pkgMd.Use {
						for _, flag := range useBlock.Flags {
							flagName := strings.TrimSpace(flag.Name)
							if !isValidUseFlagName(flagName) {
								log.Printf("Warning: ignoring invalid USE flag %q in repo %q package %s (source: metadata.xml)", flag.Name, site.RepoName, pkgKey)
								continue
							}
							gFlag := addGlobalPkg(flagName, pkgKey)
							if flag.Text != "" {
								gFlag.MetadataDescs[pkgKey] = flag.Text
							}

							rFlag := addRepoPkg(flagName, pkgKey)
							if flag.Text != "" {
								rFlag.MetadataDescs[pkgKey] = flag.Text
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
								if !isValidUseFlagName(flag) {
									continue
								}
								addGlobalPkg(flag, pkgKey)
								addRepoPkg(flag, pkgKey)
							}
						}

						// Extract REQUIRED_USE flags
						requiredUse := ver.Ebuild.Vars["REQUIRED_USE"]
						if requiredUse != "" {
							for _, flag := range extractRequiredUseFlags(requiredUse, site.RepoName, pkgKey) {
								addGlobalPkg(flag, pkgKey)
								addRepoPkg(flag, pkgKey)
							}
						}
					}
				}
			}
		}

		if site.UseDesc != nil {
			for flag, desc := range site.UseDesc.Flags {
				flagName := strings.TrimSpace(flag)
				if !isValidUseFlagName(flagName) {
					log.Printf("Warning: ignoring invalid USE flag %q in repo %q (source: profiles/use.desc)", flag, site.RepoName)
					continue
				}
				// Global
				if _, ok := globalAggUseFlags[flagName]; !ok {
					globalAggUseFlags[flagName] = &AggUseFlag{
						Name:          flagName,
						LocalDescs:    make(map[string]string),
						MetadataDescs: make(map[string]string),
					}
					globalSeenPkgs[flagName] = make(map[string]bool)
				}
				globalAggUseFlags[flagName].GlobalDesc = desc

				// Repo
				if _, ok := repoAggUseFlags[flagName]; !ok {
					repoAggUseFlags[flagName] = &AggUseFlag{
						Name:          flagName,
						LocalDescs:    make(map[string]string),
						MetadataDescs: make(map[string]string),
					}
					repoSeenPkgs[flagName] = make(map[string]bool)
				}
				repoAggUseFlags[flagName].GlobalDesc = desc
			}
		}

		if site.UseExpandDescs != nil {
			for prefix, desc := range site.UseExpandDescs {
				for suffix, text := range desc.Flags {
					flagName := prefix + "_" + suffix
					if !isValidUseFlagName(flagName) {
						log.Printf("Warning: ignoring invalid USE_EXPAND flag %q in repo %q (source: profiles/desc/%s.desc)", flagName, site.RepoName, prefix)
						continue
					}
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
					flagName := strings.TrimSpace(flag)
					if !isValidUseFlagName(flagName) {
						log.Printf("Warning: ignoring invalid USE flag %q in repo %q package %s (source: profiles/use.local.desc)", flag, site.RepoName, pkg)
						continue
					}
					// Global
					if aggFlag, ok := globalAggUseFlags[flagName]; ok {
						aggFlag.LocalDescs[pkg] = desc
					}
					// Repo
					if aggFlag, ok := repoAggUseFlags[flagName]; ok {
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
		for k, v := range site.UseDesc.Flags {
			if isValidUseFlagName(strings.TrimSpace(k)) {
				globalDescs[strings.TrimSpace(k)] = v
			}
		}
	}

	localDescs := make(map[string]map[string]string)
	if site.UseExpandDescs != nil {
		for prefix, desc := range site.UseExpandDescs {
			for suffix, text := range desc.Flags {
				flagName := prefix + "_" + suffix
				if isValidUseFlagName(flagName) {
					globalDescs[flagName] = text
				}
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
				flagName := strings.TrimSpace(flag)
				if !isValidUseFlagName(flagName) {
					continue
				}
				if localCounts[flagName] == nil {
					localCounts[flagName] = make(map[string]int)
				}
				localCounts[flagName][desc]++
			}
		}
	}

	for i := range site.Categories {
		for j := range site.Categories[i].Packages {
			pkg := &site.Categories[i].Packages[j]
			if pkg.Metadata != nil {
				for _, block := range pkg.Metadata.Use {
					for _, f := range block.Flags {
						flagName := strings.TrimSpace(f.Name)
						if f.Text != "" && isValidUseFlagName(flagName) {
							if metadataCounts[flagName] == nil {
								metadataCounts[flagName] = make(map[string]int)
							}
							metadataCounts[flagName][f.Text]++
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
