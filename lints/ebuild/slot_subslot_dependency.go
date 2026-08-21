package ebuild

import (
	"github.com/arran4/g2/lints"
)

var ruleSlotSubslotDependency = lints.RuleMetadata{
	ID:          "SlotSubslotDependency",
	Title:       "Slot and subslot dependencies",
	Description: "Whenever a package dependency specification matches a range of versions that span different slots or subslots, the package must explicitly include slot specification (deferred checking).",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/dependencies.html#pg0011", Label: "Gentoo QA Policy Guide PG0011"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "dependencies", "PG0011"},
}

func init() {
	lints.RegisterRuleMetadata(ruleSlotSubslotDependency)
}
