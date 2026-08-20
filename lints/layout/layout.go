package layout

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleRepoLayout = lints.RuleMetadata{
	ID:          "RepoLayout",
	Title:       "Repository Layout",
	Description: "Verifies the repository layout to ensure there are no stray files or unlisted categories, honoring .g2ignore if present.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"repo-layout"},
}

func init() {
	lints.RegisterRuleMetadata(ruleRepoLayout)
	lints.RegisterRepoLintRule(&RepoLayoutLintRule{})
}

type RepoLayoutLintRule struct{}

var (
	LayoutLintEnabled bool
	AllowGithubAPI    bool
	UpstreamRepoPath  string
)

func (l *RepoLayoutLintRule) LintRepo(repoDir string, site *g2.SiteData, ctx *lints.LintContext) []lints.LintResult {
	if !LayoutLintEnabled {
		return nil
	}

	var results []lints.LintResult

	ignorePatterns := []string{".g2ignore"}
	g2IgnorePath := filepath.Join(repoDir, ".g2ignore")
	if b, err := os.ReadFile(g2IgnorePath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(b)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				ignorePatterns = append(ignorePatterns, line)
			}
		}
	}

	repoCategories := make(map[string]bool)
	catPath := filepath.Join(repoDir, "profiles", "categories")
	if b, err := os.ReadFile(catPath); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				repoCategories[line] = true
			}
		}
	}

	var upstreamCategories map[string]bool
	if UpstreamRepoPath != "" {
		upstreamCategories = make(map[string]bool)
		catPath := filepath.Join(UpstreamRepoPath, "profiles", "categories")
		if b, err := os.ReadFile(catPath); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					upstreamCategories[line] = true
				}
			}
		}
	} else if AllowGithubAPI {
		upstreamCategories = g2.FetchMainGentooCategories()
	}

	validDirs := map[string]bool{
		"eclass":   true,
		"licenses": true,
		"metadata": true,
		"profiles": true,
		"scripts":  true,
		// standard git files
		".git":    true,
		".github": true,
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		isIgnored := false
		for _, pat := range ignorePatterns {
			if matched, _ := filepath.Match(pat, name); matched {
				isIgnored = true
				break
			}
			if strings.HasSuffix(pat, "/") {
				prefix := pat[:len(pat)-1]
				if matched, _ := filepath.Match(prefix, name); matched && entry.IsDir() {
					isIgnored = true
					break
				}
			}
		}

		if isIgnored {
			continue
		}

		if entry.IsDir() {
			if validDirs[name] {
				continue
			}
			inRepo := repoCategories[name]
			inUpstream := upstreamCategories != nil && upstreamCategories[name]
			if !inRepo && !inUpstream {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleRepoLayout,
					Message:      fmt.Sprintf("Stray directory or unlisted category: '%s'", name),
				})
			}
		} else {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleRepoLayout,
				Message:      fmt.Sprintf("Stray file in repository root: '%s'", name),
			})
		}
	}

	return results
}
