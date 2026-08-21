package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleRedundantS = lints.RuleMetadata{
	ID:          "RedundantS",
	Title:       "Adding redundant S=${WORKDIR}/${P}",
	Description: "If S=${WORKDIR}/${P}, then you should not add it to your ebuild. This is implied already.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/ebuild-writing/common-mistakes/index.html#adding-redundant-sworkdirp", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "qa"},
}

func init() {
	lints.RegisterRuleMetadata(ruleRedundantS)
	lints.RegisterLintRule(&RedundantSLintRule{})
}

type RedundantSLintRule struct{}

func (l *RedundantSLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
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

		printer := syntax.NewPrinter()

		syntax.Walk(f, func(node syntax.Node) bool {
			if assign, ok := node.(*syntax.Assign); ok {
				if assign.Name != nil && assign.Name.Value == "S" && assign.Value != nil {
					var buf strings.Builder
					_ = printer.Print(&buf, assign.Value)
					val := buf.String()

					if val == "${WORKDIR}/${P}" || val == "\"${WORKDIR}/${P}\"" || val == "${WORKDIR}/$P" || val == "\"${WORKDIR}/$P\"" {
						res := lints.LintResult{
							RuleMetadata: ruleRedundantS,
							Message:      fmt.Sprintf("Ebuild %s defines S=\"%s\", which is the default. You should remove it.", version.Ebuild.Path, val),
							Package:      pkgData.Category + "/" + pkgData.Name,
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
