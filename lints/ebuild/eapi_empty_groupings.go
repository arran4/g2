package ebuild

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleEmptyGroupings = lints.RuleMetadata{
	ID:          "EAPIEmptyGroupings",
	Title:       "Empty Groupings in EAPI",
	Description: "Detects empty groupings (e.g. || ( )) which are banned in EAPI 7 and later.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/ebuild-writing/eapi/index.html", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy", "eapi"},
}

func init() {
	lints.RegisterRuleMetadata(ruleEmptyGroupings)
	lints.RegisterLintRule(&EAPIEmptyGroupingsLintRule{})
}

type EAPIEmptyGroupingsLintRule struct{}

func (r *EAPIEmptyGroupingsLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	for _, ver := range pkg.Versions {
		if ver.Ebuild == nil {
			continue
		}

		eapiStr := "0"
		if ver.Ebuild.Vars != nil && ver.Ebuild.Vars["EAPI"] != "" {
			eapiStr = ver.Ebuild.Vars["EAPI"]
		}

		eapi, err := strconv.Atoi(eapiStr)
		if err != nil {
			continue // If EAPI is not a number, skip this check
		}

		if eapi < 7 {
			continue
		}

		varsToCheck := []string{"DEPEND", "RDEPEND", "BDEPEND", "PDEPEND", "REQUIRED_USE"}

		for _, varName := range varsToCheck {
			val := ver.Ebuild.Vars[varName]
			if val == "" {
				continue
			}

			// A simple state machine to find "||" followed by "(" followed by ")" without any other tokens in between
			tokens := strings.Fields(val)
			for i := 0; i < len(tokens)-2; i++ {
				if tokens[i] == "||" && tokens[i+1] == "(" && tokens[i+2] == ")" {
					res := lints.LintResult{
						RuleMetadata: ruleEmptyGroupings,
						Message:      fmt.Sprintf("[Error] Ebuild %s contains an empty grouping '|| ( )' in %s, which is banned in EAPI 7 (and later).", ver.Version, varName),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					results = append(results, res)
				}
			}
		}
	}

	return results
}
