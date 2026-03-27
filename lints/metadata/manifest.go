package metadata

import (
	"fmt"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&ManifestLintRule{})
}

type ManifestLintRule struct{}

func (r *ManifestLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *ManifestLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string

	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	lc, err := g2.ParseLayoutConf(layoutConfPath)
	if err != nil {
		return warnings
	}

	manifestHashes := lc.GetValuesAsSlice("manifest-hashes")
	manifestRequiredHashes := lc.GetValuesAsSlice("manifest-required-hashes")

	if len(manifestRequiredHashes) == 0 {
		return warnings
	}

	manifestPath := filepath.Join(repoDir, pkg.Category, pkg.Name, "Manifest")
	manifest, err := g2.ParseManifest(manifestPath)
	if err != nil {
		return warnings
	}

	for _, entry := range manifest.Entries {
		for _, requiredHash := range manifestRequiredHashes {
			found := false
			for _, hash := range entry.Hashes {
				if hash.Type == requiredHash {
					found = true
					break
				}
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf("[Error] Manifest entry %s is missing required hash '%s'", entry.Filename, requiredHash))
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
					warnings = append(warnings, fmt.Sprintf("[Warning] Manifest entry %s has unallowed hash '%s'", entry.Filename, hash.Type))
				}
			}
		}
	}

	return warnings
}
