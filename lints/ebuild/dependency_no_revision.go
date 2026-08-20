package ebuild

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleDependencyNoRevision = lints.RuleMetadata{
	ID:          "DependencyNoRevision",
	Title:       "=-dependencies with no revision",
	Description: "Whenever a non-wildcard = (equals) dependency is used on a package, the requested revision must be specified explicitly.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/dependencies.html#pg0002",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "dependencies", "pg0002"},
}

func init() {
	lints.RegisterRuleMetadata(ruleDependencyNoRevision)
	lints.RegisterLintRule(&DependencyNoRevisionLintRule{})
}

type DependencyNoRevisionLintRule struct{}

func (r *DependencyNoRevisionLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *DependencyNoRevisionLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult
	severity := string(ruleDependencyNoRevision.Severity)
	if len(severity) > 0 {
		severity = strings.ToUpper(severity[:1]) + severity[1:]
	}
	revisionRe := regexp.MustCompile(`-r\d+$`)

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			for _, depVar := range []string{"DEPEND", "RDEPEND", "PDEPEND", "BDEPEND", "IDEPEND"} {
				depStr, ok := ver.Ebuild.Vars[depVar]
				if !ok || depStr == "" {
					continue
				}

				walkDependencies(depStr, func(atomStr string) {
					atom := g2.ParsePackageAtom(atomStr)

					if atom.Operator == "=" && !strings.Contains(atomStr, "*") {
						if !revisionRe.MatchString(atom.Version) {
							res := lints.LintResult{
								RuleMetadata: ruleDependencyNoRevision,
								Message:      fmt.Sprintf("[%s] Ebuild %s uses a non-wildcard '=' dependency without an explicit revision in %s: %s", severity, ver.Version, depVar, atomStr),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							results = append(results, res)
						}
					}
				})
			}
		}
	}
	return results
}

func walkDependencies(depStr string, cb func(atomStr string)) {
	tokens := strings.Fields(depStr)
	var stateStack []string
	pendingAnyOf := false

	for _, token := range tokens {
		if token == "||" {
			pendingAnyOf = true
		} else if token == "(" {
			if pendingAnyOf {
				stateStack = append(stateStack, "||")
				pendingAnyOf = false
			} else {
				stateStack = append(stateStack, "OTHER")
			}
		} else if token == ")" {
			if len(stateStack) > 0 {
				stateStack = stateStack[:len(stateStack)-1]
			}
		} else if strings.HasSuffix(token, "?") {
			pendingAnyOf = false
		} else {
			cb(token)
			pendingAnyOf = false
		}
	}
}
