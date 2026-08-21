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

var ruleCopyrightHeader = lints.RuleMetadata{
	ID:          "CopyrightHeader",
	Title:       "Generic Copyright Header",
	Description: "Validates the presence of a plausible copyright/licensing header in ebuild files.",
	URL:         "",
	Severity:    lints.SeverityNotice,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "site-quality"},
}

var ruleGentooCopyrightHeader = lints.RuleMetadata{
	ID:          "GentooCopyrightHeader",
	Title:       "Gentoo Copyright Header",
	Description: "Validates the copyright notice for Gentoo main policy.",
	URL:         "https://devmanual.gentoo.org/general-concepts/copyright-policy/index.html",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

func init() {
	lints.RegisterRuleMetadata(ruleCopyrightHeader)
	lints.RegisterLintRule(&CopyrightHeaderLintRule{})

	lints.RegisterRuleMetadata(ruleGentooCopyrightHeader)
	lints.RegisterLintRule(&GentooCopyrightHeaderLintRule{})
}

type CopyrightHeaderLintRule struct{}

// Very forgiving generic check. Just looking for shell comments and copyright/license keywords.
func isGenericCopyrightHeader(firstLine string, secondLine string) bool {
	if !strings.HasPrefix(firstLine, "#") {
		return false
	}
	combined := strings.ToLower(firstLine + "\n" + secondLine)

	hasYear := regexp.MustCompile(`\d{4}`).MatchString(combined)

	hasCopyrightWord := strings.Contains(combined, "copyright") ||
		strings.Contains(combined, "license") ||
		strings.Contains(combined, "licence") ||
		strings.Contains(combined, "distributed") ||
		strings.Contains(combined, "terms") ||
		strings.Contains(combined, "spdx") ||
		strings.Contains(combined, "gpl") ||
		strings.Contains(combined, "bsd") ||
		strings.Contains(combined, "mit") ||
		strings.Contains(combined, "apache")

	return hasYear || hasCopyrightWord
}

func (r *CopyrightHeaderLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *CopyrightHeaderLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	return r.LintWithRuleSet(repoDir, pkg, "default", qa)
}

func (r *CopyrightHeaderLintRule) LintWithRuleSet(repoDir string, pkg *g2.PackageData, ruleSetID string, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	rs, rsOk := lints.GetRuleSet(ruleSetID)
	if !rsOk {
		return nil
	}

	entry, entryOk := rs.Rules["CopyrightHeader"]
	if !entryOk || !entry.Enabled {
		return nil
	}

	severity := entry.Severity

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			lines := strings.SplitN(ver.Ebuild.RawText, "\n", 3)
			firstLine := ""
			secondLine := ""
			if len(lines) > 0 {
				firstLine = lines[0]
			}
			if len(lines) > 1 {
				secondLine = lines[1]
			}

			if !isGenericCopyrightHeader(firstLine, secondLine) {
				res := lints.LintResult{
					RuleMetadata: ruleCopyrightHeader,
					Message:      fmt.Sprintf("[%s] Ebuild %s has a missing or malformed copyright/licensing notice.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}
	return results
}

type GentooCopyrightHeaderLintRule struct{}

var gentooCopyrightRegex = regexp.MustCompile(`^# Copyright \d{4}(?:-\d{4})? (?:Gentoo Authors|Gentoo Foundation)`)

func (r *GentooCopyrightHeaderLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *GentooCopyrightHeaderLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	return r.LintWithRuleSet(repoDir, pkg, "default", qa)
}

func (r *GentooCopyrightHeaderLintRule) LintWithRuleSet(repoDir string, pkg *g2.PackageData, ruleSetID string, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	rs, rsOk := lints.GetRuleSet(ruleSetID)
	if !rsOk {
		return nil
	}

	entry, entryOk := rs.Rules["GentooCopyrightHeader"]
	if !entryOk || !entry.Enabled {
		return nil
	}

	severity := entry.Severity

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			lines := strings.SplitN(ver.Ebuild.RawText, "\n", 2)
			if len(lines) > 0 {
				firstLine := lines[0]
				if !gentooCopyrightRegex.MatchString(firstLine) {
					res := lints.LintResult{
						RuleMetadata: ruleGentooCopyrightHeader,
						Message:      fmt.Sprintf("[%s] Ebuild %s has a missing or malformed Gentoo copyright notice.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
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
