package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleHeadTail = lints.RuleMetadata{
	ID:          "HeadAndTailUsage",
	Title:       "Head and tail usage",
	Description: "Checks for improper or deprecated use of head and tail commands, such as deprecated syntax, clumsy line counting, and unnecessary chaining with sed.",
	URLs:        []string{"https://devmanual.gentoo.org/tools-reference/head-and-tail/index.html"},
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "qa", "tools"},
}

func init() {
	lints.RegisterRuleMetadata(ruleHeadTail)
	lints.RegisterLintRule(&HeadTailLintRule{})
}

type HeadTailLintRule struct{}

func (l *HeadTailLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
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
			// Check for deprecated syntax e.g. head -5
			if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 0 {
				if lit, ok := call.Args[0].Parts[0].(*syntax.Lit); ok {
					cmd := lit.Value
					if cmd == "head" || cmd == "tail" {
						for _, arg := range call.Args {
							if len(arg.Parts) > 0 {
								if argLit, ok := arg.Parts[0].(*syntax.Lit); ok {
									if strings.HasPrefix(argLit.Value, "-") && len(argLit.Value) > 1 && argLit.Value[1] >= '0' && argLit.Value[1] <= '9' {
										res := lints.LintResult{
											RuleMetadata: ruleHeadTail,
											Message:      fmt.Sprintf("%s -N syntax is deprecated and not POSIX compliant, use %s -n N instead.", cmd, cmd),
											Package:      pkgData.Category + "/" + pkgData.Name,
										}
										results = append(results, res)
									}
								}
							}
						}

						// Check for clumsily computing line count for tail
						if cmd == "tail" {
							for _, arg := range call.Args {
								hasWc := false
								syntax.Walk(arg, func(n syntax.Node) bool {
									if c, ok := n.(*syntax.CallExpr); ok && len(c.Args) > 0 {
										if l, ok := c.Args[0].Parts[0].(*syntax.Lit); ok {
											if l.Value == "wc" {
												hasWc = true
											}
										}
									}
									return true
								})
								if hasWc {
									// Also need to check if it's inside a command substitution or arithmetic expansion
									isComputing := false
									syntax.Walk(arg, func(n syntax.Node) bool {
										if _, ok := n.(*syntax.CmdSubst); ok {
											isComputing = true
										}
										if _, ok := n.(*syntax.ArithmExp); ok {
											isComputing = true
										}
										return true
									})
									if isComputing {
										res := lints.LintResult{
											RuleMetadata: ruleHeadTail,
											Message:      "Clumsily computing line count for tail, use tail -n +X instead.",
											Package:      pkgData.Category + "/" + pkgData.Name,
										}
										results = append(results, res)
									}
								}
							}
						}
					}
				}
			}

			// Check for chaining head or tail with sed
			if binaryCmd, ok := node.(*syntax.BinaryCmd); ok {
				if binaryCmd.Op == syntax.Pipe {
					leftCmd := getBaseCommand(binaryCmd.X)
					rightCmd := getBaseCommand(binaryCmd.Y)

					if (leftCmd == "head" || leftCmd == "tail") && rightCmd == "sed" {
						res := lints.LintResult{
							RuleMetadata: ruleHeadTail,
							Message:      "Chaining head or tail with sed is usually unnecessary. Use of addresses and early exit can do the same thing with a single sed call.",
							Package:      pkgData.Category + "/" + pkgData.Name,
						}
						results = append(results, res)
					}
					if leftCmd == "sed" && (rightCmd == "head" || rightCmd == "tail") {
						res := lints.LintResult{
							RuleMetadata: ruleHeadTail,
							Message:      "Chaining head or tail with sed is usually unnecessary. Use of addresses and early exit can do the same thing with a single sed call.",
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

func getBaseCommand(node syntax.Node) string {
	var cmdName string
	syntax.Walk(node, func(n syntax.Node) bool {
		if cmdName != "" {
			return false
		}

		if call, ok := n.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			if len(call.Args[0].Parts) > 0 {
				if lit, ok := call.Args[0].Parts[0].(*syntax.Lit); ok {
					cmdName = lit.Value
					return false
				}
			}
		}

		if _, ok := n.(*syntax.Subshell); ok {
			return false
		}
		if _, ok := n.(*syntax.CmdSubst); ok {
			return false
		}

		return true
	})
	return cmdName
}
