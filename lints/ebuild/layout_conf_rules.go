package ebuild

import (
	"fmt"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&LayoutConfLintRule{})
}

type LayoutConfLintRule struct{}

func (r *LayoutConfLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *LayoutConfLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string

	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	lc, err := g2.ParseLayoutConf(layoutConfPath)
	if err != nil {
		// Cannot lint without layout.conf or if it doesn't exist. Assuming valid or not our place to report here.
		return warnings
	}

	eapisBanned := append(lc.GetValuesAsSlice("eapis-banned"), lc.GetValuesAsSlice("profile-eapis-banned")...)
	eapisDeprecated := append(lc.GetValuesAsSlice("eapis-deprecated"), lc.GetValuesAsSlice("profile-eapis-deprecated")...)
	restrictAllowed := lc.GetValuesAsSlice("restrict-allowed")
	propertiesAllowed := lc.GetValuesAsSlice("properties-allowed")

	isBanned := func(eapi string, banned []string) bool {
		for _, b := range banned {
			if b == eapi {
				return true
			}
		}
		return false
	}

	isAllowed := func(val string, allowed []string) bool {
		if len(allowed) == 0 {
			return true // If not specified, anything is allowed
		}
		for _, a := range allowed {
			if a == val || val == "" {
				return true
			}
		}
		return false
	}

	checkTokens := func(field string, rawValue string, allowed []string) []string {
		var errs []string
		tree := g2.ParseDepTree(rawValue)
		tokens, _ := tree.Evaluate(g2.IgnoreUseFlags(true))
		for _, token := range tokens {
			if !isAllowed(token, allowed) {
				errs = append(errs, fmt.Sprintf("unallowed %s token '%s'", field, token))
			}
		}
		return errs
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			eapi := ver.Ebuild.Vars["EAPI"]
			if eapi == "" {
				eapi = "0"
			}

			if isBanned(eapi, eapisBanned) {
				warnings = append(warnings, fmt.Sprintf("[Error] Ebuild %s uses banned EAPI '%s'", ver.Version, eapi))
			} else if isBanned(eapi, eapisDeprecated) {
				warnings = append(warnings, fmt.Sprintf("[Warning] Ebuild %s uses deprecated EAPI '%s'", ver.Version, eapi))
			}

			if len(restrictAllowed) > 0 {
				restrict := ver.Ebuild.Vars["RESTRICT"]
				if errs := checkTokens("RESTRICT", restrict, restrictAllowed); len(errs) > 0 {
					for _, e := range errs {
						warnings = append(warnings, fmt.Sprintf("[Error] Ebuild %s: %s", ver.Version, e))
					}
				}
			}

			if len(propertiesAllowed) > 0 {
				properties := ver.Ebuild.Vars["PROPERTIES"]
				if errs := checkTokens("PROPERTIES", properties, propertiesAllowed); len(errs) > 0 {
					for _, e := range errs {
						warnings = append(warnings, fmt.Sprintf("[Error] Ebuild %s: %s", ver.Version, e))
					}
				}
			}
		}
	}

	return warnings
}
