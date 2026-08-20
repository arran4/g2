package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleUseUnderscores = lints.RuleMetadata{
	ID:          "UseUnderscores",
	Title:       "Underscores in USE flag names",
	Description: "Underscores are reserved for USE_EXPAND flags, and must not be used within names of newly-defined regular flags.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/use-flags.html#pg0803"},
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "use", "PG0803"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUseUnderscores)
	lints.RegisterLintRule(&UseUnderscoresLintRule{})
}

type UseUnderscoresLintRule struct{}

func (r *UseUnderscoresLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *UseUnderscoresLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0803"]; ok {
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

	// Load USE_EXPAND descriptions
	descDir := filepath.Join(repoDir, "profiles", "desc")
	useExpandDescs, _ := g2.ParseUseExpandDescDir(descDir)

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			iuse := ver.Ebuild.Vars["IUSE"]
			if iuse == "" {
				continue
			}

			flags := g2.ParseIUSE(iuse)
			for _, flag := range flags {
				if strings.Contains(flag, "_") {
					isUseExpand := false
					if len(useExpandDescs) > 0 {
						for prefix := range useExpandDescs {
							if strings.HasPrefix(flag, prefix+"_") {
								isUseExpand = true
								break
							}
						}
					}

					// Architecture USE flags like linguas_* might also exist but they are usually USE_EXPAND.
					if !isUseExpand {
						res := lints.LintResult{
							RuleMetadata: ruleUseUnderscores,
							Message:      fmt.Sprintf("[Warning] Ebuild %s uses a regular USE flag '%s' containing an underscore. Underscores are reserved for USE_EXPAND flags.", ver.Version, flag),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
					}
				}
			}
		}
	}

	return results
}
