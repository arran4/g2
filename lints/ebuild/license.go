package ebuild

import (
	"fmt"
	"strings"
	"os"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleLicense = lints.RuleMetadata{
	ID:          "License",
	Title:       "LICENSE variable must explicitly list all licenses",
	Description: "The LICENSE variable must explicitly list all licenses pertaining to the corresponding source of the files installed by the package.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/other-metadata.html#pg0704"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0704"},
}

var ruleLicenseExists = lints.RuleMetadata{
	ID:          "LicenseExists",
	Title:       "LICENSE variable must only list valid licenses",
	Description: "The licenses specified in the LICENSE variable must exist in the repository or its masters.",
	URL:         "https://devmanual.gentoo.org/general-concepts/licenses/index.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

func init() {
	lints.RegisterRuleMetadata(ruleLicense)
	lints.RegisterLintRule(&LicenseLintRule{})
	lints.RegisterRuleMetadata(ruleLicenseExists)
	lints.RegisterRepoLintRule(&LicenseExistsRepoLintRule{})
}

type LicenseLintRule struct{}

func (r *LicenseLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *LicenseLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleLicense.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0704"]; ok {
			if val == "ignore" {
				return nil
			}
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
			license := strings.TrimSpace(ver.Ebuild.Vars["LICENSE"])

			// Virtuals don't require LICENSE
			if pkg.Category == "virtual" {
			    continue
			}

			if license == "" {
				res := lints.LintResult{
					RuleMetadata: ruleLicense,
					Message:      fmt.Sprintf("[%s] Ebuild %s does not specify LICENSE", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}

type LicenseExistsRepoLintRule struct{}

func (r *LicenseExistsRepoLintRule) LintRepo(repoDir string, site *g2.SiteData) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleLicenseExists.Severity

	if site != nil && site.QAPolicy != nil && site.QAPolicy.Policies != nil {
		// Using PG0704 policy setting as a fallback since LicenseExists doesn't have a specific PG policy,
		// or we can just use severity directly. For now, we use severity directly, unless PG0704 is specified
		if val, ok := site.QAPolicy.Policies["PG0704"]; ok {
			if val == "ignore" {
				return nil
			}
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

	validLicenses := make(map[string]bool)
	if site != nil {
		for _, lic := range site.ProvidedLicenses {
			validLicenses[lic] = true
		}
		if site.LicenseMapping != nil {
			for group := range site.LicenseMapping {
				validLicenses[group] = true
			}
		}

		// Parse metadata/layout.conf for masters (upstream overlays)
		if site.LayoutConf != nil {
			var masters []string
			for _, entry := range site.LayoutConf.Entries {
				if entry.Key == "masters" {
					masters = strings.Fields(entry.Value)
					break
				}
			}

			if len(masters) > 0 {
				masterPaths := make(map[string]string)

				// Parse default repos.conf
				if rc, err := g2.ParseReposConf(DefaultReposConfPath); err == nil && rc != nil {
					for _, file := range rc.Files {
						for _, sec := range file.Sections {
							if loc := sec.Get("location"); loc != "" {
								masterPaths[sec.Name] = loc
							}
						}
					}
				}

				for _, master := range masters {
					loc, ok := masterPaths[master]
					if !ok {
						loc = filepath.Join(DefaultReposBasePath, master)
					}

					// Read licenses from master
					masterLicensesDir := filepath.Join(loc, "licenses")
					if entries, err := os.ReadDir(masterLicensesDir); err == nil {
						for _, entry := range entries {
							if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
								validLicenses[entry.Name()] = true
							}
						}
					}

					// Read license groups from master
					masterLicenseGroupsPath := filepath.Join(loc, "profiles", "license_groups")
					if f, err := os.Open(masterLicenseGroupsPath); err == nil {
						defer func() { _ = f.Close() }()
						if groups, err := g2.ParseLicenseGroups(f); err == nil {
							for group := range groups {
								validLicenses[group] = true
							}
						}
					}
				}
			}
		}
	}

	if site != nil {
		for _, cat := range site.Categories {
			for _, pkg := range cat.Packages {
				for _, ver := range pkg.Versions {
					if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
						continue
					}

					licenseStr := strings.TrimSpace(ver.Ebuild.Vars["LICENSE"])
					if licenseStr == "" {
						continue
					}

					licenses := g2.ParseLicense(licenseStr)

					for _, lic := range licenses {
						// Sometimes empty strings can appear from parse issues or trailing spaces
						if lic == "" {
							continue
						}

						if !validLicenses[lic] {
							res := lints.LintResult{
								RuleMetadata: ruleLicenseExists,
								Message:      fmt.Sprintf("[%s] Ebuild %s uses a license or license group '%s' which does not exist in the repository or its masters.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, lic),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				}
			}
		}
	}

	return results
}
