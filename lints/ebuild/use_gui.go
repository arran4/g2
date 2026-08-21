package ebuild

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleUseGui = lints.RuleMetadata{
	ID:          "UseGui",
	Title:       "USE=gui flag",
	Description: "Whenever a package offers an optional GUI support, the gui flag must be used to control that support rather than historically used X or toolkit flags (like gtk, qt4, qt5). Toolkit flags can still be used to choose between multiple available GUIs, but gui is preferred.",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/use-flags.html#pg0802", Label: "Gentoo QA Policy Guide PG0802"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "use", "PG0802"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUseGui)
	lints.RegisterLintRule(&UseGuiLintRule{})
}

type UseGuiLintRule struct{}

func (r *UseGuiLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *UseGuiLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0802"]; ok {
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

	historicToolkitFlags := map[string]bool{
		"X":     true,
		"gtk":   true,
		"gtk2":  true,
		"gtk3":  true,
		"gtk4":  true,
		"qt4":   true,
		"qt5":   true,
		"qt6":   true,
		"motif": true,
		"fltk":  true,
		"tk":    true,
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			iuse := ver.Ebuild.Vars["IUSE"]
			if iuse == "" {
				continue
			}

			flags := g2.ParseIUSE(iuse)
			flagSet := make(map[string]bool)
			for _, flag := range flags {
				flagSet[flag] = true
			}

			if !flagSet["gui"] {
				for flag := range flagSet {
					if historicToolkitFlags[flag] {
						res := lints.LintResult{
							RuleMetadata: ruleUseGui,
							Message:      fmt.Sprintf("[Warning] Ebuild %s uses historic toolkit flag '%s' but lacks the 'gui' flag. The 'gui' flag must be used to control optional GUI support.", ver.Version, flag),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
						// Only one warning per ebuild is enough to complain about the lack of 'gui'
						break
					}
				}
			}
		}
	}

	return results
}
