package metadata

import (
	"fmt"
	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	lints.RegisterLintRule(&MaintainerLintRule{})
}

type MaintainerLintRule struct{}

func (r *MaintainerLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MaintainerLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string
	severity := "Warning"
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0702"]; ok {
			if val == "notice" || val == "error" || val == "warning" {
				severity = cases.Title(language.English).String(val)
			}
		}
	}

	if pkg.Metadata != nil && len(pkg.Metadata.Maintainers) == 0 {
		warnings = append(warnings, fmt.Sprintf("[%s] metadata.xml is missing a maintainer. Add at least one <maintainer> element.", severity))
	}

	return warnings
}
