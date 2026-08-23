package ebuild

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleCopyrightHeader = lints.RuleMetadata{
	ID:          "CopyrightHeader",
	Title:       "Missing or Non-Standard Copyright/License Header",
	Description: "Validates that the ebuild starts with an initial comment block containing copyright, license, or SPDX terms.",
	Severity:    lints.SeverityNotice,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "header", "license", "copyright"},
}

var ruleGentooCopyrightHeader = lints.RuleMetadata{
	ID:          "GentooCopyrightHeader",
	Title:       "Gentoo Copyright Header Policy",
	Description: "Validates that the ebuild starts with the Gentoo copyright notice (e.g., '# Copyright <year> Gentoo Authors').",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/general-concepts/copyright-policy/index.html", Label: "Gentoo Devmanual Copyright Policy"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy", "copyright"},
}

func init() {
	lints.RegisterRuleMetadata(ruleCopyrightHeader)
	lints.RegisterLintRule(&CopyrightHeaderLintRule{})

	lints.RegisterRuleMetadata(ruleGentooCopyrightHeader)
	lints.RegisterLintRule(&GentooCopyrightHeaderLintRule{})
}

var (
	// rePlausibleYear matches plausible four-digit years between 1900 and 2099.
	rePlausibleYear = regexp.MustCompile(`\b(19\d\d|20\d\d)\b`)

	// reLicenseTokens matches standalone license identifiers to prevent false positives like "commit" matching "mit".
	reLicenseTokens = regexp.MustCompile(`\b(?i:gpl|lgpl|agpl|bsd|mit|apache|mpl|isc|unlicense|zlib|epl|artistic)\b`)

	// reGentooCopyright matches the complete canonical Gentoo copyright line format.
	// Note: "Gentoo Authors" is required by current Gentoo policy; "Gentoo Foundation"
	// is retained only for backward-compatibility with historical Gentoo ebuilds (prior to 2017).
	// The regex is anchored to disallow trailing garbage.
	reGentooCopyright = regexp.MustCompile(`^#\s*Copyright\s+\d{4}(?:[-\s,]+\d{4})*\s+(?:Gentoo Authors|Gentoo Foundation)\s*$`)
)

// getInitialCommentBlock extracts the initial contiguous shell-comment lines from the top of an ebuild.
// It stops immediately at the first non-comment line (such as empty lines, EAPI declarations, or code).
func getInitialCommentBlock(rawText string) []string {
	var comments []string
	lines := strings.Split(rawText, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			comments = append(comments, trimmed)
		} else {
			break
		}
	}
	return comments
}

// hasGenericCopyrightHeader evaluates whether the initial comment block contains plausible
// copyright, licensing, or SPDX declarations using an evidence-scoring heuristic:
//   - Plausible 4-digit year (1900-2099): +2 points
//   - Explicit copyright phrasing ("copyright", "(c)", "©", "copr."): +2 points
//   - SPDX license identifier: +2 points
//   - Licensing terms ("license", "licence", "licensed", "licenced"): +1 point
//   - Distribution terms ("distributed", "terms"): +1 point
//   - Standalone license token (GPL, BSD, MIT, Apache, etc.): +1 point
//
// A score of >= 2 indicates a plausible header. This allows single strong indicators
// (e.g. a dated comment, an explicit copyright line, or an SPDX header) as well as
// combinations of weaker licensing indicators to pass without requiring Gentoo-specific phrasing.
func hasGenericCopyrightHeader(rawText string) bool {
	comments := getInitialCommentBlock(rawText)
	if len(comments) == 0 {
		return false
	}

	combined := strings.Join(comments, "\n")
	lower := strings.ToLower(combined)

	score := 0

	if rePlausibleYear.MatchString(combined) {
		score += 2
	}

	if strings.Contains(lower, "copyright") || strings.Contains(lower, "(c)") || strings.Contains(combined, "©") || strings.Contains(lower, "copr.") {
		score += 2
	}

	if strings.Contains(lower, "spdx-license-identifier") || strings.Contains(lower, "spdx") {
		score += 2
	}

	if strings.Contains(lower, "license") || strings.Contains(lower, "licence") || strings.Contains(lower, "licensed") || strings.Contains(lower, "licenced") {
		score += 1
	}

	if strings.Contains(lower, "distributed") || strings.Contains(lower, "terms") {
		score += 1
	}

	if reLicenseTokens.MatchString(lower) {
		score += 1
	}

	return score >= 2
}

// hasGentooCopyrightHeader checks if the initial comment block contains a valid Gentoo copyright notice.
func hasGentooCopyrightHeader(rawText string) bool {
	comments := getInitialCommentBlock(rawText)
	if len(comments) == 0 {
		return false
	}
	for _, line := range comments {
		if reGentooCopyright.MatchString(line) {
			return true
		}
	}
	return false
}

// CopyrightHeaderLintRule is a repository-neutral lint rule checking for generic copyright or licensing headers.
type CopyrightHeaderLintRule struct{}

func (r *CopyrightHeaderLintRule) Metadata() lints.RuleMetadata {
	return ruleCopyrightHeader
}

func (r *CopyrightHeaderLintRule) RuleSetManaged() {}

func (r *CopyrightHeaderLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult
	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			if !hasGenericCopyrightHeader(ver.Ebuild.RawText) {
				res := lints.LintResult{
					RuleMetadata: ruleCopyrightHeader,
					Message:      fmt.Sprintf("Ebuild %s has a missing or malformed copyright/licensing header.", ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}
	return results
}

// GentooCopyrightHeaderLintRule validates the Gentoo main repository copyright policy.
type GentooCopyrightHeaderLintRule struct{}

func (r *GentooCopyrightHeaderLintRule) Metadata() lints.RuleMetadata {
	return ruleGentooCopyrightHeader
}

func (r *GentooCopyrightHeaderLintRule) RuleSetManaged() {}

func (r *GentooCopyrightHeaderLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult
	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			if !hasGentooCopyrightHeader(ver.Ebuild.RawText) {
				res := lints.LintResult{
					RuleMetadata: ruleGentooCopyrightHeader,
					Message:      fmt.Sprintf("Ebuild %s has a missing or malformed Gentoo copyright notice.", ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				results = append(results, res)
			}
		}
	}
	return results
}
