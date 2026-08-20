package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func (r *OrphanedManifestLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *OrphanedManifestLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult

	if pkg.Manifest == nil || len(pkg.Manifest.Entries) == 0 {
		return results
	}

	pkgDir := filepath.Join(repoDir, pkg.Category, pkg.Name)
	usedFiles := make(map[string]bool)
	parseFailed := false
	entries, err := os.ReadDir(pkgDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ebuild") {
				parsedEbuild, err := g2.ParseEbuild(os.DirFS(pkgDir), entry.Name(), g2.ParseFull)
				if err == nil {
					for _, uri := range parsedEbuild.SrcUri {
						usedFiles[uri.Filename] = true
					}
				} else {
					parseFailed = true
				}
			}
		}
	}

	for _, entry := range pkg.Manifest.Entries {
		switch entry.Type {
		case "DIST":
			if !parseFailed && !usedFiles[entry.Filename] {
				res := lints.LintResult{
					RuleMetadata: ruleOrphanedManifest,
					Message:      fmt.Sprintf("[%s] Manifest entry for unused DIST file '%s'", ruleOrphanedManifest.Severity, entry.Filename),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}

	return results
}
