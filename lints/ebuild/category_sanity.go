package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleCategorySanity = lints.RuleMetadata{
	ID:          "CategorySanity",
	Title:       "Category Sanity",
	Description: "Checks if the package category exists in profiles/categories and main gentoo categories.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"repo-layout"},
}

// CategorySanityLintRule checks if the package category exists in profiles/categories and main gentoo categories.
type CategorySanityLintRule struct {
	mu            sync.RWMutex
	repoCatsCache map[string]map[string]bool // map[repoDir]map[category]bool
}

// SkipForSiteGen disables this rule when it is already pre-calculated manually by site.go.
var SkipForSiteGen bool

func init() {
	lints.RegisterLintRule(&CategorySanityLintRule{})
}

func (l *CategorySanityLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	if SkipForSiteGen {
		return nil
	}

	var results []lints.LintResult
	name := pkg.Category

	l.mu.RLock()
	if l.repoCatsCache == nil {
		l.mu.RUnlock()
		l.mu.Lock()
		if l.repoCatsCache == nil {
			l.repoCatsCache = make(map[string]map[string]bool)
		}
		l.mu.Unlock()
		l.mu.RLock()
	}

	supportedCategories, ok := l.repoCatsCache[repoDir]
	l.mu.RUnlock()

	if !ok {
		l.mu.Lock()
		supportedCategories, ok = l.repoCatsCache[repoDir]
		if !ok {
			supportedCategories = make(map[string]bool)
			categoriesBytes, err := os.ReadFile(filepath.Join(repoDir, "profiles", "categories"))
			if err == nil {
				lines := strings.Split(string(categoriesBytes), "\n")
				for _, line := range lines {
					cat := strings.TrimSpace(line)
					if cat != "" && !strings.HasPrefix(cat, "#") {
						supportedCategories[cat] = true
					}
				}
			}
			l.repoCatsCache[repoDir] = supportedCategories
		}
		l.mu.Unlock()
	}

	inRepo := len(supportedCategories) == 0 || supportedCategories[name]
	mainCats := g2.FetchMainGentooCategories()
	inMain := len(mainCats) == 0 || mainCats[name]

	if len(supportedCategories) > 0 && !inRepo {
		if inMain {
			res := lints.LintResult{
				RuleMetadata: ruleCategorySanity,
				Message:      fmt.Sprintf("Warning: category '%s' is not listed in repo's profiles/categories", name),
				Package:      pkg.Category + "/" + pkg.Name,
			}
			res.RuleMetadata.Severity = lints.SeverityWarning
			results = append(results, res)
		} else {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleCategorySanity,
				Message:      fmt.Sprintf("Error: category '%s' is not listed in repo's profiles/categories or the main gentoo categories list", name),
				Package:      pkg.Category + "/" + pkg.Name,
			})
		}
	} else if len(mainCats) > 0 && !inMain {
		res := lints.LintResult{
			RuleMetadata: ruleCategorySanity,
			Message:      fmt.Sprintf("Note: category '%s' is not in the main gentoo categories list", name),
			Package:      pkg.Category + "/" + pkg.Name,
		}
		res.RuleMetadata.Severity = lints.SeverityNotice
		results = append(results, res)
	}

	return results
}
