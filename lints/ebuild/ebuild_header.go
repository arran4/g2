package ebuild

import (
	"fmt"
	"regexp"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleEbuildHeader = lints.RuleMetadata{
	ID:          "EbuildHeader",
	Title:       "Ebuild Header Formatting",
	Description: "Checks if ebuild files have the correct header formatting.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

// EbuildHeaderLintRule checks if ebuild header is correct.
type EbuildHeaderLintRule struct{}

func init() {
	lints.RegisterRuleMetadata(ruleEbuildHeader)
	lints.RegisterLintRule(&EbuildHeaderLintRule{})
}

var correctHeaderRegex = regexp.MustCompile(`(?s)^# Copyright 1999-\d{4} Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2$`)

func (l *EbuildHeaderLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkg, nil)
}

func (l *EbuildHeaderLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil {
			if !correctHeaderRegex.MatchString(ver.Ebuild.EbuildHeader) {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleEbuildHeader,
					Message:      "Ebuild header is missing or incorrectly formatted.",
					Package:      pkg.Category + "/" + pkg.Name,
					File:         fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version),
				})
			}
		}
	}

	return results
}
