package ebuild

import (
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleSeparateUsr = lints.RuleMetadata{
	ID:          "SeparateUsr",
	Title:       "Support for separate /usr",
	Description: "Developers are not required to support using separate /usr filesystem without an initramfs. Warnings regarding this are typically suppressed or not fatal.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0202",
	Severity:    lints.SeverityInfo,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "filesystem", "PG0202"},
}

func init() {
	lints.RegisterRuleMetadata(ruleSeparateUsr)
	lints.RegisterLintRule(&SeparateUsrLintRule{})
}

type SeparateUsrLintRule struct{}

func (l *SeparateUsrLintRule) Lint(repoDir string, pkgData *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil, ctx)
}

func (l *SeparateUsrLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil || version.Ebuild.RawText == "" {
			continue
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		_ = f
	}

	return results
}
