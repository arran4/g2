package md5cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&MD5CacheLintRule{})
}

type MD5CacheLintRule struct{}

func (r *MD5CacheLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MD5CacheLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string

	severity := "Warning"
	// md5-cache missing doesn't have a direct PG rule, but we can respect a generic one if it existed
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0000"]; ok { // placeholder if there was one
			if val == "notice" || val == "error" || val == "warning" {
				severity = strings.ToUpper(val[:1]) + val[1:]
			}
		}
	}
	cachePath := filepath.Join(repoDir, "metadata", "md5-cache", pkg.Category, pkg.Name)
	for _, ver := range pkg.Versions {
		verCachePath := fmt.Sprintf("%s-%s", cachePath, ver.Version)
		if _, err := os.Stat(verCachePath); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("[%s] Missing md5-cache for version %s. Run 'ebuild <ebuild_file> manifest' or 'egencache' to generate it.", severity, ver.Version))
		}
	}
	return warnings
}
