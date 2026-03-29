package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMissingKeyword = lints.RuleMetadata{
	ID:          "MissingKeyword",
	Title:       "Missing Keywords",
	Description: "Checks if a package has empty KEYWORDS. Exempts virtual and *-9999 (live) packages.",
	URL:         "https://devmanual.gentoo.org/keywording/",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "keywords"},
}

func init() {
	lints.RegisterLintRule(&MissingKeywordLintRule{})
}

type MissingKeywordLintRule struct{}

func (r *MissingKeywordLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MissingKeywordLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if pkg.Category == "virtual" {
		return nil
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			if strings.HasSuffix(ver.Version, "9999") {
				continue // Live ebuilds usually don't have keywords
			}

			keywords := ver.Ebuild.Vars["KEYWORDS"]
			if strings.TrimSpace(keywords) == "" {
				res := lints.LintResult{
					RuleMetadata: ruleMissingKeyword,
					Message:      fmt.Sprintf("[%s] Ebuild %s has empty KEYWORDS", cases.Title(language.English).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}
	return results
}
