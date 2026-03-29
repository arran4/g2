package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMissingHomepage = lints.RuleMetadata{
	ID:          "MissingHomepage",
	Title:       "Missing Homepage",
	Description: "Checks if a package's HOMEPAGE is missing or invalid. Note: virtuals are exempt.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/variables/index.html#homepage",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "site-quality"},
}

func init() {
	lints.RegisterLintRule(&MissingHomepageLintRule{})
}

type MissingHomepageLintRule struct{}

func (r *MissingHomepageLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MissingHomepageLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	// virtuals do not strictly need homepages, often it's omitted
	if pkg.Category == "virtual" {
		return nil
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			homepage := ver.Ebuild.Vars["HOMEPAGE"]
			homepage = strings.TrimSpace(homepage)

			if len(homepage) == 0 {
				res := lints.LintResult{
					RuleMetadata: ruleMissingHomepage,
					Message:      fmt.Sprintf("[%s] Ebuild %s is missing HOMEPAGE", cases.Title(language.English).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			} else if !strings.HasPrefix(homepage, "http") && !strings.HasPrefix(homepage, "ftp") {
				res := lints.LintResult{
					RuleMetadata: ruleMissingHomepage,
					Message:      fmt.Sprintf("[%s] Ebuild %s HOMEPAGE '%s' does not start with http/https/ftp", cases.Title(language.English).String(string(severity)), ver.Version, homepage),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}
	return results
}
