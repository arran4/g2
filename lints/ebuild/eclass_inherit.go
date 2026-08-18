package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleConditionalInherit = lints.RuleMetadata{
	ID:          "ConditionalInherit",
	Title:       "Conditional Eclass Inheritance",
	Description: "Warns about conditional inheritance of eclasses, which is illegal unless based strictly on PN or PV.",
	URL:         "https://devmanual.gentoo.org/appendices/common-problems/index.html#qa-notice-eclass-foo-inherited-illegally",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "qa"},
}

func init() {
	lints.RegisterRuleMetadata(ruleConditionalInherit)
	lints.RegisterLintRule(&ConditionalInheritLintRule{})
}

type ConditionalInheritLintRule struct{}

func (l *ConditionalInheritLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
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

		var walk func(node syntax.Node, inConditional bool)
		walk = func(node syntax.Node, inConditional bool) {
			if node == nil {
				return
			}

			switch n := node.(type) {
			case *syntax.CallExpr:
				if len(n.Args) > 0 {
					var cmdName string
					if len(n.Args[0].Parts) == 1 {
						if lit, ok := n.Args[0].Parts[0].(*syntax.Lit); ok {
							cmdName = lit.Value
						}
					}
					if cmdName == "inherit" && inConditional {
						res := lints.LintResult{
							RuleMetadata: ruleConditionalInherit,
							Message:      fmt.Sprintf("[Error] Ebuild %s inherits eclasses conditionally, which is not permitted. All eclass inherits must be unconditional.", version.Ebuild.Path),
							Package:      pkgData.Category + "/" + pkgData.Name,
						}
						results = append(results, res)
					}
				}
			}

			// Traverse deeper
			switch n := node.(type) {
			case *syntax.File:
				for _, stmt := range n.Stmts {
					walk(stmt, inConditional)
				}
			case *syntax.Stmt:
				walk(n.Cmd, inConditional)
				for _, redir := range n.Redirs {
					walk(redir, inConditional)
				}
			case *syntax.CallExpr:
				for _, arg := range n.Args {
					walk(arg, inConditional)
				}
				for _, assign := range n.Assigns {
					walk(assign, inConditional)
				}
			case *syntax.Word:
				for _, part := range n.Parts {
					walk(part, inConditional)
				}
			case *syntax.CmdSubst:
				for _, stmt := range n.Stmts {
					walk(stmt, true) // Inside $(), considered conditional/dynamic
				}
			case *syntax.IfClause:
				for _, stmt := range n.Cond {
					walk(stmt, inConditional)
				}
				for _, stmt := range n.Then {
					// We could analyze if condition is strictly based on PN or PV,
					// but Devmanual says: "All eclass inherits must be unconditional, or based purely upon static machine-independent criteria (PN and PV are most common here)."
					// A simple rule might just warn on ANY conditional inherit, or we can check the Cond for PN/PV usage.
					// To be safe, we mark it conditional and let developer review it. Or we check if condition ONLY uses PV/PN.
					isStatic := false
					if len(n.Cond) == 1 {
						// a simple check, if condition contains PV or PN
						condStr := formatNode(n.Cond[0])
						if strings.Contains(condStr, "${PV}") || strings.Contains(condStr, "${PN}") {
							isStatic = true
						}
					}
					walk(stmt, inConditional || !isStatic)
				}
				if n.Else != nil {
					walk(n.Else, true)
				}
			case *syntax.Block:
				for _, stmt := range n.Stmts {
					walk(stmt, inConditional)
				}
			case *syntax.CaseClause:
				walk(n.Word, inConditional)
				isStatic := false
				wordStr := formatNode(n.Word)
				if strings.Contains(wordStr, "${PV}") || strings.Contains(wordStr, "${PN}") {
					isStatic = true
				}
				for _, item := range n.Items {
					for _, word := range item.Patterns {
						walk(word, inConditional)
					}
					for _, stmt := range item.Stmts {
						walk(stmt, inConditional || !isStatic)
					}
				}
			case *syntax.WhileClause:
				for _, stmt := range n.Cond {
					walk(stmt, inConditional)
				}
				for _, stmt := range n.Do {
					walk(stmt, true) // Loops are inherently conditional/dynamic
				}
			case *syntax.ForClause:
				walk(n.Loop, inConditional)
				for _, stmt := range n.Do {
					walk(stmt, true)
				}
			case *syntax.FuncDecl:
				walk(n.Body, true) // Inside function is dynamic/conditional
			}
		}

		walk(f, false)
	}

	return results
}

func formatNode(node syntax.Node) string {
	printer := syntax.NewPrinter()
	var buf strings.Builder
	printer.Print(&buf, node)
	return buf.String()
}
