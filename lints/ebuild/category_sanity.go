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

// CategorySanityLintRule checks if the package category exists in profiles/categories and main gentoo categories.
type CategorySanityLintRule struct {
	mu           sync.RWMutex
	repoCatsCache map[string]map[string]bool // map[repoDir]map[category]bool
}

// SkipForSiteGen disables this rule when it is already pre-calculated manually by site.go.
var SkipForSiteGen bool

func init() {
	lints.RegisterLintRule(&CategorySanityLintRule{})
}

func (l *CategorySanityLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	if SkipForSiteGen {
		return nil
	}

	var warnings []string
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
			warnings = append(warnings, fmt.Sprintf("Warning: category '%s' is not listed in repo's profiles/categories", name))
		} else {
			warnings = append(warnings, fmt.Sprintf("Error: category '%s' is not listed in repo's profiles/categories or the main gentoo categories list", name))
		}
	} else if len(mainCats) > 0 && !inMain {
		warnings = append(warnings, fmt.Sprintf("Note: category '%s' is not in the main gentoo categories list", name))
	}

	return warnings
}
