package ebuild

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleBannedVariables = lints.RuleMetadata{
	ID:          "EAPIBannedVariables",
	Title:       "Banned Variables in EAPI",
	Description: "Detects the use of variables that have been banned or removed in specific EAPI versions.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/eapi/index.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "eapi"},
}

func init() {
	lints.RegisterRuleMetadata(ruleBannedVariables)
	lints.RegisterLintRule(&BannedVariablesLintRule{})
}

type BannedVariablesLintRule struct{}

func (r *BannedVariablesLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
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
			continue
		}

		bannedVariables := map[string]int{
			"PORTDIR":     7,
			"ECLASSDIR":   7,
			"DESTTREE":    7,
			"INSDESTTREE": 7,
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(ver.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			if assign, ok := node.(*syntax.Assign); ok {
				if assign.Name != nil {
					varName := assign.Name.Value
					if minEapi, banned := bannedVariables[varName]; banned && eapi >= minEapi {
						res := lints.LintResult{
							RuleMetadata: ruleBannedVariables,
							Message:      fmt.Sprintf("[Error] Ebuild %s assigns to '%s', which is banned in EAPI %d (and later).", ver.Version, varName, minEapi),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						results = append(results, res)
					}
				}
			}

			if param, ok := node.(*syntax.ParamExp); ok {
				if param.Param != nil {
					varName := param.Param.Value
					if minEapi, banned := bannedVariables[varName]; banned && eapi >= minEapi {
						res := lints.LintResult{
							RuleMetadata: ruleBannedVariables,
							Message:      fmt.Sprintf("[Error] Ebuild %s references '%s', which is banned in EAPI %d (and later).", ver.Version, varName, minEapi),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						results = append(results, res)
					}
				}
			}
			return true
		})
	}

	return results
}
