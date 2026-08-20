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

var ruleStabilizeNewVersions = lints.RuleMetadata{
	ID:          "StabilizeNewVersions",
	Title:       "Stabilize New Versions",
	Description: "Checks if a stabilized version missed some architectures that had stable versions.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/keywords.html#pg0402",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "keywords"},
}

func init() {
	lints.RegisterRuleMetadata(ruleStabilizeNewVersions)
	lints.RegisterLintRule(&StabilizeNewVersionsLintRule{})
}

type StabilizeNewVersionsLintRule struct{}

func (r *StabilizeNewVersionsLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *StabilizeNewVersionsLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
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

	previousStableArches := make(map[string]bool)

	for _, ver := range sortedVersions {
		keywordsStr := ver.Ebuild.Vars["KEYWORDS"]

		pkgArches := make(map[string]bool)
		stableArches := make(map[string]bool)

		hasStar := false
		if strings.TrimSpace(keywordsStr) != "" {
			for _, kw := range strings.Fields(keywordsStr) {
				arch := strings.TrimLeft(kw, "~-")
				if arch == "*" {
					hasStar = true
				}
				pkgArches[arch] = true
				if !strings.HasPrefix(kw, "~") && !strings.HasPrefix(kw, "-") {
					stableArches[arch] = true
				}
			}
		}

		if len(stableArches) > 0 && !hasStar {
			// Found a stabilized version. Check if it missed any previous stable arches
			var missedArches []string
			for arch := range previousStableArches {
				if pkgArches[arch] && !stableArches[arch] {
					// This architecture was stable in previous versions,
					// is present in this version (as unstable/testing ~arch),
					// but wasn't stabilized along with the other architectures in this version.
					missedArches = append(missedArches, arch)
				}
			}

			if len(missedArches) > 0 {
				sort.Strings(missedArches)
				results = append(results, lints.LintResult{
					RuleMetadata: ruleStabilizeNewVersions,
					Message:      fmt.Sprintf("[%s] Ebuild %s is stabilized but misses arches: %s", cases.Title(language.Und, cases.NoLower).String(string(lints.SeverityWarning)), ver.Version, strings.Join(missedArches, ", ")),
					Package:      pkg.Category + "/" + pkg.Name,
				})
			}
		}

		// Update stable arches for the next version
		for arch := range stableArches {
			previousStableArches[arch] = true
		}
	}

	return results
}
