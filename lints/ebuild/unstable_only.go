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

var ruleUnstableOnly = lints.RuleMetadata{
	ID:          "UnstableOnly",
	Title:       "Unstable Only",
	Description: "Checks if a package has only unstable keywords for an architecture.",
	URL:         "",
	Severity:    lints.SeverityNotice, // pkgcheck puts it as Info
	Source:      lints.SourcePkgcheck,
	Tags:        []string{"ebuild", "keywords"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUnstableOnly)
	lints.RegisterLintRule(&UnstableOnlyLintRule{})
}

type UnstableOnlyLintRule struct{}

func (r *UnstableOnlyLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *UnstableOnlyLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
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

	if len(sortedVersions) == 0 {
		return nil
	}

	sort.Slice(sortedVersions, func(i, j int) bool {
		return g2.CompareVersions(sortedVersions[i].Version, sortedVersions[j].Version) < 0
	})

	// Collect all architectures mentioned in the package
	allArches := make(map[string]bool)
	archVersions := make(map[string][]string)
	hasStable := make(map[string]bool)

	for _, ver := range sortedVersions {
		keywordsStr := ver.Ebuild.Vars["KEYWORDS"]

		if strings.TrimSpace(keywordsStr) != "" {
			for _, kw := range strings.Fields(keywordsStr) {
				arch := strings.TrimLeft(kw, "~-")
				if arch == "*" {
					continue
				}
				allArches[arch] = true

				if strings.HasPrefix(kw, "~") {
					archVersions[arch] = append(archVersions[arch], ver.Version)
				} else if !strings.HasPrefix(kw, "-") {
					hasStable[arch] = true
				}
			}
		}
	}

	// Group arches by the exact set of unstable versions
	unstableGroups := make(map[string][]string)
	for arch := range allArches {
		if !hasStable[arch] && len(archVersions[arch]) > 0 {
			versionsKey := strings.Join(archVersions[arch], ",")
			unstableGroups[versionsKey] = append(unstableGroups[versionsKey], arch)
		}
	}

	for versionsStr, arches := range unstableGroups {
		sort.Strings(arches)
		versions := strings.Split(versionsStr, ",")

		results = append(results, lints.LintResult{
			RuleMetadata: ruleUnstableOnly,
			Message:      fmt.Sprintf("[%s] Package has only unstable keywords for arch(es): [ %s ], all versions are unstable: [ %s ]",
				cases.Title(language.Und, cases.NoLower).String(string(lints.SeverityNotice)),
				strings.Join(arches, ", "),
				strings.Join(versions, ", ")),
			Package:      pkg.Category + "/" + pkg.Name,
		})
	}

	return results
}
