package metadata

import (
	"fmt"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleManifestChecks = lints.RuleMetadata{
	ID:          "ManifestChecks",
	Title:       "Manifest File Checks",
	Description: "Ensures the Manifest file exists and conforms to layout.conf.",
	URL:         "https://devmanual.gentoo.org/general-concepts/manifest/index.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"manifest"},
}

func init() {
	lints.RegisterRuleMetadata(ruleManifestChecks)
	lints.RegisterLintRule(&ManifestLintRule{})
}

type ManifestLintRule struct{}

func (r *ManifestLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *ManifestLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	// Check if missing entirely
	if pkg.Manifest == nil {
		if len(pkg.Versions) > 0 {
			res := lints.LintResult{
				RuleMetadata: ruleManifestChecks,
				Message:      fmt.Sprintf("[%s] Missing or unparsable Manifest file", lints.SeverityError),
				Package:      pkg.Category + "/" + pkg.Name,
			}
			res.RuleMetadata.Severity = lints.SeverityError
			results = append(results, res)
		}
		return results
	}

	// layout.conf checks
	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	lc, err := g2.ParseLayoutConf(layoutConfPath)
	if err != nil {
		return results
	}

	manifestHashes := lc.GetValuesAsSlice("manifest-hashes")
	manifestRequiredHashes := lc.GetValuesAsSlice("manifest-required-hashes")

	for _, entry := range pkg.Manifest.Entries {
		for _, requiredHash := range manifestRequiredHashes {
			found := false
			for _, hash := range entry.Hashes {
				if hash.Type == requiredHash {
					found = true
					break
				}
			}
			if !found {
				res := lints.LintResult{
					RuleMetadata: ruleManifestChecks,
					Message:      fmt.Sprintf("[%s] Manifest entry %s is missing required hash '%s'", cases.Title(language.Und, cases.NoLower).String(string(lints.SeverityError)), entry.Filename, requiredHash),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = lints.SeverityError
				results = append(results, res)
			}
		}

		if len(manifestHashes) > 0 {
			for _, hash := range entry.Hashes {
				allowed := false
				for _, allowedHash := range manifestHashes {
					if hash.Type == allowedHash {
						allowed = true
						break
					}
				}
				if !allowed {
					res := lints.LintResult{
						RuleMetadata: ruleManifestChecks,
						Message:      fmt.Sprintf("[%s] Manifest entry %s has unallowed hash '%s'", cases.Title(language.Und, cases.NoLower).String(string(lints.SeverityWarning)), entry.Filename, hash.Type),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = lints.SeverityWarning
					results = append(results, res)
				}
			}
		}
	}

	return results
}
