package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleUseControlledOptionalRdepend = lints.RuleMetadata{
	ID:          "UseControlledOptionalRdepend",
	Title:       "USE-Controlled Optional RDEPENDS",
	Description: "USE-controlled optional RDEPs are generally not acceptable except under very specific circumstances. A USE flag used only to pull in a runtime dependency without affecting the build should be avoided.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/dependencies.html#pg0001"},
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "rdepend", "use-flags", "PG0001"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUseControlledOptionalRdepend)
	lints.RegisterLintRule(&UseControlledOptionalRdependRule{})
}

type UseControlledOptionalRdependRule struct{}

func (r *UseControlledOptionalRdependRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *UseControlledOptionalRdependRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleUseControlledOptionalRdepend.Severity

	if qa != nil {
		if val, ok := qa.Policies["PG0001"]; ok {
			if val == "ignore" {
				return nil
			}
			switch val {
			case "error":
				severity = lints.SeverityError
			case "warning":
				severity = lints.SeverityWarning
			case "notice":
				severity = lints.SeverityNotice
			}
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			rdependStr, ok := ver.Ebuild.Vars["RDEPEND"]
			if !ok || rdependStr == "" {
				continue
			}

			// Extract USE flags from RDEPEND
			rdependFlags := r.extractUseFlags(rdependStr)
			if len(rdependFlags) == 0 {
				continue
			}

			// Check other DEPEND variables
			otherDepFlags := make(map[string]bool)
			for _, depVar := range []string{"DEPEND", "BDEPEND", "PDEPEND"} {
				if depStr, ok := ver.Ebuild.Vars[depVar]; ok && depStr != "" {
					flags := r.extractUseFlags(depStr)
					for flag := range flags {
						otherDepFlags[flag] = true
					}
				}
			}

			// Just scan the raw text for the flag usage.
			rawText := string(ver.Ebuild.RawText)

			for flag := range rdependFlags {
				// If used in another dependency var, it's likely valid
				if otherDepFlags[flag] {
					continue
				}

				// If the flag is used in the ebuild body (e.g., `use flag`, `usex flag`), it's valid
				isUsedInBody := false

				// Common use functions
				useFuncs := []string{"use", "usev", "usex", "use_with", "use_enable"}
				for _, f := range useFuncs {
					if strings.Contains(rawText, f+" "+flag) || strings.Contains(rawText, f+"\t"+flag) {
						isUsedInBody = true
						break
					}
				}

				if !isUsedInBody {
					res := lints.LintResult{
						RuleMetadata: ruleUseControlledOptionalRdepend,
						Message:      fmt.Sprintf("[%s] Ebuild %s uses USE flag '%s' purely for an optional RDEPEND. USE-controlled optional RDEPs are not acceptable except under very specific circumstances (PG0001).", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, flag),
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

func (r *UseControlledOptionalRdependRule) extractUseFlags(depStr string) map[string]bool {
	flags := make(map[string]bool)
	tokens := strings.Fields(depStr)
	for _, token := range tokens {
		if strings.HasSuffix(token, "?") {
			flag := strings.TrimSuffix(token, "?")
			// Remove any leading negation
			flag = strings.TrimPrefix(flag, "!")
			flags[flag] = true
		}
	}
	return flags
}
