package metadata

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMaintainerType = lints.RuleMetadata{
	ID:          "MaintainerType",
	Title:       "Maintainer Type Must Be Explicit",
	Description: "Ensures that all Gentoo maintainers in metadata.xml have their type explicitly set to either person or project.",
	URL:         "https://www.gentoo.org/glep/glep-0067.html#new-metadata-xml-format",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"metadata.xml", "site-quality", "GLEP67"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMaintainerType)
	lints.RegisterLintRule(&MaintainerTypeLintRule{})
}

type MaintainerTypeLintRule struct{}

func (r *MaintainerTypeLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MaintainerTypeLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["GLEP67"]; ok {
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

	if pkg.Metadata != nil {
		for _, m := range pkg.Metadata.Maintainers {
			if m.Type != "person" && m.Type != "project" {
				res := lints.LintResult{
					RuleMetadata: ruleMaintainerType,
					Message:      fmt.Sprintf("[%s] Maintainer '%s' must explicitly set type to 'person' or 'project'", cases.Title(language.Und, cases.NoLower).String(string(severity)), m.Email),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
