package metadata

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMaintainerNeeded = lints.RuleMetadata{
	ID:          "MaintainerNeeded",
	Title:       "Maintainer Needed Comment Missing or Incorrect",
	Description: "Ensures that a package with no maintainers has the <!-- maintainer-needed --> comment in its metadata.xml, and packages with maintainers do not have it.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/general-concepts/package-maintainers/index.html#adding-and-removing-maintainers", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"metadata.xml", "site-quality", "PG0703"}, // Using a new tag, e.g., PG0703
}

func init() {
	lints.RegisterRuleMetadata(ruleMaintainerNeeded)
	lints.RegisterLintRule(&MaintainerNeededLintRule{})
}

type MaintainerNeededLintRule struct{}

func (r *MaintainerNeededLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MaintainerNeededLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0703"]; ok {
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
		hasMaintainers := len(pkg.Metadata.Maintainers) > 0
		hasComment := false
		for _, comment := range pkg.Metadata.Comments {
			if strings.Contains(comment, "maintainer-needed") {
				hasComment = true
				break
			}
		}

		if !hasMaintainers && !hasComment {
			res := lints.LintResult{
				RuleMetadata: ruleMaintainerNeeded,
				Message:      fmt.Sprintf("[%s] Package has no maintainers but is missing the <!-- maintainer-needed --> comment in metadata.xml", cases.Title(language.Und, cases.NoLower).String(string(severity))),
				Package:      pkg.Category + "/" + pkg.Name,
			}
			res.RuleMetadata.Severity = severity
			results = append(results, res)
		} else if hasMaintainers && hasComment {
			res := lints.LintResult{
				RuleMetadata: ruleMaintainerNeeded,
				Message:      fmt.Sprintf("[%s] Package has maintainers but contains the <!-- maintainer-needed --> comment in metadata.xml", cases.Title(language.Und, cases.NoLower).String(string(severity))),
				Package:      pkg.Category + "/" + pkg.Name,
			}
			res.RuleMetadata.Severity = severity
			results = append(results, res)
		}
	}

	return results
}
