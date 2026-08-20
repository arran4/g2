package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleIndentingWhitespace = lints.RuleMetadata{
	ID:          "IndentingWhitespace",
	Title:       "Indenting and Whitespace",
	Description: "Validates that ebuilds use tabs for indentation and contain no trailing whitespace.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/file-format/index.html#indenting-and-whitespace",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "whitespace"},
}

func init() {
	lints.RegisterRuleMetadata(ruleIndentingWhitespace)
	lints.RegisterLintRule(&IndentingWhitespaceLintRule{})
}

type IndentingWhitespaceLintRule struct{}

func (r *IndentingWhitespaceLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *IndentingWhitespaceLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil {
		if val, ok := qa.Policies[ruleIndentingWhitespace.ID]; ok {
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
			lines := strings.Split(ver.Ebuild.RawText, "\n")
			for i, line := range lines {
				line = strings.TrimSuffix(line, "\r")
				if len(line) == 0 {
					continue
				}

				// Check trailing whitespace
				if line[len(line)-1] == ' ' || line[len(line)-1] == '\t' {
					res := lints.LintResult{
						RuleMetadata: ruleIndentingWhitespace,
						Message:      fmt.Sprintf("[%s] Ebuild %s has trailing whitespace on line %d.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, i+1),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}

				// Check leading space for indentation and tabs after indentation
				indenting := true
				positions := 0
				hasTabAfterIndent := false
				hasSpaceIndent := false

				for _, ch := range line {
					if indenting {
						if ch == '\t' {
							positions += 4
							continue
						} else if ch == ' ' {
							hasSpaceIndent = true
							indenting = false
							positions += 1
						} else {
							indenting = false
							positions += 1
						}
					} else {
						if ch == '\t' {
							hasTabAfterIndent = true
							positions += 4
						} else {
							positions += 1
						}
					}
				}

				if hasSpaceIndent {
					res := lints.LintResult{
						RuleMetadata: ruleIndentingWhitespace,
						Message:      fmt.Sprintf("[%s] Ebuild %s uses spaces for indentation on line %d.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, i+1),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}

				if hasTabAfterIndent {
					res := lints.LintResult{
						RuleMetadata: ruleIndentingWhitespace,
						Message:      fmt.Sprintf("[%s] Ebuild %s has a tab character outside of indentation on line %d.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, i+1),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}

				if positions > 80 {
					res := lints.LintResult{
						RuleMetadata: ruleIndentingWhitespace,
						Message:      fmt.Sprintf("[%s] Ebuild %s has a line exceeding 80 positions on line %d.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, i+1),
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
