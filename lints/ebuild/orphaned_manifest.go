package ebuild

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleOrphanedManifest = lints.RuleMetadata{
	ID:          "OrphanedManifest",
	Title:       "Orphaned Manifest files and lines",
	Description: "Checks for files that are listed in the Manifest but not used by any ebuild, or not present in the package directory.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "manifest"},
}

func init() {
	lints.RegisterRuleMetadata(ruleOrphanedManifest)
	lints.RegisterLintRule(&OrphanedManifestLintRule{})
}

type OrphanedManifestLintRule struct{}

func (r *OrphanedManifestLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *OrphanedManifestLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	if pkg.Manifest == nil || len(pkg.Manifest.Entries) == 0 {
		return results
	}

	usedFiles := make(map[string]bool)
	for _, ver := range pkg.Versions {
		// Try to read ebuild to extract URIs
		ebuildPath := filepath.Join(repoDir, pkg.Category, pkg.Name, fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version))
		content, err := os.ReadFile(ebuildPath)
		if err != nil {
			continue // If we can't read it, skip
		}

		vars := g2.ParseEbuildVariables(ebuildPath)
		uris, err := g2.ExtractURIs(string(content), vars)
		if err == nil {
			for _, uri := range uris {
				usedFiles[uri.Filename] = true
			}
		}
	}

	pkgDir := filepath.Join(repoDir, pkg.Category, pkg.Name)
	for _, entry := range pkg.Manifest.Entries {
		switch entry.Type {
		case "DIST":
			if !usedFiles[entry.Filename] {
				res := lints.LintResult{
					RuleMetadata: ruleOrphanedManifest,
					Message:      fmt.Sprintf("[%s] Manifest entry for unused DIST file '%s'", ruleOrphanedManifest.Severity, entry.Filename),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		case "EBUILD", "MISC", "AUX":
			var filePath string
			if entry.Type == "AUX" {
				filePath = filepath.Join(pkgDir, "files", entry.Filename)
			} else {
				filePath = filepath.Join(pkgDir, entry.Filename)
			}

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				res := lints.LintResult{
					RuleMetadata: ruleOrphanedManifest,
					Message:      fmt.Sprintf("[%s] Manifest entry for non-existent %s file '%s'", ruleOrphanedManifest.Severity, entry.Type, entry.Filename),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}

	return results
}
