package ebuild

import (
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleNetworkSandbox = lints.RuleMetadata{
	ID:          "NetworkSandbox",
	Title:       "Network Sandbox Bypass",
	Description: "Checks if a package bypasses the network sandbox using RESTRICT=\"network-sandbox\".",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "network-sandbox"},
}

func init() {
	lints.RegisterRuleMetadata(ruleNetworkSandbox)
	lints.RegisterLintRule(&NetworkSandboxLintRule{})
}

type NetworkSandboxLintRule struct{}

func (r *NetworkSandboxLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *NetworkSandboxLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			restrict := ver.Ebuild.Vars["RESTRICT"]
			for _, r := range strings.Fields(restrict) {
				if r == "network-sandbox" {
					res := lints.LintResult{
						RuleMetadata: ruleNetworkSandbox,
						Message:      "Bypassing the network sandbox using `RESTRICT=\"network-sandbox\"` to download dependencies via `npm` during `src_install` violates Gentoo's core packaging policies. Ebuilds must support offline and reproducible builds, meaning all assets must be fetched via `SRC_URI` during the fetch phase and verified using Manifest checksums. To resolve this, you should package the required node modules into a tarball, host it, reference it in `SRC_URI`, and perform an offline installation.",
						Package:      pkg.Category + "/" + pkg.Name,
					}
					results = append(results, res)
					break
				}
			}
		}
	}
	return results
}
