package ebuild

import (
	"github.com/arran4/g2/lints"
)

var ruleUseDependencies = lints.RuleMetadata{
	ID:          "UseDependencies",
	Title:       "USE dependencies",
	Description: "Whenever a package depends on a multi-flag package or requires specific USE configurations, explicit USE dependencies should be specified where required (deferred checking).",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/dependencies.html#pg0021", Label: "Gentoo QA Policy Guide PG0021"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "dependencies", "PG0021"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUseDependencies)
}
