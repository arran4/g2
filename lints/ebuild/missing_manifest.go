package ebuild

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleMissingManifest = lints.RuleMetadata{
	ID:          "MissingManifest",
	Title:       "Missing Manifest entry for distfile",
	Description: "Checks for distfiles that are required by an ebuild but not listed in the Manifest.",
	Severity:    lints.SeverityError,
	Source:      lints.SourcePkgcheck,
	Tags:        []string{"ebuild", "manifest"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMissingManifest)
	lints.RegisterLintRule(&MissingManifestLintRule{})
}

type MissingManifestLintRule struct{}

func (r *MissingManifestLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *MissingManifestLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	return r.lintWithFS(os.DirFS(repoDir), filepath.Join(pkg.Category, pkg.Name), pkg, qa)
}

func (r *MissingManifestLintRule) lintWithFS(repoFS fs.FS, pkgDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	manifestFiles := make(map[string]bool)
	if pkg.Manifest != nil {
		for _, entry := range pkg.Manifest.Entries {
			if entry.Type == "DIST" {
				manifestFiles[entry.Filename] = true
			}
		}
	}

	entries, err := fs.ReadDir(repoFS, pkgDir)
	if err != nil {
		return results
	}

	pkgFS, err := fs.Sub(repoFS, pkgDir)
	if err != nil {
		return results
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ebuild") {
			ebuildName := entry.Name()
			version := strings.TrimSuffix(ebuildName, ".ebuild")
			if strings.HasPrefix(version, pkg.Name+"-") {
				version = strings.TrimPrefix(version, pkg.Name+"-")
			}

			parsedEbuild, err := g2.ParseEbuild(pkgFS, ebuildName, g2.ParseFull)
			if err != nil {
				continue
			}

			srcUriVar := parsedEbuild.Vars["SRC_URI"]
			if srcUriVar == "" {
				continue
			}
			dummyContent := fmt.Sprintf("SRC_URI=\"%s\"", srcUriVar)
			uris, err := g2.ExtractURIs(dummyContent, parsedEbuild.Vars)
			if err != nil {
				continue
			}

			var missingFiles []string
			for _, uri := range uris {
				if !manifestFiles[uri.Filename] {
					missingFiles = append(missingFiles, uri.Filename)
				}
			}

			if len(missingFiles) > 0 {
				res := lints.LintResult{
					RuleMetadata: ruleMissingManifest,
					Message:      fmt.Sprintf("[%s] version %s: distfile missing from Manifest: [ %s ]", ruleMissingManifest.Severity, version, strings.Join(missingFiles, ", ")),
					Package:      pkg.Category + "/" + pkg.Name,
					File:         ebuildName,
				}
				results = append(results, res)
			}
		}
	}

	return results
}
