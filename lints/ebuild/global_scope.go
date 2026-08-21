package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleGlobalScope = lints.RuleMetadata{
	ID:          "GlobalScope",
	Title:       "Invalid commands in global scope",
	Description: "Warns about using external commands like sed, awk, grep, has_version, etc. in global scope.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/appendices/common-problems/index.html#qa-notice-foo-in-global-scope", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy", "qa"},
}

func init() {
	lints.RegisterRuleMetadata(ruleGlobalScope)
	lints.RegisterLintRule(&GlobalScopeLintRule{})
}

type GlobalScopeLintRule struct{}

func (l *GlobalScopeLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	disallowedCommands := map[string]bool{
		"sed":          true,
		"awk":          true,
		"grep":         true,
		"egrep":        true,
		"cut":          true,
		"has_version":  true,
		"best_version": true,
		"python":       true,
		"perl":         true,
	}

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		var walkGlobal func(node syntax.Node)
		walkGlobal = func(node syntax.Node) {
			if node == nil {
				return
			}

			switch n := node.(type) {
			case *syntax.FuncDecl:
				// Do not walk inside function body
				return
			case *syntax.CallExpr:
				if len(n.Args) > 0 {
					var cmdName string
					if len(n.Args[0].Parts) == 1 {
						if lit, ok := n.Args[0].Parts[0].(*syntax.Lit); ok {
							cmdName = lit.Value
						}
					}
					if cmdName != "" && disallowedCommands[cmdName] {
						res := lints.LintResult{
							RuleMetadata: ruleGlobalScope,
							Message:      fmt.Sprintf("[Error] Ebuild %s calls %s in global scope. This is not allowed.", version.Ebuild.Path, cmdName),
							Package:      pkgData.Category + "/" + pkgData.Name,
						}
						results = append(results, res)
					}
				}
			}

			// Continue walking down
			switch n := node.(type) {
			case *syntax.File:
				for _, stmt := range n.Stmts {
					walkGlobal(stmt)
				}
			case *syntax.Stmt:
				walkGlobal(n.Cmd)
				for _, redir := range n.Redirs {
					walkGlobal(redir)
				}
			case *syntax.CallExpr:
				for _, arg := range n.Args {
					walkGlobal(arg)
				}
				for _, assign := range n.Assigns {
					walkGlobal(assign)
				}
			case *syntax.Word:
				for _, part := range n.Parts {
					walkGlobal(part)
				}
			case *syntax.CmdSubst:
				for _, stmt := range n.Stmts {
					walkGlobal(stmt)
				}
			case *syntax.IfClause:
				for _, stmt := range n.Cond {
					walkGlobal(stmt)
				}
				for _, stmt := range n.Then {
					walkGlobal(stmt)
				}
				if n.Else != nil {
					walkGlobal(n.Else)
				}
			case *syntax.Block:
				for _, stmt := range n.Stmts {
					walkGlobal(stmt)
				}
			case *syntax.Assign:
				if n.Value != nil {
					walkGlobal(n.Value)
				}
				if n.Array != nil {
					walkGlobal(n.Array)
				}
			case *syntax.ArrayExpr:
				for _, elem := range n.Elems {
					walkGlobal(elem)
				}
			case *syntax.ArrayElem:
				walkGlobal(n.Value)
			case *syntax.DeclClause:
				for _, assign := range n.Args {
					walkGlobal(assign)
				}
			case *syntax.CaseClause:
				walkGlobal(n.Word)
				for _, item := range n.Items {
					for _, word := range item.Patterns {
						walkGlobal(word)
					}
					for _, stmt := range item.Stmts {
						walkGlobal(stmt)
					}
				}
			case *syntax.WhileClause:
				for _, stmt := range n.Cond {
					walkGlobal(stmt)
				}
				for _, stmt := range n.Do {
					walkGlobal(stmt)
				}
			case *syntax.ForClause:
				walkGlobal(n.Loop)
				for _, stmt := range n.Do {
					walkGlobal(stmt)
				}
			case *syntax.WordIter:
				for _, word := range n.Items {
					walkGlobal(word)
				}
			case *syntax.Subshell:
				for _, stmt := range n.Stmts {
					walkGlobal(stmt)
				}
			case *syntax.BinaryCmd:
				walkGlobal(n.X)
				walkGlobal(n.Y)
			case *syntax.CoprocClause:
				walkGlobal(n.Stmt)
			case *syntax.TimeClause:
				walkGlobal(n.Stmt)
			case *syntax.LetClause:
				for _, expr := range n.Exprs {
					walkGlobal(expr)
				}
			case *syntax.TestClause:
				walkGlobal(n.X)
			case *syntax.BinaryTest:
				walkGlobal(n.X)
				walkGlobal(n.Y)
			case *syntax.UnaryTest:
				walkGlobal(n.X)
			case *syntax.ParenTest:
				walkGlobal(n.X)
			}
		}

		walkGlobal(f)
	}

	return results
}
