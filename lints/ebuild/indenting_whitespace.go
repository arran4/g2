package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleIndentingWhitespace = lints.RuleMetadata{
	ID:          "IndentingWhitespace",
	Title:       "Indenting and Whitespace",
	Description: "Checks if ebuild files use tabs for indentation and have no trailing whitespace.",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "whitespace"},
}

// IndentingWhitespaceLintRule checks if ebuild files use tabs for indentation and have no trailing whitespace.
type IndentingWhitespaceLintRule struct{}

func init() {
	lints.RegisterRuleMetadata(ruleIndentingWhitespace)
	lints.RegisterLintRule(&IndentingWhitespaceLintRule{})
}

func (l *IndentingWhitespaceLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkg, nil)
}

func (l *IndentingWhitespaceLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			lines := strings.Split(ver.Ebuild.RawText, "\n")
			for i, line := range lines {
				if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
					results = append(results, lints.LintResult{
						RuleMetadata: ruleIndentingWhitespace,
						Message:      fmt.Sprintf("Trailing whitespace found at line %d", i+1),
						Package:      pkg.Category + "/" + pkg.Name,
						File:         fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version),
						Line:         i + 1,
					})
				}

				trimmedLeftSpace := strings.TrimLeft(line, " \t")
				if len(line) != len(trimmedLeftSpace) && line[0] == ' ' {
					results = append(results, lints.LintResult{
						RuleMetadata: ruleIndentingWhitespace,
						Message:      fmt.Sprintf("Leading space instead of tab for indentation found at line %d", i+1),
						Package:      pkg.Category + "/" + pkg.Name,
						File:         fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version),
						Line:         i + 1,
					})
				}
			}
		}
	}

	return results
}
