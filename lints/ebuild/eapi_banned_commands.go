package ebuild

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleBannedCommands = lints.RuleMetadata{
	ID:          "EAPIBannedCommands",
	Title:       "Banned Commands in EAPI",
	Description: "Detects the use of commands that have been banned or deprecated in specific EAPI versions.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/eapi/index.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "eapi"},
}

func init() {
	lints.RegisterRuleMetadata(ruleBannedCommands)
	lints.RegisterLintRule(&BannedCommandsLintRule{})
}

type BannedCommandsLintRule struct{}

func (r *BannedCommandsLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
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

		bannedCommands := map[string]int{
			"einstall": 6,
			"dohtml":   7, // banned in 7, deprecated in 6, we'll error on 7
			"dolib":    7,
			"libopts":  7,
			"hasq":     8,
			"hasv":     8,
			"useq":     8,
			"assert":   9,
			"domo":     9,
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(ver.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			cmd, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}

			if len(cmd.Args) > 0 {
				var cmdName string
				if len(cmd.Args[0].Parts) == 1 {
					if lit, ok := cmd.Args[0].Parts[0].(*syntax.Lit); ok {
						cmdName = lit.Value
					}
				}

				if minEapi, banned := bannedCommands[cmdName]; banned {
					if eapi >= minEapi {
						res := lints.LintResult{
							RuleMetadata: ruleBannedCommands,
							Message:      fmt.Sprintf("[Error] Ebuild %s uses '%s', which is banned in EAPI %d (and later).", ver.Version, cmdName, minEapi),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						results = append(results, res)
					} else if cmdName == "dohtml" && eapi == 6 {
						res := lints.LintResult{
							RuleMetadata: ruleBannedCommands,
							Message:      fmt.Sprintf("[Warning] Ebuild %s uses 'dohtml', which is deprecated in EAPI 6.", ver.Version),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = lints.SeverityWarning
						results = append(results, res)
					}
				}
			}
			return true
		})
	}

	return results
}
