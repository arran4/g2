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

var ruleLayoutConf = lints.RuleMetadata{
	ID:          "LayoutConfRules",
	Title:       "Layout Conf Checks",
	Description: "Validates layout.conf elements like manifest-hashes and eapis-deprecated.",
	URL:         "https://devmanual.gentoo.org/general-concepts/overlay-layout/",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"repo-layout", "manifest"},
}

func init() {
	lints.RegisterRuleMetadata(ruleLayoutConf)
	lints.RegisterLintRule(&LayoutConfLintRule{})
}

type LayoutConfLintRule struct{}

func (r *LayoutConfLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *LayoutConfLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	lc, err := g2.ParseLayoutConf(layoutConfPath)
	if err != nil {
		return results
	}

	eapisDeprecated := lc.GetValuesAsSlice("eapis-deprecated")
	eapisBanned := lc.GetValuesAsSlice("eapis-banned")
	propertiesAllowed := lc.GetValuesAsSlice("properties-allowed")
	restrictAllowed := lc.GetValuesAsSlice("restrict-allowed")

	for _, ver := range pkg.Versions {
		if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
			continue
		}

		eapi := ver.Ebuild.Vars["EAPI"]
		if eapi == "" {
			eapi = "0"
		}

		for _, dep := range eapisBanned {
			if eapi == dep {
				res := lints.LintResult{
					RuleMetadata: ruleLayoutConf,
					Message:      fmt.Sprintf("[%s] EAPI %s is banned in layout.conf", lints.SeverityError, eapi),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = lints.SeverityError
				results = append(results, res)
			}
		}

		for _, dep := range eapisDeprecated {
			if eapi == dep {
				res := lints.LintResult{
					RuleMetadata: ruleLayoutConf,
					Message:      fmt.Sprintf("[%s] EAPI %s is deprecated in layout.conf", cases.Title(language.Und, cases.NoLower).String(string(severity)), eapi),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}

		if len(propertiesAllowed) > 0 {
			props := ver.Ebuild.Vars["PROPERTIES"]
			for _, prop := range strings.Fields(props) {
				allowed := false
				for _, a := range propertiesAllowed {
					if prop == a || prop == "-"+a {
						allowed = true
						break
					}
				}
				if !allowed {
					res := lints.LintResult{
						RuleMetadata: ruleLayoutConf,
						Message:      fmt.Sprintf("[%s] PROPERTY '%s' is not listed in layout.conf properties-allowed", cases.Title(language.Und, cases.NoLower).String(string(severity)), prop),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}
			}
		}

		if len(restrictAllowed) > 0 {
			restr := ver.Ebuild.Vars["RESTRICT"]
			for _, r := range strings.Fields(restr) {
				allowed := false
				for _, a := range restrictAllowed {
					if r == a || r == "-"+a {
						allowed = true
						break
					}
				}
				if !allowed {
					res := lints.LintResult{
						RuleMetadata: ruleLayoutConf,
						Message:      fmt.Sprintf("[%s] RESTRICT '%s' is not listed in layout.conf restrict-allowed", cases.Title(language.Und, cases.NoLower).String(string(severity)), r),
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
