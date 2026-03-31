package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMissingSlot = lints.RuleMetadata{
	ID:          "MissingSlot",
	Title:       "Missing SLOT",
	Description: "Detects ebuilds that do not define a SLOT variable, which is required.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "metadata"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMissingSlot)
	lints.RegisterLintRule(&MissingSlotLintRule{})
}

type MissingSlotLintRule struct{}

func (r *MissingSlotLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MissingSlotLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleMissingSlot.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0804"]; ok { // Assign a pseudo-QA policy code or omit
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
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			slot, ok := ver.Ebuild.Vars["SLOT"]
			if !ok || strings.TrimSpace(slot) == "" {
				res := lints.LintResult{
					RuleMetadata: ruleMissingSlot,
					Message:      fmt.Sprintf("[%s] Ebuild %s is missing the required SLOT variable.", cases.Title(language.English).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
