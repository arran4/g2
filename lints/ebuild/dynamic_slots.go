package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleDynamicSlots = lints.RuleMetadata{
	ID:          "DynamicSlots",
	Title:       "Dynamic Slots (multislot flag)",
	Description: "The use of multislot to alter SLOT values (as well as any other USE-dependent slot values) in the Gentoo repository is forbidden.",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/other-metadata.html#pg0701", Label: "Gentoo QA Policy Guide PG0701"},
		{URL: "https://wiki.gentoo.org/index.php?title=Project:Quality_Assurance/Policies&oldid=109991#multislot.2FUSE-dependent_SLOT", Label: "Gentoo QA Policy Archive"},
		{URL: "https://bugs.gentoo.org/174407", Label: "Gentoo Bug 174407"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "PG0701"},
}

func init() {
	lints.RegisterRuleMetadata(ruleDynamicSlots)
	lints.RegisterLintRule(&DynamicSlotsLintRule{})
}

type DynamicSlotsLintRule struct{}

func (r *DynamicSlotsLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *DynamicSlotsLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleDynamicSlots.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0701"]; ok {
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

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			iuseStr := ver.Ebuild.Vars["IUSE"]
			hasMultislot := false
			for _, token := range strings.Fields(iuseStr) {
				if token == "multislot" || token == "+multislot" || token == "-multislot" {
					hasMultislot = true
					break
				}
			}

			if hasMultislot {
				res := lints.LintResult{
					RuleMetadata: ruleDynamicSlots,
					Message:      fmt.Sprintf("[%s] Ebuild %s uses 'multislot' in IUSE which is forbidden", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
