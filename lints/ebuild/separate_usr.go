package ebuild

import (
	"github.com/arran4/g2/lints"
)

var ruleSeparateUsr = lints.RuleMetadata{
	ID:          "SeparateUsr",
	Title:       "Support for separate /usr",
	Description: "Developers are not required to support using separate /usr filesystem without an initramfs. Informational policy reference.",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0202", Label: "Gentoo QA Policy Guide PG0202"},
	},
	Severity: lints.SeverityInfo,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "filesystem", "PG0202"},
}

func init() {
	lints.RegisterRuleMetadata(ruleSeparateUsr)
}
