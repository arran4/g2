package metadata

import (
	"fmt"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleLayoutConfRepo = lints.RuleMetadata{
	ID:          "LayoutConfRepo",
	Title:       "Layout Conf Repository Checks",
	Description: "Ensures the layout.conf file exists and conforms to GLEP 82.",
	URL:         "https://www.gentoo.org/glep/glep-0082.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"repo-layout", "metadata"},
}

func init() {
	lints.RegisterRuleMetadata(ruleLayoutConfRepo)
	lints.RegisterRepoLintRule(&LayoutConfRepoLintRule{})
}

type LayoutConfRepoLintRule struct{}

func (r *LayoutConfRepoLintRule) LintRepo(repoDir string, site *g2.SiteData) []lints.LintResult {
	var results []lints.LintResult

	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	lc, err := g2.ParseLayoutConf(layoutConfPath)
	if err != nil {
		// layout.conf is missing or unparseable, GLEP 82 requires it to exist
		results = append(results, lints.LintResult{
			RuleMetadata: ruleLayoutConfRepo,
			Message:      fmt.Sprintf("[%s] layout.conf is missing or unparseable", lints.SeverityError),
		})
		return results
	}

	// 1. Check masters (Required key)
	if !lc.HasKey("masters") {
		results = append(results, lints.LintResult{
			RuleMetadata: ruleLayoutConfRepo,
			Message:      fmt.Sprintf("[%s] layout.conf is missing the required 'masters' key", lints.SeverityError),
		})
	}

	// 2. Check use-manifests
	if lc.HasKey("use-manifests") {
		um := lc.UseManifests()
		if um != "strict" && um != "true" && um != "false" {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleLayoutConfRepo,
				Message:      fmt.Sprintf("[%s] layout.conf has invalid 'use-manifests' value: %s", lints.SeverityError, um),
			})
		}
	}

	// 3. Check boolean values
	boolKeys := []string{"update-changelog", "thin-manifests", "sign-commits", "sign-manifests"}
	for _, key := range boolKeys {
		if lc.HasKey(key) {
			val := lc.GetValue(key)
			if val != "true" && val != "false" {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleLayoutConfRepo,
					Message:      fmt.Sprintf("[%s] layout.conf has invalid boolean value for '%s': %s", lints.SeverityError, key, val),
				})
			}
		}
	}

	// 4. Check for unknown keys
	validKeys := map[string]bool{
		"masters":                  true,
		"manifest-hashes":          true,
		"manifest-required-hashes": true,
		"use-manifests":            true,
		"update-changelog":         true,
		"cache-formats":            true,
		"eapis-deprecated":         true,
		"eapis-banned":             true,
		"eapis-testing":            true,
		"profile-eapis-deprecated": true,
		"profile-eapis-banned":     true,
		"repo-name":                true,
		"aliases":                  true,
		"thin-manifests":           true,
		"sign-commits":             true,
		"sign-manifests":           true,
		"properties-allowed":       true,
		"restrict-allowed":         true,
		"profile-formats":          true,
	}

	for _, entry := range lc.Entries {
		if !validKeys[entry.Key] {
			res := lints.LintResult{
				RuleMetadata: ruleLayoutConfRepo,
				Message:      fmt.Sprintf("[%s] layout.conf contains unknown key: %s", cases.Title(language.Und, cases.NoLower).String(string(lints.SeverityNotice)), entry.Key),
			}
			res.RuleMetadata.Severity = lints.SeverityNotice
			results = append(results, res)
		}
	}

	// Add package fields for nicer formatting, although it's a repo-level rule
	// Using "metadata/layout.conf" as file indicator
	for i := range results {
		results[i].File = "metadata/layout.conf"
	}

	return results
}
