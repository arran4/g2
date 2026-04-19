package metadata

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMaintainerMissing = lints.RuleMetadata{
	ID:          "MaintainerMissing",
	Title:       "Missing Maintainer",
	Description: "Ensures that a package has at least one maintainer defined in its metadata.xml.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/misc-files/metadata.xml/index.html#maintainer-field",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"metadata.xml", "site-quality"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMaintainerMissing)
	lints.RegisterLintRule(&MaintainerLintRule{})
}

type MaintainerLintRule struct{}

func (r *MaintainerLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MaintainerLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0702"]; ok {
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
		if len(pkg.Metadata.Maintainers) == 0 {
			res := lints.LintResult{
				RuleMetadata: ruleMaintainerMissing,
				Message:      fmt.Sprintf("[%s] Package has no maintainers defined in metadata.xml", cases.Title(language.Und, cases.NoLower).String(string(severity))),
				Package:      pkg.Category + "/" + pkg.Name,
			}
			res.RuleMetadata.Severity = severity
			results = append(results, res)
		}
	}

	return results
}
