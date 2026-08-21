package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleDependencyPitfalls = lints.RuleMetadata{
	ID:          "DependencyPitfalls",
	Title:       "Dependency Pitfalls",
	Description: "Checks for common dependency pitfalls such as := inside any-of groups or weak blockers in DEPEND.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/general-concepts/dependencies/index.html#common-pitfalls", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityWarning, // Usually warnings
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy"},
}

func init() {
	lints.RegisterRuleMetadata(ruleDependencyPitfalls)
	lints.RegisterLintRule(&DependencyPitfallsLintRule{})
}

type DependencyPitfallsLintRule struct{}

func (r *DependencyPitfallsLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *DependencyPitfallsLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleDependencyPitfalls.Severity

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			// Check DEPEND, RDEPEND, PDEPEND, BDEPEND
			for _, depVar := range []string{"DEPEND", "RDEPEND", "PDEPEND", "BDEPEND"} {
				depStr, ok := ver.Ebuild.Vars[depVar]
				if !ok || depStr == "" {
					continue
				}

				r.walkDependencies(depStr, func(atomStr string, insideAnyOf bool) {
					atom := g2.ParsePackageAtom(atomStr)

					// Pitfall 1: := slot operator inside any-of groups (|| ( ... ))
					// Warning: Do not place := dependency specifications inside || ( ... ) groups.
					if insideAnyOf && atom.Slot != "" && strings.HasSuffix(atom.Slot, "=") {
						res := lints.LintResult{
							RuleMetadata: ruleDependencyPitfalls,
							Message:      fmt.Sprintf("[%s] Ebuild %s has ':=' slot operator inside an any-of group (||) in %s: %s", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, depVar, atomStr),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						results = append(results, res)
					}

					// Pitfall 2: Weak blockers in DEPEND.
					// Weak blockers (!) are primarily meaningful in RDEPEND.
					// "Remember the general caveat from above: weak blockers should be included in RDEPEND rather than used purely in DEPEND."
					// Actually, "Weak blockers that are pure DEPEND do not work correctly. Always include your weak blockers in RDEPEND!"
					// The rule is typically: weak blocker in DEPEND is bad, unless it's a strong blocker (!!)
					if depVar == "DEPEND" && atom.Operator == "!" {
						res := lints.LintResult{
							RuleMetadata: ruleDependencyPitfalls,
							Message:      fmt.Sprintf("[%s] Ebuild %s uses a weak blocker (!) purely in DEPEND. Use strong blockers (!!) in DEPEND or weak blockers in RDEPEND: %s", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, atomStr),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						results = append(results, res)
					}
				})
			}
		}
	}
	return results
}

func (r *DependencyPitfallsLintRule) walkDependencies(depStr string, cb func(atomStr string, insideAnyOf bool)) {
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
			// It's a USE flag condition, ignored
			pendingAnyOf = false
		} else {
			// This is a package atom
			inAnyOf := false
			for _, state := range stateStack {
				if state == "||" {
					inAnyOf = true
					break
				}
			}
			cb(token, inAnyOf)
			pendingAnyOf = false
		}
	}
}
