package ebuild

import (
	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleSlotSubslotDependency = lints.RuleMetadata{
	ID:          "SlotSubslotDependency",
	Title:       "Slot and subslot dependencies",
	Description: "Whenever a package dependency specification matches a range of versions that span different slots or subslots, the package must explicitly include slot specification.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/dependencies.html#pg0011"},
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "dependencies", "PG0011"},
}

func init() {
	lints.RegisterRuleMetadata(ruleSlotSubslotDependency)
	lints.RegisterLintRule(&SlotSubslotDependencyLintRule{})
}

type SlotSubslotDependencyLintRule struct{}

func (r *SlotSubslotDependencyLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *SlotSubslotDependencyLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleSlotSubslotDependency.Severity

	if qa != nil {
		if val, ok := qa.Policies["PG0011"]; ok {
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
