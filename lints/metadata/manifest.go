package metadata

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/general-concepts/manifest/index.html", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"manifest"},
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
	ctx := lints.GetRepoContext(repoDir)
	var lc *g2.LayoutConf
	if ctx != nil {
		lc = ctx.LayoutConf
	} else {
		layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
		var err error
		lc, err = g2.ParseLayoutConf(layoutConfPath)
		if err != nil {
			return results
		}
	}
	if lc == nil {
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

		// Ensure the file exists if it's expected to be in the tree
		if entry.Type == "EBUILD" || entry.Type == "MISC" || entry.Type == "AUX" {
			var err error
			if ctx != nil && ctx.FS != nil {
				var relPath string
				if entry.Type == "AUX" {
					relPath = filepath.ToSlash(filepath.Join(ctx.RepoDirInFS, pkg.Category, pkg.Name, "files", entry.Filename))
				} else {
					relPath = filepath.ToSlash(filepath.Join(ctx.RepoDirInFS, pkg.Category, pkg.Name, entry.Filename))
				}
				_, err = fs.Stat(ctx.FS, relPath)
			} else {
				var filePath string
				if entry.Type == "AUX" {
					filePath = filepath.Join(repoDir, pkg.Category, pkg.Name, "files", entry.Filename)
				} else {
					filePath = filepath.Join(repoDir, pkg.Category, pkg.Name, entry.Filename)
				}
				_, err = os.Stat(filePath)
			}
			if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
				res := lints.LintResult{
					RuleMetadata: ruleManifestChecks,
					Message:      fmt.Sprintf("[%s] Manifest entry %s references a missing file", cases.Title(language.English).String(string(lints.SeverityError)), entry.Filename),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}

	return results
}
