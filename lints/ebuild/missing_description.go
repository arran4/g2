package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMissingDescription = lints.RuleMetadata{
	ID:          "MissingDescription",
	Title:       "Missing or Short Description",
	Description: "Checks if a package's DESCRIPTION is missing or unhelpfully short (less than 10 characters).",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/variables/index.html#description",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "site-quality", "search-quality"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMissingDescription)
	lints.RegisterLintRule(&MissingDescriptionLintRule{})
}

type MissingDescriptionLintRule struct{}

func (r *MissingDescriptionLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MissingDescriptionLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			desc := ver.Ebuild.Vars["DESCRIPTION"]
			desc = strings.TrimSpace(desc)

			if len(desc) == 0 {
				res := lints.LintResult{
					RuleMetadata: ruleMissingDescription,
					Message:      fmt.Sprintf("[%s] Ebuild %s is missing DESCRIPTION", cases.Title(language.English).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			} else if len(desc) < 10 {
				res := lints.LintResult{
					RuleMetadata: ruleMissingDescription,
					Message:      fmt.Sprintf("[%s] Ebuild %s DESCRIPTION is suspiciously short ('%s')", cases.Title(language.English).String(string(severity)), ver.Version, desc),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}
	return results
}
