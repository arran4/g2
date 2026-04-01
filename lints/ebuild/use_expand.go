package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

type UseExpandRule struct{}

func init() {
	rule := &UseExpandRule{}
	lints.RegisterLintRule(rule)
	lints.RegisterRuleMetadata(rule.Metadata())
}

func (r *UseExpandRule) Metadata() lints.RuleMetadata {
	return lints.RuleMetadata{
		ID:          "UseExpandUnsupported",
		Title:       "Unsupported USE_EXPAND flag usage",
		Description: "Checks if an ebuild uses USE_EXPAND flags that are not defined in the corresponding profiles/desc/*.desc file",
		Severity:    lints.SeverityError,
		Source:      "g2",
		Tags:        []string{"ebuild", "use"},
	}
}

func (r *UseExpandRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	// Load USE_EXPAND descriptions
	descDir := filepath.Join(repoDir, "profiles", "desc")
	useExpandDescs, err := g2.ParseUseExpandDescDir(descDir)
	if err != nil || len(useExpandDescs) == 0 {
		return results // No descriptions available to validate against
	}

	for _, ver := range pkgData.Versions {
		if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
			continue
		}

		iuse := ver.Ebuild.Vars["IUSE"]
		if iuse == "" {
			continue
		}

		parsedFlags := g2.ParseIUSE(iuse)
		for _, flag := range parsedFlags {
			var matchedPrefix, matchedSuffix string
			for prefix := range useExpandDescs {
				if strings.HasPrefix(flag, prefix+"_") {
					// Find the longest matching prefix
					if len(prefix) > len(matchedPrefix) {
						matchedPrefix = prefix
						matchedSuffix = strings.TrimPrefix(flag, prefix+"_")
					}
				}
			}

			if matchedPrefix != "" {
				expandName := matchedPrefix
				expandFlag := matchedSuffix

				if desc, ok := useExpandDescs[expandName]; ok {
					if _, valid := desc.Flags[expandFlag]; !valid {
						results = append(results, lints.LintResult{
							RuleMetadata: r.Metadata(),
							Message:      fmt.Sprintf("Version %s: USE_EXPAND flag '%s' uses unsupported value '%s' (not found in %s.desc)", ver.Version, flag, expandFlag, expandName),
							File:         fmt.Sprintf("%s-%s.ebuild", pkgData.Name, ver.Version),
						})
					}
				}
			}
		}
	}

	return results
}
