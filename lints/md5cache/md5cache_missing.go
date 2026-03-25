package md5cache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&MD5CacheLintRule{})
}

type MD5CacheLintRule struct{}

func (r *MD5CacheLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string
	cachePath := filepath.Join(repoDir, "metadata", "md5-cache", pkg.Category, pkg.Name)
	for _, ver := range pkg.Versions {
		verCachePath := fmt.Sprintf("%s-%s", cachePath, ver.Version)
		if _, err := os.Stat(verCachePath); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("Missing md5-cache for version %s. Run 'ebuild <ebuild_file> manifest' or 'egencache' to generate it.", ver.Version))
		}
	}
	return warnings
}
