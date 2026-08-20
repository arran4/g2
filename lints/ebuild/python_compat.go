package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	rulePythonCompatPG0501 = lints.RuleMetadata{
		ID:          "PythonCompatPG0501",
		Title:       "Missing PYTHON_COMPAT (PG0501)",
		Description: "All ebuilds using Python must explicitly define supported Python implementations in PYTHON_COMPAT.",
		URL:         "https://projects.gentoo.org/qa/policy-guide/languages.html#pg0501",
		Severity:    lints.SeverityError,
		Source:      lints.SourceQA,
		Tags:        []string{"ebuild", "gentoo-policy", "PG0501"},
	}

	rulePythonCompatPG0502 = lints.RuleMetadata{
		ID:          "PythonCompatPG0502",
		Title:       "Python 2 usage (PG0502)",
		Description: "Python 2 is being phased out. Python 2 support should not be introduced or maintained where possible.",
		URL:         "https://projects.gentoo.org/qa/policy-guide/languages.html#pg0502",
		Severity:    lints.SeverityWarning,
		Source:      lints.SourceQA,
		Tags:        []string{"ebuild", "gentoo-policy", "PG0502"},
	}
)

func init() {
	lints.RegisterRuleMetadata(rulePythonCompatPG0501)
	lints.RegisterRuleMetadata(rulePythonCompatPG0502)
	lints.RegisterLintRule(&PythonCompatLintRule{})
}

type PythonCompatLintRule struct{}

func (r *PythonCompatLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *PythonCompatLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	severity0501 := rulePythonCompatPG0501.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0501"]; ok {
			if val == "ignore" {
				// Don't report PG0501
				severity0501 = ""
			} else if val == "notice" {
				severity0501 = lints.SeverityNotice
			} else if val == "warning" {
				severity0501 = lints.SeverityWarning
			} else if val == "error" {
				severity0501 = lints.SeverityError
			}
		}
	}

	severity0502 := rulePythonCompatPG0502.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0502"]; ok {
			if val == "ignore" {
				severity0502 = ""
			} else if val == "notice" {
				severity0502 = lints.SeverityNotice
			} else if val == "warning" {
				severity0502 = lints.SeverityWarning
			} else if val == "error" {
				severity0502 = lints.SeverityError
			}
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			inherited := ver.Ebuild.Vars["INHERITED"]
			inheritsPython := false
			for _, eclass := range strings.Fields(inherited) {
				if strings.HasPrefix(eclass, "python-") {
					inheritsPython = true
					break
				}
			}

			if inheritsPython {
				pythonCompat, hasPythonCompat := ver.Ebuild.Vars["PYTHON_COMPAT"]

				if !hasPythonCompat && severity0501 != "" {
					res := lints.LintResult{
						RuleMetadata: rulePythonCompatPG0501,
						Message:      fmt.Sprintf("[%s] Ebuild %s inherits a python eclass but does not define PYTHON_COMPAT (PG0501).", cases.Title(language.Und, cases.NoLower).String(string(severity0501)), ver.Version),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity0501
					results = append(results, res)
				}

				if hasPythonCompat && severity0502 != "" {
					for _, compat := range strings.Fields(pythonCompat) {
						if strings.HasPrefix(compat, "python2_") || strings.HasPrefix(compat, "pypy2") {
							res := lints.LintResult{
								RuleMetadata: rulePythonCompatPG0502,
								Message:      fmt.Sprintf("[%s] Ebuild %s defines Python 2 compatibility '%s' in PYTHON_COMPAT (PG0502).", cases.Title(language.Und, cases.NoLower).String(string(severity0502)), ver.Version, compat),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity0502
							results = append(results, res)
							break
						}
					}
				}
			}
		}
	}

	return results
}
