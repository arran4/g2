package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleRespectVariables = lints.RuleMetadata{
	ID:          "RespectVariables",
	Title:       "Respect Variables",
	Description: "Checks for assignments that overwrite CFLAGS, CXXFLAGS, or LDFLAGS without appending or referencing the original variable.",
	URL:         "https://devmanual.gentoo.org/general-concepts/user-environment/index.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "qa"},
}

func init() {
	lints.RegisterRuleMetadata(ruleRespectVariables)
	lints.RegisterLintRule(&RespectVariablesLintRule{})
}

type RespectVariablesLintRule struct{}

func (l *RespectVariablesLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			if assign, ok := node.(*syntax.Assign); ok {
				if assign.Name != nil && assign.Value != nil { // Ignore `export CFLAGS` where Value is nil
					name := assign.Name.Value
					if name == "CFLAGS" || name == "CXXFLAGS" || name == "LDFLAGS" {
						if !assign.Append && !hasSelfReference(assign.Value, name) {
							res := lints.LintResult{
								RuleMetadata: ruleRespectVariables,
								Message:      fmt.Sprintf("Ebuild %s overwrites %s unconditionally. You must respect the user's %s (e.g., use += or filter-flags instead).", version.Ebuild.Path, name, name),
								Package:      pkgData.Category + "/" + pkgData.Name,
							}
							results = append(results, res)
						}
					}
				}
			}
			return true
		})
	}

	return results
}

func hasSelfReference(word *syntax.Word, varName string) bool {
	if word == nil {
		return false
	}
	found := false
	syntax.Walk(word, func(node syntax.Node) bool {
		if param, ok := node.(*syntax.ParamExp); ok {
			if param.Param != nil && param.Param.Value == varName {
				found = true
				return false // stop walking
			}
		}
		return true
	})
	return found
}
