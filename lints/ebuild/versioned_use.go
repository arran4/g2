package ebuild

import (
	"fmt"
	"regexp"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleVersionedUse = lints.RuleMetadata{
	ID:          "VersionedUse",
	Title:       "Versioned USE flags",
	Description: "If a need arises to create new USE flags responsible for switching between multiple versions of a specific dependency, it is recommended that flat, explicitly versioned flags are used (e.g. qt4, qt5). The hierarchical form used e.g. by GTK+ (gtk meaning 2-or-3, then optionally gtk2 or gtk3 to switch between both) is discouraged.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/use-flags.html#pg0801"},
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "use", "PG0801"},
}

func init() {
	lints.RegisterRuleMetadata(ruleVersionedUse)
	lints.RegisterLintRule(&VersionedUseLintRule{})
}

type VersionedUseLintRule struct{}

func (r *VersionedUseLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *VersionedUseLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0801"]; ok {
			if val == "ignore" {
				return nil
			}
			if val == "notice" || val == "error" || val == "warning" {
				switch val {
				case "notice":
					severity = lints.SeverityNotice
				case "error":
					severity = lints.SeverityError
				case "warning":
					severity = lints.SeverityWarning
				}
			}
		}
	}

	versionedFlagRegex := regexp.MustCompile(`^[a-zA-Z]+[0-9]+$`)

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			iuse := ver.Ebuild.Vars["IUSE"]
			if iuse == "" {
				continue
			}

			flags := g2.ParseIUSE(iuse)
			flagSet := make(map[string]bool)
			for _, flag := range flags {
				// Strip USE_EXPAND or + / - prefixes handled by ParseIUSE (wait, ParseIUSE strips + and -)
				flagSet[flag] = true
			}

			for flag := range flagSet {
				if versionedFlagRegex.MatchString(flag) {
					// Extract the unversioned part (e.g. "gtk" from "gtk2")
					unversioned := ""
					for i, r := range flag {
						if r >= '0' && r <= '9' {
							unversioned = flag[:i]
							break
						}
					}

					if unversioned != "" && flagSet[unversioned] {
						// Found hierarchical versioning (e.g., both gtk and gtk2)
						res := lints.LintResult{
							RuleMetadata: ruleVersionedUse,
							Message:      fmt.Sprintf("[Warning] Ebuild %s uses hierarchical versioned USE flags (%s and %s). Flat, explicitly versioned flags (e.g. %s only) are recommended.", ver.Version, unversioned, flag, flag),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
					}
				}
			}
		}
	}

	return results
}
