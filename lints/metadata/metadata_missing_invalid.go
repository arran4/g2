package metadata

import (
	"fmt"
	"os"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&MetadataLintRule{})
}

type MetadataLintRule struct{}

func (r *MetadataLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string
	if pkg.Metadata == nil {
		if pkg.MetadataError != nil {
			if os.IsNotExist(pkg.MetadataError) {
				warnings = append(warnings, "metadata.xml is missing. Create one to describe the package.")
			} else {
				warnings = append(warnings, fmt.Sprintf("metadata.xml is invalid: %v. Fix the XML syntax or schema.", pkg.MetadataError))
			}
		} else {
			warnings = append(warnings, "metadata.xml is missing or invalid. Check the file for issues.")
		}
	}
	return warnings
}
