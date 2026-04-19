package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleLicenseSanity = lints.RuleMetadata{
	ID:          "LicenseSanity",
	Title:       "License Sanity",
	Description: "Identifies ebuilds improperly placing full-text licenses in the LICENSE variable, missing alphanumeric characters (e.g., //), or containing URL-breaking slashes.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/variables/index.html#license",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "site-quality", "gentoo-policy"},
}

func init() {
	lints.RegisterRuleMetadata(ruleLicenseSanity)
	lints.RegisterLintRule(&LicenseSanityLintRule{})
}

type LicenseSanityLintRule struct{}

func (r *LicenseSanityLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *LicenseSanityLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0804"]; ok {
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
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			license := ver.Ebuild.Vars["LICENSE"]
			if license == "" {
				continue // handled by another lint possibly
			}

			// check if it's potentially a full text instead of identifiers
			// e.g. contains lots of spaces or common full-text phrases
			if strings.Contains(strings.ToLower(license), "copyright (c)") ||
				strings.Contains(strings.ToLower(license), "all rights reserved") ||
				len(strings.Fields(license)) > 20 {
				res := lints.LintResult{
					RuleMetadata: ruleLicenseSanity,
					Message:      fmt.Sprintf("[%s] Ebuild %s appears to have full-text license in LICENSE variable instead of identifiers", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			} else {
				// parse to individual licenses to check for alphanumerics and slashes
				licenses := g2.ParseLicense(license)
				for _, lic := range licenses {
					if lic == "" {
						continue
					}
					hasAlnum := false
					for _, ch := range lic {
						if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
							hasAlnum = true
							break
						}
					}
					if !hasAlnum {
						res := lints.LintResult{
							RuleMetadata: ruleLicenseSanity,
							Message:      fmt.Sprintf("[%s] Ebuild %s contains invalid license identifier '%s' (missing alphanumeric characters)", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, lic),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
					} else if strings.Contains(lic, "/") {
						res := lints.LintResult{
							RuleMetadata: ruleLicenseSanity,
							Message:      fmt.Sprintf("[%s] Ebuild %s contains license identifier '%s' with a slash, which breaks URLs", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, lic),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
					}
				}
			}
		}
	}
	return results
}
