package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var rulePackageCollision = lints.RuleMetadata{
	ID:          "PackageCollision",
	Title:       "Package Collision",
	Description: "Validates that a package does not collide with or shadow a package from another repository.",
	URL:         "https://devmanual.gentoo.org/general-concepts/package-collisions/index.html",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "collision"},
}

func init() {
	lints.RegisterRuleMetadata(rulePackageCollision)
	lints.RegisterLintRule(&PackageCollisionLintRule{})
}

type PackageCollisionLintRule struct{}

func (r *PackageCollisionLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *PackageCollisionLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult

	if ctx == nil || ctx.OtherRepos == nil || len(ctx.OtherRepos) == 0 {
		return results
	}

	for repoName, otherSite := range ctx.OtherRepos {
		for _, cat := range otherSite.Categories {
			if cat.Name == pkg.Category {
				for _, otherPkg := range cat.Packages {
					if otherPkg.Name == pkg.Name {
						res := lints.LintResult{
							RuleMetadata: rulePackageCollision,
							Message:      fmt.Sprintf("Package %s/%s collides with package in repository '%s'", pkg.Category, pkg.Name, repoName),
							Package:      fmt.Sprintf("%s/%s", pkg.Category, pkg.Name),
						}
						results = append(results, res)
						break
					}
				}
				break
			}
		}
	}

	return results
}
