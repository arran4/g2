package ebuild

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var rulePythonSingleR1UseDep = lints.RuleMetadata{
	ID:          "PythonSingleR1UseDep",
	Title:       "Invalid PYTHON_USEDEP with python-single-r1",
	Description: "inherit python-single-r1 and PYTHON_USEDEP shouldn't be used together without python_gen_cond_dep.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/eclass-reference/python-single-r1.eclass/index.html", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy"},
}

var (
	useDepRe           = regexp.MustCompile(`\$\{?PYTHON_USEDEP\}?`)
	pythonGenCondDepRe = regexp.MustCompile(`\$\(\s*python_gen_cond_dep\b`)
)

func init() {
	lints.RegisterRuleMetadata(rulePythonSingleR1UseDep)
	lints.RegisterLintRule(&PythonSingleR1UseDepLintRule{})
}

type PythonSingleR1UseDepLintRule struct{}

func (r *PythonSingleR1UseDepLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *PythonSingleR1UseDepLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PythonSingleR1UseDep"]; ok {
			if val == "ignore" {
				return nil
			}
			if val == "notice" || val == "error" || val == "warning" {
				switch val {
				case "notice":
					severity = lints.SeverityNotice
				case "error":
					severity = lints.SeverityError
				case "warning":
					severity = lints.SeverityWarning
				}
			}
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			inherited := ver.Ebuild.Vars["INHERITED"]
			inheritsPythonSingleR1 := false
			for _, eclass := range strings.Fields(inherited) {
				if eclass == "python-single-r1" {
					inheritsPythonSingleR1 = true
					break
				}
			}

			if inheritsPythonSingleR1 && ver.Ebuild.RawText != "" {
				// Strip $(python_gen_cond_dep ...) to check if PYTHON_USEDEP is used incorrectly outside
				strippedText := stripPythonGenCondDep(ver.Ebuild.RawText)

				if useDepRe.MatchString(strippedText) {
					res := lints.LintResult{
						RuleMetadata: rulePythonSingleR1UseDep,
						Message:      fmt.Sprintf("[%s] Ebuild %s inherits python-single-r1 and uses PYTHON_USEDEP without python_gen_cond_dep.", severity, ver.Version),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}
			}
		}
	}

	return results
}

func stripPythonGenCondDep(val string) string {
	for {
		loc := pythonGenCondDepRe.FindStringIndex(val)
		if loc == nil {
			break
		}

		idx := loc[0]

		// find matching parenthesis
		parenCount := 0
		endIdx := idx
		for i := idx; i < len(val); i++ {
			if val[i] == '(' {
				parenCount++
			} else if val[i] == ')' {
				parenCount--
				if parenCount == 0 {
					endIdx = i
					break
				}
			}
		}

		if endIdx == idx { // Unbalanced or unexpected
			break
		}

		val = val[:idx] + val[endIdx+1:]
	}
	return val
}
