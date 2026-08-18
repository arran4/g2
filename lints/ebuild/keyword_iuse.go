package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleKeywordIUSE = lints.RuleMetadata{
	ID:          "KeywordIUSE",
	Title:       "Keyword in IUSE",
	Description: "Checks if a keyword is used in IUSE or `use <keyword>` checks.",
	Severity:    lints.SeverityWarning,
	Source:      "g2",
	Tags:        []string{"ebuild", "use", "keywords"},
}

func init() {
	lints.RegisterRuleMetadata(ruleKeywordIUSE)
	lints.RegisterLintRule(&KeywordIUSELintRule{})
}

type KeywordIUSELintRule struct{}

func (r *KeywordIUSELintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *KeywordIUSELintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	// Get arches list to know the keywords
	archesFile := filepath.Join(repoDir, "profiles", "arch.list")
	arches, err := g2.ParseArchListFile(archesFile)
	if err != nil {
		return results
	}

	knownArches := make(map[string]bool)
	for _, arch := range arches.Arches {
		knownArches[arch] = true
		knownArches["~"+arch] = true
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
			continue
		}

		iuse := ver.Ebuild.Vars["IUSE"]
		if iuse != "" {
			parsedFlags := g2.ParseIUSE(iuse)
			for _, flag := range parsedFlags {
				if knownArches[flag] {
					results = append(results, lints.LintResult{
						RuleMetadata: ruleKeywordIUSE,
						Message:      fmt.Sprintf("Version %s: Architecture keyword '%s' found in IUSE", ver.Version, flag),
						File:         fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version),
					})
				}
			}
		}

		// Also check for `use <keyword>` and `usex <keyword>` etc.
		if ver.Ebuild.RawText != "" {
			f, err := syntax.NewParser().Parse(strings.NewReader(ver.Ebuild.RawText), "")
			if err == nil {
				syntax.Walk(f, func(node syntax.Node) bool {
					call, ok := node.(*syntax.CallExpr)
					if ok && len(call.Args) > 1 {
						cmd := call.Args[0].Lit()
						if cmd == "use" || cmd == "usex" || cmd == "use_with" || cmd == "use_enable" || cmd == "useq" || cmd == "usev" {
							arg := call.Args[1].Lit()
							if knownArches[arg] {
								results = append(results, lints.LintResult{
									RuleMetadata: ruleKeywordIUSE,
									Message:      fmt.Sprintf("Version %s: Architecture keyword '%s' used in `%s %s`", ver.Version, arg, cmd, arg),
									File:         fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version),
								})
							}
						}
					}
					return true
				})
			}
		}
	}

	return results
}
