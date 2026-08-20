package ebuild

import (
	"fmt"
	"regexp"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleEbuildHeader = lints.RuleMetadata{
	ID:          "EbuildHeader",
	Title:       "Invalid Ebuild Header",
	Description: "Validates that the ebuild header matches the expected Gentoo copyright header.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/file-format/index.html",
	Severity:    lints.SeverityNotice,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

var validHeaderRegex = regexp.MustCompile(`^# Copyright 1999-\d{4} Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\n`)

func init() {
	lints.RegisterRuleMetadata(ruleEbuildHeader)
	lints.RegisterLintRule(&EbuildHeaderLintRule{})
}

type EbuildHeaderLintRule struct{}

func (r *EbuildHeaderLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *EbuildHeaderLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityNotice

	if qa != nil {
		if val, ok := qa.Policies[ruleEbuildHeader.ID]; ok {
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

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			if !validHeaderRegex.MatchString(ver.Ebuild.RawText) {
				res := lints.LintResult{
					RuleMetadata: ruleEbuildHeader,
					Message:      fmt.Sprintf("[%s] Ebuild %s has an invalid header. It should exactly match the first two lines of header.txt followed by an empty line.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}
	return results
}
