package ebuild

import (
	"fmt"
	"unicode/utf8"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleCharacterSet = lints.RuleMetadata{
	ID:          "CharacterSet",
	Title:       "Character Set",
	Description: "Checks if ebuild files use the UTF-8 character set.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "encoding"},
}

// CharacterSetLintRule checks if ebuild files use the UTF-8 character set.
type CharacterSetLintRule struct{}

func init() {
	lints.RegisterRuleMetadata(ruleCharacterSet)
	lints.RegisterLintRule(&CharacterSetLintRule{})
}

func (l *CharacterSetLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkg, nil)
}

func (l *CharacterSetLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			if !utf8.ValidString(ver.Ebuild.RawText) {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleCharacterSet,
					Message:      "Ebuild file contains non-UTF-8 characters.",
					Package:      pkg.Category + "/" + pkg.Name,
					File:         fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version),
				})
			}
		}
	}

	return results
}
