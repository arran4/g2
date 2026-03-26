package metadata

import (
	"fmt"
	"os"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&MetadataLintRule{})
}

type MetadataLintRule struct{}

func (r *MetadataLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MetadataLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string
	severity := "Warning"
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0701"]; ok { // Map to some PG rule for missing metadata
			if val == "notice" || val == "error" || val == "warning" {
				severity = strings.ToUpper(val[:1]) + val[1:]
			}
		}
	}
	if pkg.Metadata == nil {
		if pkg.MetadataError != nil {
			if os.IsNotExist(pkg.MetadataError) {
				warnings = append(warnings, fmt.Sprintf("[%s] metadata.xml is missing. Create one to describe the package.", severity))
			} else {
				warnings = append(warnings, fmt.Sprintf("[%s] metadata.xml is invalid: %v. Fix the XML syntax or schema.", severity, pkg.MetadataError))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("[%s] metadata.xml is missing or invalid. Check the file for issues.", severity))
		}
	}
	return warnings
}
