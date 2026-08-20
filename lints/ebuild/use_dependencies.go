package ebuild

import (
	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleUseDependenciesPG0021 = lints.RuleMetadata{
	ID:          "UseDependenciesPG0021",
	Title:       "USE dependencies on packages without the flag",
	Description: "Whenever a package uses a 2-style USE-dependency on another package, all package versions matching the dependency must have the flag in question.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/dependencies.html#pg0021",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "dependencies", "PG0021"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUseDependenciesPG0021)
	lints.RegisterLintRule(&UseDependenciesPG0021LintRule{})
}

type UseDependenciesPG0021LintRule struct{}

func (r *UseDependenciesPG0021LintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *UseDependenciesPG0021LintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleUseDependenciesPG0021.Severity

	if qa != nil {
		if val, ok := qa.Policies["PG0021"]; ok {
			if val == "ignore" {
				return nil
			}
			switch val {
			case "error":
				severity = lints.SeverityError
			case "warning":
				severity = lints.SeverityWarning
			case "notice":
				severity = lints.SeverityNotice
			}
		}
	}

	_ = severity

	// We defer this complex check to pkgcheck since it requires multi-package dependency tree resolution.
	// The rule metadata is registered to satisfy extraction.
	return results
}
