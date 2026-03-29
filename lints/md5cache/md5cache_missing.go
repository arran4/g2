package md5cache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMD5CacheMissing = lints.RuleMetadata{
	ID:          "Md5CacheMissing",
	Title:       "Missing MD5 Cache",
	Description: "Verifies that an md5-cache entry exists for every ebuild version.",
	URL:         "https://devmanual.gentoo.org/general-concepts/overlay-layout/#md5-cache",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"md5-cache", "site-quality"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMD5CacheMissing)
	lints.RegisterLintRule(&MD5CacheLintRule{})
}

type MD5CacheLintRule struct{}

func (r *MD5CacheLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MD5CacheLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0802"]; ok {
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

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil {
			cachePath := filepath.Join(repoDir, "metadata", "md5-cache", pkg.Category, pkg.Name+"-"+ver.Version)
			if _, err := os.Stat(cachePath); os.IsNotExist(err) {
				res := lints.LintResult{
					RuleMetadata: ruleMD5CacheMissing,
					Message:      fmt.Sprintf("[%s] Missing md5-cache for ebuild %s-%s", cases.Title(language.English).String(string(severity)), pkg.Name, ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
