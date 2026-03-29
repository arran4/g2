package ebuild

import (
	"fmt"
	"regexp"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleInvalidSlot = lints.RuleMetadata{
	ID:          "InvalidSlot",
	Title:       "Invalid SLOT",
	Description: "Validates that the SLOT variable only contains valid characters.",
	URL:         "https://devmanual.gentoo.org/general-concepts/slotting/",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

// PMS 3.1.3: Alphanumeric, plus _, ., -
// Can optionally have a subslot, separated by /
var validSlotRegex = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)?$`)

func init() {
	lints.RegisterRuleMetadata(ruleInvalidSlot)
	lints.RegisterLintRule(&InvalidSlotLintRule{})
}

type InvalidSlotLintRule struct{}

func (r *InvalidSlotLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *InvalidSlotLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			slot, ok := ver.Ebuild.Vars["SLOT"]
			if !ok {
				// if SLOT isn't defined, parser puts empty string, or we skip
				continue
			}

			if slot != "" && !validSlotRegex.MatchString(slot) {
				res := lints.LintResult{
					RuleMetadata: ruleInvalidSlot,
					Message:      fmt.Sprintf("[%s] Ebuild %s has invalid SLOT '%s'", cases.Title(language.English).String(string(severity)), ver.Version, slot),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}
	return results
}
