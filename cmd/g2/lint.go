package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"

	_ "github.com/arran4/g2/lints/ebuild"
	_ "github.com/arran4/g2/lints/eclass"
	_ "github.com/arran4/g2/lints/md5cache"
	_ "github.com/arran4/g2/lints/metadata"
	_ "github.com/arran4/g2/lints/news"
)

func (cfg *MainArgConfig) cmdLint(args []string) error {
	if len(args) > 0 && args[0] == "list" {
		return cfg.cmdLintList(args[1:])
	}

	if len(args) > 0 {
		subcmd := args[0]
		switch subcmd {
		case "repo":
			return cfg.cmdLintRepo(args[1:])
		case "package":
			return cfg.cmdLintPackage(args[1:])
		case "query":
			return cfg.cmdLintQuery(args[1:])
		}
	}

	// Fallback to backward-compatible lint behavior
	return cfg.runOldLint(args)
}

func (cfg *MainArgConfig) cmdLintList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	format := fs.String("format", "text", "Output format: text, json")

	if err := fs.Parse(args); err != nil {
		return err
	}

	rules := lints.GetAllRules()
	if *format == "json" {
		out, _ := json.MarshalIndent(rules, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	fmt.Println("Available Lint Rules:")
	for _, r := range rules {
		fmt.Printf("- %s [%s] (%s): %s\n", r.ID, r.Severity, r.Source, r.Description)
		if r.URL != "" {
			fmt.Printf("  URL: %s\n", r.URL)
		}
	}
	return nil
}

func (cfg *MainArgConfig) runOldLint(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint [flags] [<location>] [<target_package>...]\n\n")
		fmt.Printf("  <location>\tOptional path to the overlay directory (defaults to '.'). Detected automatically if it's a valid repo.\n")
		fmt.Printf("  <target_package>\tOptional specific packages or categories to lint instead of the entire repository (e.g. app-misc/foo or just foo).\n\n")
		fmt.Printf("Flags:\n")
		fs.PrintDefaults()
	}
	format := fs.String("format", "text", "Output format: text, json, or github-actions")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")
	disableRule := fs.String("disable-rule", "", "Comma-separated list of rule IDs to ignore (case-insensitive)")
	ignoreTag := fs.String("ignore-tag", "", "Comma-separated list of tags to ignore")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	var targetPkgs []string

	if fs.NArg() > 0 {
		firstArg := fs.Arg(0)
		isRepo := false
		if _, err := os.Stat(filepath.Join(firstArg, "profiles")); err == nil {
			isRepo = true
		} else if _, err := os.Stat(filepath.Join(firstArg, "metadata", "layout.conf")); err == nil {
			isRepo = true
		} else if firstArg == "." {
			isRepo = true
		}

		if isRepo {
			location = firstArg
			targetPkgs = fs.Args()[1:]
		} else {
			targetPkgs = fs.Args()
		}
	}

	siteData, err := parseRepo(os.DirFS(location), ".", "Linting", true, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	var targetMap map[string]bool
	if len(targetPkgs) > 0 {
		targetMap = make(map[string]bool)
		for _, p := range targetPkgs {
			cleanP := filepath.ToSlash(filepath.Clean(p))
			targetMap[cleanP] = true
		}
	}

	hasErrors := false

	var allResults []lints.LintResult

	for _, cat := range siteData.Categories {
		for _, pkg := range cat.Packages {
			if len(targetMap) > 0 {
				qualified := pkg.Category + "/" + pkg.Name
				if !targetMap[qualified] && !targetMap[pkg.Name] && !targetMap[pkg.Category] {
					continue
				}
			}
			pkgCopy := g2.PackageData{
				Name:          pkg.Name,
				Category:      pkg.Category,
				Metadata:      pkg.Metadata,
				MetadataError: pkg.MetadataError,
				Manifest:      pkg.Manifest,
			}
			for _, v := range pkg.Versions {
				pkgCopy.Versions = append(pkgCopy.Versions, g2.VersionData{
					Version:      v.Version,
					Ebuild:       v.Ebuild,
					EbuildRawURL: v.EbuildRawURL,
				})
			}

			lintWarnings := lints.PerformLintingResults(location, &pkgCopy)

			// Filter warnings
			var filteredWarnings []lints.LintResult
			for _, w := range lintWarnings {
				if *severityFilter != "" && !strings.EqualFold(string(w.RuleMetadata.Severity), *severityFilter) {
					continue
				}
				if *sourceFilter != "" && string(w.RuleMetadata.Source) != *sourceFilter {
					continue
				}
				if *tagFilter != "" {
					hasTag := false
					for _, t := range w.RuleMetadata.Tags {
						if t == *tagFilter {
							hasTag = true
							break
						}
					}
					if !hasTag {
						continue
					}
				}
				isIgnored := false
				if *disableRule != "" {
					disabled := strings.Split(*disableRule, ",")
					for _, dr := range disabled {
						if strings.EqualFold(string(w.RuleMetadata.ID), strings.TrimSpace(dr)) {
							isIgnored = true
							break
						}
					}
				}
				if !isIgnored && *ignoreTag != "" {
					ignoredTags := strings.Split(*ignoreTag, ",")
					for _, it := range ignoredTags {
						it = strings.TrimSpace(it)
						for _, t := range w.RuleMetadata.Tags {
							if strings.EqualFold(t, it) {
								isIgnored = true
								break
							}
						}
						if isIgnored {
							break
						}
					}
				}
				if isIgnored {
					continue
				}

				filteredWarnings = append(filteredWarnings, w)
			}

			for i := range filteredWarnings {
				if filteredWarnings[i].Package == "" {
					filteredWarnings[i].Package = pkg.Category + "/" + pkg.Name
				}
			}

			if len(filteredWarnings) > 0 {
				hasErrors = true
				if *format == "text" {
					packageGroups := make(map[string][]lints.LintResult)
					for _, w := range filteredWarnings {
						packageGroups[w.Package] = append(packageGroups[w.Package], w)
					}

					var pkgNames []string
					for k := range packageGroups {
						pkgNames = append(pkgNames, k)
					}
					sort.Strings(pkgNames)

					for _, pkgName := range pkgNames {
						warnings := packageGroups[pkgName]
						fmt.Printf("[%s]\n", pkgName)
						for _, w := range warnings {
							fmt.Printf("  - %s\n", w.Message)
						}
					}
				}
				allResults = append(allResults, filteredWarnings...)
			}
		}
	}

	switch *format {
	case "json":
		out, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting json: %w", err)
		}
		fmt.Println(string(out))
	case "github-actions":
		printGithubActions(allResults)
	}

	if hasErrors {
		return fmt.Errorf("linting found errors")
	}

	if *format == "text" {
		fmt.Println("Linting passed successfully.")
	}
	return nil
}

func printGithubActions(results []lints.LintResult) {
	for _, res := range results {
		level := "error"
		switch res.RuleMetadata.Severity {
		case lints.SeverityWarning:
			level = "warning"
		case lints.SeverityNotice, lints.SeverityInfo:
			level = "notice" // GitHub actions doesn't have info, map to notice
		}

		msg := escapeGithubActions(res.Message)
		title := escapeGithubProperty(res.RuleMetadata.Title)
		file := res.File
		if file == "" {
			file = res.Package
		}

		var props []string
		if file != "" {
			props = append(props, fmt.Sprintf("file=%s", escapeGithubProperty(file)))
		}
		if res.Line > 0 {
			props = append(props, fmt.Sprintf("line=%d", res.Line))
		}
		if title != "" {
			props = append(props, fmt.Sprintf("title=%s", title)) // don't strictly need to escape if we know title format, but let's be safe
		}

		propStr := ""
		if len(props) > 0 {
			propStr = " " + strings.Join(props, ",")
		}

		fmt.Printf("::%s%s::%s\n", level, propStr, msg)
	}
}

func escapeGithubActions(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

func escapeGithubProperty(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}

type LintQuery struct {
	RepoPath  string
	Category  string
	Package   string
	VersionOp string // e.g. ">=", "<", "~", "="
	Version   string // e.g. "1.2.3"
	VWildcard string // e.g. "3." for "v3"
}

func (cfg *MainArgConfig) runLintCore(location string, targetMap map[string]bool, query *LintQuery, format, severityFilter, sourceFilter, tagFilter, disableRule, ignoreTag string) error {
	siteData, err := parseRepo(os.DirFS(location), ".", "Linting", true, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	hasErrors := false
	var allResults []lints.LintResult

	for _, eclass := range siteData.Eclasses {
		if len(targetMap) > 0 {
			eclassName := filepath.Base(eclass.Path)
			if !targetMap[eclassName] && !targetMap[eclass.Path] && !targetMap["eclass"] {
				continue
			}
		}

		eclassWarnings := lints.PerformEclassLintingResults(location, eclass)

		var filteredEclassWarnings []lints.LintResult
		for _, w := range eclassWarnings {
			if severityFilter != "" && !strings.EqualFold(string(w.RuleMetadata.Severity), severityFilter) {
				continue
			}
			if sourceFilter != "" && string(w.RuleMetadata.Source) != sourceFilter {
				continue
			}
			if tagFilter != "" {
				hasTag := false
				for _, t := range w.RuleMetadata.Tags {
					if t == tagFilter {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
			}
			isIgnored := false
			if disableRule != "" {
				disabled := strings.Split(disableRule, ",")
				for _, dr := range disabled {
					if strings.EqualFold(string(w.RuleMetadata.ID), strings.TrimSpace(dr)) {
						isIgnored = true
						break
					}
				}
			}
			if !isIgnored && ignoreTag != "" {
				ignoredTags := strings.Split(ignoreTag, ",")
				for _, it := range ignoredTags {
					it = strings.TrimSpace(it)
					for _, t := range w.RuleMetadata.Tags {
						if strings.EqualFold(t, it) {
							isIgnored = true
							break
						}
					}
					if isIgnored {
						break
					}
				}
			}
			if isIgnored {
				continue
			}

			filteredEclassWarnings = append(filteredEclassWarnings, w)
		}

		for i := range filteredEclassWarnings {
			if filteredEclassWarnings[i].Package == "" {
				filteredEclassWarnings[i].Package = filepath.Base(eclass.Path)
			}
		}

		if len(filteredEclassWarnings) > 0 {
			hasErrors = true
			if format == "text" {
				packageGroups := make(map[string][]lints.LintResult)
				for _, w := range filteredEclassWarnings {
					packageGroups[w.Package] = append(packageGroups[w.Package], w)
				}

				var pkgNames []string
				for k := range packageGroups {
					pkgNames = append(pkgNames, k)
				}
				sort.Strings(pkgNames)

				for _, pkgName := range pkgNames {
					warnings := packageGroups[pkgName]
					fmt.Printf("[%s]\n", pkgName)
					for _, w := range warnings {
						fmt.Printf("  - %s\n", w.Message)
					}
				}
			}
			allResults = append(allResults, filteredEclassWarnings...)
		}
	}

	for _, cat := range siteData.Categories {
		if query != nil && query.Category != "" && query.Category != cat.Name {
			continue
		}
		for _, pkg := range cat.Packages {
			if query != nil && query.Package != "" && query.Package != pkg.Name {
				continue
			}

			if len(targetMap) > 0 {
				qualified := pkg.Category + "/" + pkg.Name
				if !targetMap[qualified] && !targetMap[pkg.Name] && !targetMap[pkg.Category] {
					continue
				}
			}

			pkgCopy := g2.PackageData{
				Name:          pkg.Name,
				Category:      pkg.Category,
				Metadata:      pkg.Metadata,
				MetadataError: pkg.MetadataError,
				Manifest:      pkg.Manifest,
			}
			for _, v := range pkg.Versions {
				// Filter versions if query is present
				if query != nil {
					if query.VWildcard != "" {
						// Match if the version exactly equals the wildcard (e.g. "3" matches "3")
						// OR if it starts with the wildcard followed by a dot or hyphen (e.g. "3" matches "3.1" or "3-r1")
						if v.Version != query.VWildcard && !strings.HasPrefix(v.Version, query.VWildcard+".") && !strings.HasPrefix(v.Version, query.VWildcard+"-") {
							continue
						}
					}
					if query.VersionOp != "" && query.Version != "" {
						vPadded := g2.PadVersion(v.Version)
						qPadded := g2.PadVersion(query.Version)

						match := false
						switch query.VersionOp {
						case "==":
							match = vPadded == qPadded
						case ">":
							match = vPadded > qPadded
						case "<":
							match = vPadded < qPadded
						case ">=":
							match = vPadded >= qPadded
						case "<=":
							match = vPadded <= qPadded
						case "~":
							// Roughly matches prefix for versions
							match = strings.HasPrefix(v.Version, query.Version)
						case "=":
							match = vPadded == qPadded
						}

						if !match {
							continue
						}
					}
				}

				pkgCopy.Versions = append(pkgCopy.Versions, g2.VersionData{
					Version:      v.Version,
					Ebuild:       v.Ebuild,
					EbuildRawURL: v.EbuildRawURL,
				})
			}

			// If all versions were filtered out, skip linting this package
			if len(pkg.Versions) > 0 && len(pkgCopy.Versions) == 0 {
				continue
			}

			lintWarnings := lints.PerformLintingResults(location, &pkgCopy)

			var filteredWarnings []lints.LintResult
			for _, w := range lintWarnings {
				if severityFilter != "" && !strings.EqualFold(string(w.RuleMetadata.Severity), severityFilter) {
					continue
				}
				if sourceFilter != "" && string(w.RuleMetadata.Source) != sourceFilter {
					continue
				}
				if tagFilter != "" {
					hasTag := false
					for _, t := range w.RuleMetadata.Tags {
						if t == tagFilter {
							hasTag = true
							break
						}
					}
					if !hasTag {
						continue
					}
				}
				isIgnored := false
				if disableRule != "" {
					disabled := strings.Split(disableRule, ",")
					for _, dr := range disabled {
						if strings.EqualFold(string(w.RuleMetadata.ID), strings.TrimSpace(dr)) {
							isIgnored = true
							break
						}
					}
				}
				if !isIgnored && ignoreTag != "" {
					ignoredTags := strings.Split(ignoreTag, ",")
					for _, it := range ignoredTags {
						it = strings.TrimSpace(it)
						for _, t := range w.RuleMetadata.Tags {
							if strings.EqualFold(t, it) {
								isIgnored = true
								break
							}
						}
						if isIgnored {
							break
						}
					}
				}
				if isIgnored {
					continue
				}

				filteredWarnings = append(filteredWarnings, w)
			}

			for i := range filteredWarnings {
				if filteredWarnings[i].Package == "" {
					filteredWarnings[i].Package = pkg.Category + "/" + pkg.Name
				}
			}

			if len(filteredWarnings) > 0 {
				hasErrors = true
				if format == "text" {
					packageGroups := make(map[string][]lints.LintResult)
					for _, w := range filteredWarnings {
						packageGroups[w.Package] = append(packageGroups[w.Package], w)
					}
					for pkgName, warnings := range packageGroups {
						fmt.Printf("[%s]\n", pkgName)
						for _, w := range warnings {
							fmt.Printf("  - %s\n", w.Message)
						}
					}
				}
				allResults = append(allResults, filteredWarnings...)
			}
		}
	}

	switch format {
	case "json":
		out, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting json: %w", err)
		}
		fmt.Println(string(out))
	case "github-actions":
		printGithubActions(allResults)
	}

	if hasErrors {
		return fmt.Errorf("linting found errors")
	}

	if format == "text" {
		fmt.Println("Linting passed successfully.")
	}
	return nil
}

func (cfg *MainArgConfig) cmdLintRepo(args []string) error {
	fs := flag.NewFlagSet("repo", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint repo [flags] [<location>]\n\n")
		fmt.Printf("  <location>\tOptional path to the overlay directory (defaults to '.').\n\n")
		fmt.Printf("Flags:\n")
		fs.PrintDefaults()
	}
	format := fs.String("format", "text", "Output format: text, json, or github-actions")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")
	disableRule := fs.String("disable-rule", "", "Comma-separated list of rule IDs to ignore (case-insensitive)")
	ignoreTag := fs.String("ignore-tag", "", "Comma-separated list of tags to ignore")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	return cfg.runLintCore(location, nil, nil, *format, *severityFilter, *sourceFilter, *tagFilter, *disableRule, *ignoreTag)
}

func (cfg *MainArgConfig) cmdLintPackage(args []string) error {
	fs := flag.NewFlagSet("package", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint package [flags] [<location>] <target_package>...\n\n")
		fmt.Printf("  <location>\tOptional path to the overlay directory (defaults to '.'). Detected automatically if it's a valid repo.\n")
		fmt.Printf("  <target_package>\tSpecific packages or categories to lint (e.g. app-misc/foo or just foo).\n\n")
		fmt.Printf("Flags:\n")
		fs.PrintDefaults()
	}
	format := fs.String("format", "text", "Output format: text, json, or github-actions")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")
	disableRule := fs.String("disable-rule", "", "Comma-separated list of rule IDs to ignore (case-insensitive)")
	ignoreTag := fs.String("ignore-tag", "", "Comma-separated list of tags to ignore")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	var targetPkgs []string

	if fs.NArg() > 0 {
		firstArg := fs.Arg(0)
		isRepo := false
		if _, err := os.Stat(filepath.Join(firstArg, "profiles")); err == nil {
			isRepo = true
		} else if _, err := os.Stat(filepath.Join(firstArg, "metadata", "layout.conf")); err == nil {
			isRepo = true
		} else if firstArg == "." {
			isRepo = true
		}

		if isRepo {
			location = firstArg
			targetPkgs = fs.Args()[1:]
		} else {
			targetPkgs = fs.Args()
		}
	}

	if len(targetPkgs) == 0 {
		return fmt.Errorf("usage: g2 lint package [flags] [<location>] <target_package>")
	}

	targetMap := make(map[string]bool)
	for _, p := range targetPkgs {
		cleanP := filepath.ToSlash(filepath.Clean(p))
		targetMap[cleanP] = true
	}

	return cfg.runLintCore(location, targetMap, nil, *format, *severityFilter, *sourceFilter, *tagFilter, *disableRule, *ignoreTag)
}

func parseLintQuery(queryStr string, defaultLocation string) (*LintQuery, error) {
	q := &LintQuery{RepoPath: defaultLocation}

	// Handle repo suffix (::repo)
	if idx := strings.LastIndex(queryStr, "::"); idx != -1 {
		repoName := queryStr[idx+2:]
		queryStr = queryStr[:idx]

		rc, err := g2.ParseReposConf("/etc/portage/repos.conf")
		if err == nil {
			found := false
			for _, f := range rc.Files {
				for _, s := range f.Sections {
					if s.Name == repoName {
						for _, line := range s.Lines {
							if strings.HasPrefix(strings.TrimSpace(line), "location") {
								parts := strings.SplitN(line, "=", 2)
								if len(parts) == 2 {
									q.RepoPath = strings.TrimSpace(parts[1])
									found = true
									break
								}
							}
						}
					}
					if found {
						break
					}
				}
				if found {
					break
				}
			}
			if !found && q.RepoPath == defaultLocation {
				// Fallback if not found but we know they asked for a repo
				q.RepoPath = "/var/db/repos/" + repoName
			}
		} else {
			q.RepoPath = "/var/db/repos/" + repoName
		}
	}

	// Handle operators
	ops := []string{">=", "<=", ">", "<", "~", "="}
	for _, op := range ops {
		if strings.HasPrefix(queryStr, op) {
			q.VersionOp = op
			queryStr = strings.TrimPrefix(queryStr, op)
			break
		}
	}

	// Handle version wildcards e.g. app-misc/foo-v3
	if idx := strings.LastIndex(queryStr, "-v"); idx != -1 {
		vPart := queryStr[idx+2:]
		// Check if it's a simple number to treat as wildcard
		isNum := true
		if vPart == "" {
			isNum = false
		} else {
			for _, r := range vPart {
				if r < '0' || r > '9' {
					isNum = false
					break
				}
			}
		}
		if isNum {
			q.VWildcard = vPart
			queryStr = queryStr[:idx]
		}
	}

	// If an operator was found, the rest of the string has the package AND the version, like "app-misc/foo-1.0.0"
	// We need to split the package and the version.
	if q.VersionOp != "" {
		// A Gentoo package version usually starts after the last hyphen followed by a digit.
		// However, it's easier to use a regex or just scan backwards.
		lastHyphen := strings.LastIndex(queryStr, "-")
		if lastHyphen != -1 && lastHyphen < len(queryStr)-1 {
			// Basic heuristic: if the part after hyphen starts with digit, it's the version
			if queryStr[lastHyphen+1] >= '0' && queryStr[lastHyphen+1] <= '9' {
				q.Version = queryStr[lastHyphen+1:]
				queryStr = queryStr[:lastHyphen]
			}
		}
	}

	// The remaining query string should be the package or category/package
	if queryStr != "" {
		parts := strings.Split(queryStr, "/")
		if len(parts) == 2 {
			q.Category = parts[0]
			q.Package = parts[1]
		} else {
			q.Package = queryStr
		}
	}

	return q, nil
}

func (cfg *MainArgConfig) cmdLintQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint query [flags] [<location>] <query>\n\n")
		fmt.Printf("  <location>\tOptional path to the overlay directory (defaults to '.').\n")
		fmt.Printf("  <query>\tQuery string (e.g. '>=app-misc/foo-v3::guru').\n\n")
		fmt.Printf("Flags:\n")
		fs.PrintDefaults()
	}
	format := fs.String("format", "text", "Output format: text, json, or github-actions")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")
	disableRule := fs.String("disable-rule", "", "Comma-separated list of rule IDs to ignore (case-insensitive)")
	ignoreTag := fs.String("ignore-tag", "", "Comma-separated list of tags to ignore")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var location, queryStr string
	if fs.NArg() == 0 {
		location = "."
		queryStr = ""
	} else if fs.NArg() == 1 {
		location = "."
		queryStr = fs.Arg(0)
	} else {
		location = fs.Arg(0)
		queryStr = fs.Arg(1)
	}

	query, err := parseLintQuery(queryStr, location)
	if err != nil {
		return fmt.Errorf("parsing query: %w", err)
	}

	return cfg.runLintCore(query.RepoPath, nil, query, *format, *severityFilter, *sourceFilter, *tagFilter, *disableRule, *ignoreTag)
}
