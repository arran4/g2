package metadata

import (
	"fmt"
	"path/filepath"
	"strings"

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

	// Helper validator for booleans
	validateBool := func(contents ...string) error {
		if len(contents) != 1 {
			return fmt.Errorf("expected 1 value, got %d", len(contents))
		}
		if contents[0] != "true" && contents[0] != "false" {
			return fmt.Errorf("must be 'true' or 'false'")
		}
		return nil
	}

	// Helper validator for space-separated lists
	validateList := func(contents ...string) error {
		// Content is already the raw string, we just consider it valid if it's there
		return nil
	}

	// Validator for hashes
	validateHashes := func(contents ...string) error {
		validHashes := map[string]bool{
			"BLAKE2B":   true,
			"BLAKE2S":   true,
			"SHA512":    true,
			"SHA256":    true,
			"WHIRLPOOL": true,
			"RMD160":    true,
			"SHA1":      true,
			"MD5":       true,
		}
		for _, hash := range contents {
			if !validHashes[hash] {
				return fmt.Errorf("unknown hash algorithm '%s'", hash)
			}
		}
		return nil
	}

	// 4. Check for unknown keys
	validKeys := map[string]func(contents ...string) error{
		"masters":                  validateList,
		"manifest-hashes":          validateHashes,
		"manifest-required-hashes": validateHashes,
		"use-manifests": func(contents ...string) error {
			if len(contents) != 1 {
				return fmt.Errorf("expected 1 value, got %d", len(contents))
			}
			if contents[0] != "strict" && contents[0] != "true" && contents[0] != "false" {
				return fmt.Errorf("must be 'strict', 'true', or 'false'")
			}
			return nil
		},
		"update-changelog":         validateBool,
		"cache-formats":            validateList,
		"eapis-deprecated":         validateList,
		"eapis-banned":             validateList,
		"eapis-testing":            validateList,
		"profile-eapis-deprecated": validateList,
		"profile-eapis-banned":     validateList,
		"repo-name": func(contents ...string) error {
			return nil // Any string is fine
		},
		"aliases":            validateList,
		"thin-manifests":     validateBool,
		"sign-commits":       validateBool,
		"sign-manifests":     validateBool,
		"properties-allowed": validateList,
		"restrict-allowed":   validateList,
		"profile-formats":    validateList,
	}

	for _, entry := range lc.Entries {
		if _, ok := validKeys[entry.Key]; !ok {
			res := lints.LintResult{
				RuleMetadata: ruleLayoutConfRepo,
				Message:      fmt.Sprintf("[%s] layout.conf contains unknown key: %s", cases.Title(language.Und, cases.NoLower).String(string(lints.SeverityNotice)), entry.Key),
			}
			res.RuleMetadata.Severity = lints.SeverityNotice
			results = append(results, res)
		} else if validator := validKeys[entry.Key]; validator != nil {
			// Validate the specific format/content of the key here
			// using the defined func(contents ...string) error.

			// For the validator, pass the fields if the value is meant to be space-separated
			// We pass the fields to match the `contents ...string` signature,
			// except if it's supposed to be treated as a single token we might pass it differently.
			// Let's just pass `strings.Fields(entry.Value)...`.
			// Wait, if it's a single token, `strings.Fields` splits on spaces. This works well for boolean and space-separated lists.
			if err := validator(strings.Fields(entry.Value)...); err != nil {
				res := lints.LintResult{
					RuleMetadata: ruleLayoutConfRepo,
					Message:      fmt.Sprintf("[%s] layout.conf key validation failed %s: %s", lints.SeverityError, entry.Key, err.Error()),
				}
				results = append(results, res)
			}
		}
	}

	// Add package fields for nicer formatting, although it's a repo-level rule
	// Using "metadata/layout.conf" as file indicator
	for i := range results {
		results[i].File = "metadata/layout.conf"
	}

	return results
}
