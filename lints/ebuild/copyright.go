package ebuild

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleCopyrightNotice = lints.RuleMetadata{
	ID:          "CopyrightNotice",
	Title:       "Copyright Notice",
	Description: "Validates the copyright notice in ebuild files.",
	URL:         "https://devmanual.gentoo.org/general-concepts/copyright-policy/index.html",
	Severity:    lints.SeverityNotice,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "site-quality"},
}

func init() {
	lints.RegisterRuleMetadata(ruleCopyrightNotice)
	lints.RegisterLintRule(&CopyrightLintRule{})
}

type CopyrightLintRule struct{}

var copyrightRegex = regexp.MustCompile(`^# Copyright \d{4}(?:-\d{4})? (?:Gentoo Authors|Gentoo Foundation)`)

func (r *CopyrightLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *CopyrightLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityNotice
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["CopyrightNotice"]; ok {
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

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			lines := strings.SplitN(ver.Ebuild.RawText, "\n", 2)
			if len(lines) > 0 {
				firstLine := lines[0]
				if !copyrightRegex.MatchString(firstLine) {
					res := lints.LintResult{
						RuleMetadata: ruleCopyrightNotice,
						Message:      fmt.Sprintf("[%s] Ebuild %s has a missing or malformed copyright notice.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}
			}
		}
	}
	return results
}
