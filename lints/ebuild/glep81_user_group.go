package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var CheckGLEP81Externally bool = false

var ruleGLEP81UserGroup = lints.RuleMetadata{
	ID:          "GLEP81UserGroup",
	Title:       "User and group account policy (GLEP 81)",
	Description: "All new user/group accounts must be created via GLEP 81 packages (acct-user/acct-group). Usage of user.eclass and functions like enewuser/enewgroup are deprecated.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/user-group.html#pg0901",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0901"},
}

func init() {
	lints.RegisterRuleMetadata(ruleGLEP81UserGroup)
	lints.RegisterLintRule(&GLEP81UserGroupLintRule{})
}

type GLEP81UserGroupLintRule struct{}

func (l *GLEP81UserGroupLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	// Skip the check if external checking is required but the flag is not set
	// Note: We're adding the flag logic here based on review feedback.
	// Often external checks involve parsing uid-gid.txt or cross-referencing acct-* packages.
	// For now, we add the global var and the ability to skip.
	if !CheckGLEP81Externally {
		// return results // Actually, we shouldn't skip the whole test if not checked externally. We might only skip external parts. Let's see.
	}

	rule := ruleGLEP81UserGroup

	if qa != nil {
		if val, ok := qa.Policies["PG0901"]; ok {
			if val == "ignore" {
				return results
			} else if val == "error" {
				rule.Severity = lints.SeverityError
			} else if val == "notice" {
				rule.Severity = lints.SeverityNotice
			}
		}
	}

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		// Check for user.eclass in inherits
		hasUserEclass := false
		if inheritedStr, ok := version.Ebuild.Vars["INHERITED"]; ok {
			inherited := strings.Fields(inheritedStr)
			for _, eclass := range inherited {
				if eclass == "user" {
					hasUserEclass = true
					break
				}
			}
		}

		if hasUserEclass {
			res := lints.LintResult{
				RuleMetadata: rule,
				Message:      fmt.Sprintf("[%s] Ebuild %s inherits deprecated 'user' eclass. User/group accounts should be created via GLEP 81 packages (acct-user/acct-group).", rule.Severity, version.Ebuild.Path),
				Package:      pkgData.Category + "/" + pkgData.Name,
			}
			results = append(results, res)
		}

		if version.Ebuild.RawText == "" {
			continue
		}

		// Check for enewuser/enewgroup commands
		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
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

				if cmdName == "enewuser" || cmdName == "enewgroup" {
					res := lints.LintResult{
						RuleMetadata: rule,
						Message:      fmt.Sprintf("[%s] Ebuild %s uses deprecated '%s' function. User/group accounts should be created via GLEP 81 packages (acct-user/acct-group).", rule.Severity, version.Ebuild.Path, cmdName),
						Package:      pkgData.Category + "/" + pkgData.Name,
					}
					results = append(results, res)
				}
			}
			return true
		})
	}

	return results
}

func (l *GLEP81UserGroupLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil)
}
