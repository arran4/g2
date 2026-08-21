package ebuild

import (
	"github.com/arran4/g2/lints"
)

var ruleUseDependencies = lints.RuleMetadata{
	ID:          "UseDependencies",
	Title:       "USE dependencies",
	Description: "Whenever a package uses a 2-style USE-dependency, every matching package version must have the specified flag; otherwise the dependency must be narrowed or use a 4-style default (deferred checking).",
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
