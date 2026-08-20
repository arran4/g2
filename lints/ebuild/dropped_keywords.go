package ebuild

import (
	"fmt"
	"strings"
	"sort"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleDroppedKeywords = lints.RuleMetadata{
	ID:          "DroppedKeywords",
	Title:       "Dropped Keywords",
	Description: "Checks if arch keywords were dropped during version bumping.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/keywords.html#pg0401",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "keywords"},
}

func init() {
	lints.RegisterRuleMetadata(ruleDroppedKeywords)
	lints.RegisterLintRule(&DroppedKeywordsLintRule{})
}

type DroppedKeywordsLintRule struct{}

func (r *DroppedKeywordsLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *DroppedKeywordsLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult

	// Filter and sort non-live ebuilds
	var sortedVersions []g2.VersionData
	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			if strings.HasSuffix(ver.Version, "9999") {
				continue // Live ebuilds usually don't have keywords
			}
			sortedVersions = append(sortedVersions, ver)
		}
	}

	if len(sortedVersions) <= 1 {
		return nil
	}

	sort.Slice(sortedVersions, func(i, j int) bool {
		return g2.CompareVersions(sortedVersions[i].Version, sortedVersions[j].Version) < 0
	})

	seenArches := make(map[string]bool)
	previousArches := make(map[string]bool)

	// Track dropped arches per version to detect regressions
	changes := make(map[string][]string)

	for _, ver := range sortedVersions {
		keywordsStr := ver.Ebuild.Vars["KEYWORDS"]

		pkgArches := make(map[string]bool)
		disabledArches := make(map[string]bool)

		hasStar := false
		if strings.TrimSpace(keywordsStr) != "" {
			for _, kw := range strings.Fields(keywordsStr) {
				arch := strings.TrimLeft(kw, "~-")
				if arch == "*" {
					hasStar = true
				}
				pkgArches[arch] = true
				if strings.HasPrefix(kw, "-") {
					disabledArches[strings.TrimLeft(kw, "-")] = true
				}
			}
		}

		drops := make(map[string]bool)
		if !hasStar {
			for arch := range previousArches {
				if !pkgArches[arch] {
					drops[arch] = true
				}
			}
			for arch := range seenArches {
				if !pkgArches[arch] {
					drops[arch] = true
				}
			}
		}

		for key := range drops {
			changes[key] = append(changes[key], ver.Version)
		}

		if len(changes) > 0 {
			adds := make(map[string]bool)
			for arch := range pkgArches {
				if !previousArches[arch] && !disabledArches[arch] {
					adds[arch] = true
				}
			}

			for key := range adds {
				delete(changes, key)
			}
		}

		for arch := range pkgArches {
			seenArches[arch] = true
		}

		previousArches = pkgArches
	}

	dropped := make(map[string][]string)
	for key, pkgs := range changes {
		if len(pkgs) > 0 {
			// only report the most recent pkg with dropped keywords (like pkgcheck does without verbose)
			latestPkg := pkgs[len(pkgs)-1]
			dropped[latestPkg] = append(dropped[latestPkg], key)
		}
	}

	for pkgVer, keys := range dropped {
		sort.Strings(keys)
		if len(keys) > 0 {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleDroppedKeywords,
				Message:      fmt.Sprintf("[%s] Ebuild %s has dropped keywords: %s", cases.Title(language.Und, cases.NoLower).String(string(lints.SeverityWarning)), pkgVer, strings.Join(keys, ", ")),
				Package:      pkg.Category + "/" + pkg.Name,
			})
		}
	}

	return results
}
