package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"github.com/stretchr/testify/assert"
)

func TestPackageCollisionLintRule(t *testing.T) {
	rule := &PackageCollisionLintRule{}

	t.Run("No collision", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-misc",
			Name:     "foo",
		}

		ctx := &lints.LintContext{
			OtherRepos: map[string]*g2.SiteData{
				"other-repo": {
					Categories: []g2.CategoryData{
						{
							Name: "app-misc",
							Packages: []g2.PackageData{
								{Name: "bar"},
							},
						},
					},
				},
			},
		}

		results := rule.LintWithQA(".", pkg, nil, ctx)
		assert.Empty(t, results)
	})

	t.Run("Collision detected", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-misc",
			Name:     "foo",
		}

		ctx := &lints.LintContext{
			OtherRepos: map[string]*g2.SiteData{
				"other-repo": {
					Categories: []g2.CategoryData{
						{
							Name: "app-misc",
							Packages: []g2.PackageData{
								{Name: "bar"},
								{Name: "foo"},
							},
						},
					},
				},
			},
		}

		results := rule.LintWithQA(".", pkg, nil, ctx)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Message, "collides with package in repository 'other-repo'")
	})

	t.Run("No ctx", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-misc",
			Name:     "foo",
		}
		results := rule.LintWithQA(".", pkg, nil, nil)
		assert.Empty(t, results)
	})
}
