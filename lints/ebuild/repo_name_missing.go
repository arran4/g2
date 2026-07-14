package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleRepoNameMissing = lints.RuleMetadata{
	ID:          "RepoNameMissing",
	Title:       "Missing Repository Name",
	Description: "Every Gentoo repository must define a unique repository name, either via repo-name in metadata/layout.conf or in profiles/repo_name.",
	URL:         "https://devmanual.gentoo.org/general-concepts/overlay-layout/",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"repo-layout"},
}

func init() {
	lints.RegisterRuleMetadata(ruleRepoNameMissing)
	lints.RegisterLintRule(&RepoNameMissingLintRule{})
}

type RepoNameMissingLintRule struct {
	mu        sync.RWMutex
	repoCache map[string]bool // map[repoDir]hasRepoName
}

func (r *RepoNameMissingLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *RepoNameMissingLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	r.mu.RLock()
	if r.repoCache == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if r.repoCache == nil {
			r.repoCache = make(map[string]bool)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	hasName, cached := r.repoCache[repoDir]
	r.mu.RUnlock()

	if !cached {
		r.mu.Lock()
		hasName, cached = r.repoCache[repoDir]
		if !cached {
			hasName = false
			// Check layout.conf
			lc, err := g2.ParseLayoutConf(filepath.Join(repoDir, "metadata", "layout.conf"))
			if err == nil && lc.GetValue("repo-name") != "" {
				hasName = true
			} else {
				// Check profiles/repo_name
				b, err := os.ReadFile(filepath.Join(repoDir, "profiles", "repo_name"))
				if err == nil && strings.TrimSpace(string(b)) != "" {
					hasName = true
				}
			}
			r.repoCache[repoDir] = hasName
		}
		r.mu.Unlock()
	}

	if !hasName {
		res := lints.LintResult{
			RuleMetadata: ruleRepoNameMissing,
			Message:      fmt.Sprintf("[%s] Repository is missing a repo-name in metadata/layout.conf or profiles/repo_name", cases.Title(language.Und, cases.NoLower).String(string(severity))),
			Package:      pkg.Category + "/" + pkg.Name,
		}
		results = append(results, res)
	}

	return results
}
