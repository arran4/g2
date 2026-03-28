package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&SubshellFunctionLintRule{})
}

type SubshellFunctionLintRule struct{}

func (l *SubshellFunctionLintRule) Name() string {
	return "SubshellFunction"
}

func (l *SubshellFunctionLintRule) Description() string {
	return "Warns when an ebuild uses parentheses (...) instead of braces {...} for function bodies"
}

func (l *SubshellFunctionLintRule) Lint(repoDir string, pkgData *g2.PackageData) []string {
	var results []string

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		for _, warning := range version.Ebuild.ParseWarnings {
			if strings.Contains(warning, "treating as subshell body") {
				results = append(results, fmt.Sprintf("Warning: Ebuild %s uses a subshell for a function body instead of braces", version.Ebuild.Path))
			}
		}
	}

	return results
}
