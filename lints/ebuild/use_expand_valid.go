package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleUseExpandValid = lints.RuleMetadata{
	ID:          "UseExpandValid",
	Title:       "USE_EXPAND Flags Valid",
	Description: "Verifies that USE flags that look like USE_EXPAND flags (prefix_flag) are documented in the corresponding profiles/desc/<prefix>.desc file.",
	URL:         "https://devmanual.gentoo.org/general-concepts/use-flags/",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "site-quality", "use-expand"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUseExpandValid)
	lints.RegisterLintRule(&UseExpandLintRule{})
}

type UseExpandLintRule struct{
	parsedDescs map[string]map[string]*g2.UseExpandDesc // repoDir -> UseExpandDescs
}

func (r *UseExpandLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *UseExpandLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0702"]; ok { // Assign a different PG code or use a generic one? I'll just use the severity logic.
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

	if r.parsedDescs == nil {
		r.parsedDescs = make(map[string]map[string]*g2.UseExpandDesc)
	}

	useExpandDescs, ok := r.parsedDescs[repoDir]
	if !ok {
		useExpandDir := filepath.Join(repoDir, "profiles", "desc")
		descs, err := g2.ParseUseExpandDescDir(useExpandDir)
		if err != nil {
			descs = make(map[string]*g2.UseExpandDesc) // cache empty map to avoid re-parsing on error
		}
		r.parsedDescs[repoDir] = descs
		useExpandDescs = descs
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			var flagsToCheck []string
			if iuse := ver.Ebuild.Vars["IUSE"]; iuse != "" {
				flags := strings.Fields(iuse)
				for _, flag := range flags {
					flag = strings.TrimPrefix(flag, "+")
					flag = strings.TrimPrefix(flag, "-")
					if flag != "" {
						flagsToCheck = append(flagsToCheck, flag)
					}
				}
			}

			// Add flags from REQUIRED_USE
			if requiredUse := ver.Ebuild.Vars["REQUIRED_USE"]; requiredUse != "" {
				tree := g2.ParseDepTree(requiredUse)
				vals, _ := tree.Evaluate(g2.IgnoreUseFlags(true))
				for _, v := range vals {
					if v != "(" && v != ")" && v != "||" && v != "^^" && v != "??" && !strings.HasSuffix(v, "?") {
						v = strings.TrimPrefix(v, "!")
						flagsToCheck = append(flagsToCheck, v)
					}
				}
			}

			// Add flags from depend variables that are used as conditionals
			depVars := []string{"DEPEND", "RDEPEND", "BDEPEND", "PDEPEND"}
			for _, depVar := range depVars {
				if depStr := ver.Ebuild.Vars[depVar]; depStr != "" {
					// We need to parse the dependency tree to find conditionals, but g2.ParseDepTree doesn't easily expose the conditional node structure directly in a way to list *only* conditional flags. We can just use strings.Fields and check for suffix '?'.
					tokens := strings.Fields(depStr)
					for _, token := range tokens {
						if strings.HasSuffix(token, "?") {
							flag := strings.TrimSuffix(token, "?")
							flag = strings.TrimPrefix(flag, "!") // handle !flag?
							flagsToCheck = append(flagsToCheck, flag)
						}
					}
				}
			}

			// Deduplicate flags
			uniqueFlags := make(map[string]bool)
			for _, flag := range flagsToCheck {
				if !uniqueFlags[flag] {
					uniqueFlags[flag] = true

					// Check if flag matches any known prefix
					var matchedPrefixes []string
					for prefix := range useExpandDescs {
						if strings.HasPrefix(flag, prefix+"_") {
							matchedPrefixes = append(matchedPrefixes, prefix)
						}
					}

					if len(matchedPrefixes) > 0 {
						foundValid := false
						var checkList []string
						for _, prefix := range matchedPrefixes {
							suffix := strings.TrimPrefix(flag, prefix+"_")
							if _, exists := useExpandDescs[prefix].Flags[suffix]; exists {
								foundValid = true
								break
							}
							checkList = append(checkList, prefix)
						}

						if !foundValid {
							res := lints.LintResult{
								RuleMetadata: ruleUseExpandValid,
								Message:      fmt.Sprintf("[%s] USE_EXPAND flag '%s' (matches prefixes %s) in ebuild %s is not documented in corresponding .desc files", cases.Title(language.English).String(string(severity)), flag, strings.Join(checkList, ", "), ver.Version),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				}
			}
		}
	}

	return results
}
