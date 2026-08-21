package ebuild

import (
	"fmt"
	"unicode/utf8"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleCharacterSet = lints.RuleMetadata{
	ID:          "CharacterSet",
	Title:       "Character Set",
	Description: "Validates that ebuild files use the UTF-8 character set.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/ebuild-writing/file-format/index.html#character-set", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy", "encoding"},
}

func init() {
	lints.RegisterRuleMetadata(ruleCharacterSet)
	lints.RegisterLintRule(&CharacterSetLintRule{})
}

type CharacterSetLintRule struct{}

func (r *CharacterSetLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *CharacterSetLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if qa != nil {
		if val, ok := qa.Policies[ruleCharacterSet.ID]; ok {
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
			if !utf8.ValidString(ver.Ebuild.RawText) {
				res := lints.LintResult{
					RuleMetadata: ruleCharacterSet,
					Message:      fmt.Sprintf("[%s] Ebuild %s is not valid UTF-8.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}
	return results
}
