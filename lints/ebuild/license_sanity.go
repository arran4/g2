package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	lints.RegisterLintRule(&LicenseSanityLintRule{})
}

type LicenseSanityLintRule struct{}

func (r *LicenseSanityLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *LicenseSanityLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string

	severity := "Warning"
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0802"]; ok { // Arbitrary PG code mapping for License Sanity for example, or general
			if val == "notice" || val == "error" || val == "warning" {
				severity = cases.Title(language.English).String(val)
			}
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			licenseStr := ver.Ebuild.Vars["LICENSE"]
			if licenseStr != "" {
				parsedLicenses := g2.ParseLicense(licenseStr)

				isFullText := false

				if len(licenseStr) > 400 {
					isFullText = true
				} else {
					punctuationCount := 0
					stopWordCount := 0

					stopWords := map[string]bool{
						"the": true, "and": true, "this": true, "that": true, "for": true,
						"with": true, "without": true, "copyright": true, "software": true,
						"permission": true, "provided": true, "conditions": true,
						"is": true, "not": true, "are": true, "be": true, "or": true,
					}

					for _, lic := range parsedLicenses {
						if strings.Contains(lic, ",") || strings.Contains(lic, ";") {
							punctuationCount++
						}

						lowerLic := strings.ToLower(lic)
						lowerLic = strings.Trim(lowerLic, ".,;:\"'")
						if stopWords[lowerLic] {
							stopWordCount++
						}
					}

					if punctuationCount >= 2 || stopWordCount >= 3 {
						isFullText = true
					}
				}

				for _, lic := range parsedLicenses {
					hasAlphanumeric := false
					for _, r := range lic {
						if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
							hasAlphanumeric = true
							break
						}
					}

					if !hasAlphanumeric {
						warnings = append(warnings, fmt.Sprintf("[%s] Ebuild %s has a LICENSE '%s' without valid characters.", severity, ver.Version, lic))
					} else if strings.Contains(lic, "/") {
						warnings = append(warnings, fmt.Sprintf("[%s] Ebuild %s has a LICENSE '%s' containing a slash, which may break URLs.", severity, ver.Version, lic))
					}
				}

				if isFullText {
					warnings = append(warnings, fmt.Sprintf("[%s] Ebuild %s has a LICENSE variable that looks like a full-text license rather than a license identifier.", severity, ver.Version))
				}
			}
		}
	}
	return warnings
}
