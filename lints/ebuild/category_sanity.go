package ebuild

import (
	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

// CategorySanityLintRule checks if the package category exists in profiles/categories and main gentoo categories.
// For performance reasons, this logic is pre-calculated and appended during repo parsing in cmd/g2/site.go.
// This rule exists in the registry purely for visibility and tracking.
type CategorySanityLintRule struct{}

func init() {
	lints.RegisterLintRule(&CategorySanityLintRule{})
}

func (l *CategorySanityLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	// Logic is pre-calculated in site.go to avoid parsing profiles/categories
	// and fetching main Gentoo categories multiple times during linting.
	return nil
}
