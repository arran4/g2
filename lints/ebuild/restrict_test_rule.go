package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleRestrictTest = lints.RuleMetadata{
	ID:          "RestrictTest",
	Title:       "RESTRICT=test for USE=-test",
	Description: "Whenever the package uses test flag to control test prerequisites (or another flag with a similar purpose), it must explicitly restrict tests when the flag is unset.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/other-metadata.html#pg0703"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0703"},
}

func init() {
	lints.RegisterRuleMetadata(ruleRestrictTest)
	lints.RegisterLintRule(&RestrictTestLintRule{})
}

type RestrictTestLintRule struct{}

func (r *RestrictTestLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func hasRestrictTestForUnsetTest(restrictStr string) bool {
	tokens := strings.Fields(restrictStr)
	var conditions []string

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if strings.HasSuffix(token, "?") {
			cond := token
			if i+1 < len(tokens) && tokens[i+1] == "(" {
				conditions = append(conditions, cond)
				i++ // skip "("
			} else {
				// single token
				if i+1 < len(tokens) {
					if tokens[i+1] == "test" {
						if cond == "!test?" || len(conditions) == 0 {
                            if cond == "!test?" {
                                return true
                            }
						}
					}
					i++ // consume the token
				}
			}
		} else if token == ")" {
			if len(conditions) > 0 {
				conditions = conditions[:len(conditions)-1]
			}
		} else if token == "test" {
			if len(conditions) == 0 {
				return true // Unconditional restrict
			}
			for _, c := range conditions {
				if c == "!test?" {
					return true
				}
			}
		}
	}
	return false
}

func (r *RestrictTestLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleRestrictTest.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0703"]; ok {
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
			iuseStr := ver.Ebuild.Vars["IUSE"]
			if !strings.Contains(iuseStr, "test") {
				continue
			}

            // Check if test is actually in IUSE (accounting for +test or -test)
            hasTestIUSE := false
            for _, token := range strings.Fields(iuseStr) {
                if token == "test" || token == "+test" || token == "-test" {
                    hasTestIUSE = true
                    break
                }
            }

            if !hasTestIUSE {
                continue
            }

			restrictStr := ver.Ebuild.Vars["RESTRICT"]
			if !hasRestrictTestForUnsetTest(restrictStr) {
				res := lints.LintResult{
					RuleMetadata: ruleRestrictTest,
					Message:      fmt.Sprintf("[%s] Ebuild %s has 'test' in IUSE but does not have RESTRICT=\"!test? ( test )\"", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
