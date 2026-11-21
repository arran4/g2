package main

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/arran4/g2"
)

// UseFlagSourceKind identifies the authoritative source file/construct where a USE flag or description was observed.
type UseFlagSourceKind string

const (
	SourceUseDesc      UseFlagSourceKind = "use.desc"
	SourceUseExpand    UseFlagSourceKind = "use_expand"
	SourceUseLocalDesc UseFlagSourceKind = "use.local.desc"
	SourceMetadataXML  UseFlagSourceKind = "metadata.xml"
	SourceIUSE         UseFlagSourceKind = "IUSE"
	SourceRequiredUse  UseFlagSourceKind = "REQUIRED_USE"
)

// UseFlagScope represents the authoritative Gentoo namespace/provenance scope of a USE flag.
type UseFlagScope string

const (
	ScopeGlobalDefined UseFlagScope = "global_defined"
	ScopeUseExpand     UseFlagScope = "use_expand"
	ScopePackageScoped UseFlagScope = "package_scoped"
	ScopeUndocumented  UseFlagScope = "undocumented"
)

// UseFlagDefinition preserves a single repository-attributed description record without overwriting across overlays.
type UseFlagDefinition struct {
	RepoName    string
	Category    string
	Package     string
	SourceKind  UseFlagSourceKind
	SourceFile  string
	Description string
}

// UseFlagPackageUsage captures per-(repository, package) usage and descriptions for a flag.
type UseFlagPackageUsage struct {
	RepoName     string
	Category     string
	Package      string
	DeclaredIUSE bool
	RequiredUse  bool
	Versions     []string
	LocalDesc    string
	MetadataDesc string
}

// ProvenanceUseFlagGroups groups USE flags strictly by authoritative metadata provenance rather than package frequency.
type ProvenanceUseFlagGroups struct {
	GlobalDefined    []ProvenanceUseFlagItem
	UseExpandDefined []ProvenanceUseFlagItem
	PackageScoped    []ProvenanceUseFlagItem
	Undocumented     []ProvenanceUseFlagItem
}

// ProvenanceUseFlagItem represents a single row in provenance-grouped USE listings.
type ProvenanceUseFlagItem struct {
	Name             string
	Desc             string
	Scope            UseFlagScope
	DistinctPackages int
	PackageInstances int
	HasConflict      bool
}

func isValidSiteUseFlagName(name string) bool {
	return isValidUseFlagName(name)
}

func aggregateUseFlagsWithProvenance(sites []*g2.SiteData, aggPackages map[string]*AggPackage) ([]*AggUseFlag, map[string]*AggUseFlag) {
	sortedSites := make([]*g2.SiteData, 0, len(sites))
	for _, s := range sites {
		if s != nil {
			sortedSites = append(sortedSites, s)
		}
	}
	sort.Slice(sortedSites, func(i, j int) bool {
		return sortedSites[i].RepoName < sortedSites[j].RepoName
	})

	flagMap := make(map[string]*AggUseFlag)
	seenDistinctPkgs := make(map[string]map[string]bool)
	type usageKey struct {
		repo string
		pkg  string
	}
	usageMaps := make(map[string]map[usageKey]*UseFlagPackageUsage)

	ensureFlag := func(flag string) *AggUseFlag {
		f, ok := flagMap[flag]
		if !ok {
			f = &AggUseFlag{
				Name:          flag,
				LocalDescs:    make(map[string]string),
				MetadataDescs: make(map[string]string),
			}
			flagMap[flag] = f
			seenDistinctPkgs[flag] = make(map[string]bool)
			usageMaps[flag] = make(map[usageKey]*UseFlagPackageUsage)
		}
		return f
	}

	ensureUsage := func(flag, repoName, cat, pkgName string) *UseFlagPackageUsage {
		f := ensureFlag(flag)
		pkgKey := cat + "/" + pkgName
		if !seenDistinctPkgs[flag][pkgKey] && aggPackages != nil {
			if ap, ok := aggPackages[pkgKey]; ok {
				f.Packages = append(f.Packages, ap)
				f.DistinctPackageCount++
				f.Count = f.DistinctPackageCount
				seenDistinctPkgs[flag][pkgKey] = true
			}
		} else if !seenDistinctPkgs[flag][pkgKey] && aggPackages == nil {
			f.DistinctPackageCount++
			f.Count = f.DistinctPackageCount
			seenDistinctPkgs[flag][pkgKey] = true
		}

		uk := usageKey{repo: repoName, pkg: pkgKey}
		u, ok := usageMaps[flag][uk]
		if !ok {
			u = &UseFlagPackageUsage{
				RepoName: repoName,
				Category: cat,
				Package:  pkgName,
			}
			usageMaps[flag][uk] = u
		}
		return u
	}

	for _, site := range sortedSites {
		repoName := site.RepoName
		for _, cat := range site.Categories {
			for _, p := range cat.Packages {
				pkgKey := p.Category + "/" + p.Name
				if p.Metadata != nil {
					for _, useBlock := range p.Metadata.Use {
						for _, flag := range useBlock.Flags {
							flagName := strings.TrimSpace(flag.Name)
							if !isValidSiteUseFlagName(flagName) {
								log.Printf("Warning: ignoring invalid USE flag %q in repo %q package %s (source: metadata.xml)", flag.Name, repoName, pkgKey)
								continue
							}
							u := ensureUsage(flagName, repoName, p.Category, p.Name)
							if flag.Text != "" {
								u.MetadataDesc = flag.Text
								f := flagMap[flagName]
								if f.MetadataDescs[pkgKey] == "" {
									f.MetadataDescs[pkgKey] = flag.Text
								}
								f.LocalDefs = append(f.LocalDefs, UseFlagDefinition{
									RepoName:    repoName,
									Category:    p.Category,
									Package:     p.Name,
									SourceKind:  SourceMetadataXML,
									SourceFile:  pkgKey + "/metadata.xml",
									Description: flag.Text,
								})
							}
						}
					}
				}

				for _, ver := range p.Versions {
					if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
						continue
					}
					vName := ver.Version
					if pv := ver.Ebuild.Vars["PV"]; pv != "" {
						vName = pv
					}
					if iuse := ver.Ebuild.Vars["IUSE"]; iuse != "" {
						for _, flagObj := range parseIUSEFlagsFunc(iuse) {
							if !isValidSiteUseFlagName(flagObj.Name) {
								continue
							}
							u := ensureUsage(flagObj.Name, repoName, p.Category, p.Name)
							u.DeclaredIUSE = true
							if vName != "" && !containsStrUse(u.Versions, vName) {
								u.Versions = append(u.Versions, vName)
							}
						}
					}
					if req := ver.Ebuild.Vars["REQUIRED_USE"]; req != "" {
						for _, flag := range extractRequiredUseFlagsFromExpr(req, repoName, pkgKey, true) {
							u := ensureUsage(flag, repoName, p.Category, p.Name)
							u.RequiredUse = true
						}
					}
				}
			}
		}

		if site.UseDesc != nil {
			flagKeys := make([]string, 0, len(site.UseDesc.Flags))
			for k := range site.UseDesc.Flags {
				flagKeys = append(flagKeys, k)
			}
			sort.Strings(flagKeys)
			for _, flag := range flagKeys {
				flagName := strings.TrimSpace(flag)
				if !isValidSiteUseFlagName(flagName) {
					log.Printf("Warning: ignoring invalid USE flag %q in repo %q (source: profiles/use.desc)", flag, repoName)
					continue
				}
				desc := site.UseDesc.Flags[flag]
				f := ensureFlag(flagName)
				f.GlobalDefs = append(f.GlobalDefs, UseFlagDefinition{
					RepoName:    repoName,
					SourceKind:  SourceUseDesc,
					SourceFile:  "profiles/use.desc",
					Description: desc,
				})
				if f.GlobalDesc == "" && desc != "" {
					f.GlobalDesc = desc
				}
			}
		}

		if site.UseExpandDescs != nil {
			prefixes := make([]string, 0, len(site.UseExpandDescs))
			for p := range site.UseExpandDescs {
				prefixes = append(prefixes, p)
			}
			sort.Strings(prefixes)
			for _, prefix := range prefixes {
				desc := site.UseExpandDescs[prefix]
				if desc == nil {
					continue
				}
				suffixes := make([]string, 0, len(desc.Flags))
				for s := range desc.Flags {
					suffixes = append(suffixes, s)
				}
				sort.Strings(suffixes)
				for _, suffix := range suffixes {
					flagName := prefix + "_" + suffix
					if !isValidSiteUseFlagName(flagName) {
						log.Printf("Warning: ignoring invalid USE_EXPAND flag %q in repo %q (source: profiles/desc/%s.desc)", flagName, repoName, prefix)
						continue
					}
					text := desc.Flags[suffix]
					f := ensureFlag(flagName)
					f.GlobalDefs = append(f.GlobalDefs, UseFlagDefinition{
						RepoName:    repoName,
						SourceKind:  SourceUseExpand,
						SourceFile:  "profiles/desc/" + prefix + ".desc",
						Description: text,
					})
					if f.GlobalDesc == "" && text != "" {
						f.GlobalDesc = text
					}
				}
			}
		}

		if site.UseLocalDesc != nil {
			pkgKeys := make([]string, 0, len(site.UseLocalDesc.Flags))
			for pk := range site.UseLocalDesc.Flags {
				pkgKeys = append(pkgKeys, pk)
			}
			sort.Strings(pkgKeys)
			for _, pkgKey := range pkgKeys {
				flags := site.UseLocalDesc.Flags[pkgKey]
				flagNames := make([]string, 0, len(flags))
				for fn := range flags {
					flagNames = append(flagNames, fn)
				}
				sort.Strings(flagNames)
				cat, pkgName := splitCatPkg(pkgKey)
				for _, flagName := range flagNames {
					trimmedFlag := strings.TrimSpace(flagName)
					if !isValidSiteUseFlagName(trimmedFlag) {
						log.Printf("Warning: ignoring invalid USE flag %q in repo %q package %s (source: profiles/use.local.desc)", flagName, repoName, pkgKey)
						continue
					}
					desc := flags[flagName]
					f := ensureFlag(trimmedFlag)
					if f.LocalDescs[pkgKey] == "" && desc != "" {
						f.LocalDescs[pkgKey] = desc
					}
					f.LocalDefs = append(f.LocalDefs, UseFlagDefinition{
						RepoName:    repoName,
						Category:    cat,
						Package:     pkgName,
						SourceKind:  SourceUseLocalDesc,
						SourceFile:  "profiles/use.local.desc",
						Description: desc,
					})
					if u, ok := usageMaps[trimmedFlag][usageKey{repo: repoName, pkg: pkgKey}]; ok && u.LocalDesc == "" {
						u.LocalDesc = desc
					}
				}
			}
		}
	}

	sortedFlags := make([]*AggUseFlag, 0, len(flagMap))
	for flagName, f := range flagMap {
		sort.Slice(f.Packages, func(i, j int) bool {
			return f.Packages[i].Category+"/"+f.Packages[i].Name < f.Packages[j].Category+"/"+f.Packages[j].Name
		})
		sort.Slice(f.GlobalDefs, func(i, j int) bool {
			if f.GlobalDefs[i].RepoName != f.GlobalDefs[j].RepoName {
				return f.GlobalDefs[i].RepoName < f.GlobalDefs[j].RepoName
			}
			return f.GlobalDefs[i].SourceFile < f.GlobalDefs[j].SourceFile
		})
		sort.Slice(f.LocalDefs, func(i, j int) bool {
			if f.LocalDefs[i].RepoName != f.LocalDefs[j].RepoName {
				return f.LocalDefs[i].RepoName < f.LocalDefs[j].RepoName
			}
			ki := f.LocalDefs[i].Category + "/" + f.LocalDefs[i].Package
			kj := f.LocalDefs[j].Category + "/" + f.LocalDefs[j].Package
			if ki != kj {
				return ki < kj
			}
			return f.LocalDefs[i].SourceKind < f.LocalDefs[j].SourceKind
		})

		um := usageMaps[flagName]
		usages := make([]UseFlagPackageUsage, 0, len(um))
		for _, u := range um {
			sort.Strings(u.Versions)
			usages = append(usages, *u)
		}
		sort.Slice(usages, func(i, j int) bool {
			if usages[i].RepoName != usages[j].RepoName {
				return usages[i].RepoName < usages[j].RepoName
			}
			return usages[i].Category+"/"+usages[i].Package < usages[j].Category+"/"+usages[j].Package
		})
		f.Usages = usages
		f.PackageInstanceCount = len(usages)

		hasUseDesc := false
		hasUseExpand := false
		distinctGlobalDescs := make(map[string]bool)
		for _, gd := range f.GlobalDefs {
			switch gd.SourceKind {
			case SourceUseDesc:
				hasUseDesc = true
			case SourceUseExpand:
				hasUseExpand = true
			}
			if gd.Description != "" {
				distinctGlobalDescs[gd.Description] = true
			}
		}
		f.HasConflictingGlobalDescs = len(distinctGlobalDescs) > 1

		switch {
		case hasUseDesc:
			f.Scope = ScopeGlobalDefined
		case hasUseExpand:
			f.Scope = ScopeUseExpand
		case len(f.LocalDefs) > 0:
			f.Scope = ScopePackageScoped
		default:
			f.Scope = ScopeUndocumented
		}

		for _, u := range usages {
			pkgKey := u.Category + "/" + u.Package
			if u.MetadataDesc == "" && u.LocalDesc == "" && f.GlobalDesc == "" {
				f.Warnings = append(f.Warnings, fmt.Sprintf("Warning: USE flag '%s' used by %s (%s) has no description in metadata.xml, use.local.desc or use.desc", f.Name, pkgKey, u.RepoName))
			} else if u.MetadataDesc == "" && f.GlobalDesc == "" {
				f.Warnings = append(f.Warnings, fmt.Sprintf("Warning: USE flag '%s' used by %s (%s) is not documented in its metadata.xml", f.Name, pkgKey, u.RepoName))
			}
		}

		sortedFlags = append(sortedFlags, f)
	}

	sort.Slice(sortedFlags, func(i, j int) bool {
		return sortedFlags[i].Name < sortedFlags[j].Name
	})
	return sortedFlags, flagMap
}

func splitCatPkg(key string) (string, string) {
	if idx := strings.IndexByte(key, '/'); idx != -1 {
		return key[:idx], key[idx+1:]
	}
	return "", key
}

func containsStrUse(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func groupUseFlagsByProvenanceFunc(flags []*AggUseFlag) ProvenanceUseFlagGroups {
	var groups ProvenanceUseFlagGroups
	for _, f := range flags {
		if f == nil || !isValidSiteUseFlagName(f.Name) {
			continue
		}
		item := ProvenanceUseFlagItem{
			Name:             f.Name,
			Desc:             getBestDescFunc(f, ""),
			Scope:            f.Scope,
			DistinctPackages: f.DistinctPackageCount,
			PackageInstances: f.PackageInstanceCount,
			HasConflict:      f.HasConflictingGlobalDescs,
		}
		if item.DistinctPackages == 0 && f.Count > 0 {
			item.DistinctPackages = f.Count
		}
		switch f.Scope {
		case ScopeGlobalDefined:
			groups.GlobalDefined = append(groups.GlobalDefined, item)
		case ScopeUseExpand:
			groups.UseExpandDefined = append(groups.UseExpandDefined, item)
		case ScopePackageScoped:
			groups.PackageScoped = append(groups.PackageScoped, item)
		default:
			groups.Undocumented = append(groups.Undocumented, item)
		}
	}
	return groups
}
