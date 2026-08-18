package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	rule := &PackageCollisionLintRule{}
	lints.RegisterRuleMetadata(rule.Metadata())
	lints.RegisterLintRule(rule)
}

type PackageCollisionLintRule struct{}

func (r *PackageCollisionLintRule) Metadata() lints.RuleMetadata {
	return lints.RuleMetadata{
		ID:          "PG1000",
		Title:       "Package Name Collision",
		Description: "Warns if a package shares the exact same name with a package in another category.",
		Severity:    lints.SeverityWarning,
		Source:      lints.SourceG2,
		Tags:        []string{"pkg_collisions"},
	}
}

func (r *PackageCollisionLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *PackageCollisionLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	if qa != nil {
		policy := qa.Policies[string(r.Metadata().ID)]
		if policy == "ignore" {
			return nil
		}
	}

		var results []lints.LintResult
	dirsToCheck := []string{repoDir}
	seenDirs := map[string]bool{repoDir: true}

	if ctx != nil {
		for _, r := range ctx.OtherRepos {
			if !seenDirs[r] {
				dirsToCheck = append(dirsToCheck, r)
				seenDirs[r] = true
			}
		}
	}

	for _, dir := range dirsToCheck {
		categories, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, cat := range categories {
			if !cat.IsDir() || strings.HasPrefix(cat.Name(), ".") || cat.Name() == "metadata" || cat.Name() == "profiles" || cat.Name() == "eclass" {
				continue
			}

			// If it's the exact same category and name, it's just an overlay overriding it, not a collision.
			if cat.Name() == pkg.Category {
				continue
			}

			// Check if package name exists in this category
			pkgPath := filepath.Join(dir, cat.Name(), pkg.Name)
			if stat, err := os.Stat(pkgPath); err == nil && stat.IsDir() {
				// Potential collision
				var repoName string
				if repoInfo, err := os.ReadFile(filepath.Join(dir, "profiles", "repo_name")); err == nil {
					repoName = strings.TrimSpace(string(repoInfo))
				} else {
					repoName = filepath.Base(dir)
				}

				results = append(results, lints.LintResult{
					RuleMetadata: r.Metadata(),
					Message:      fmt.Sprintf("Package name collision: '%s' is also defined in category '%s' in repository '%s'.", pkg.Name, cat.Name(), repoName),
				})
			}
		}
	}

	return results
}
