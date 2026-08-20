package metadata

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleProxiedMaintainer = lints.RuleMetadata{
	ID:          "ProxiedMaintainer",
	Title:       "Proxied Maintainer Rules",
	Description: "Ensures that packages maintained by proxied maintainers explicitly list their proxy developer or project.",
	URL:         "https://devmanual.gentoo.org/general-concepts/package-maintainers/index.html#adding-and-removing-maintainers",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"metadata.xml", "site-quality", "PG0704"}, // Using a new tag, e.g., PG0704
}

func init() {
	lints.RegisterRuleMetadata(ruleProxiedMaintainer)
	lints.RegisterLintRule(&ProxiedMaintainerLintRule{})
}

type ProxiedMaintainerLintRule struct{}

func (r *ProxiedMaintainerLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *ProxiedMaintainerLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0704"]; ok {
			if val == "ignore" {
				return nil
			}
			if val == "notice" || val == "error" || val == "warning" {
				switch val {
				case "notice":
					severity = lints.SeverityNotice
				case "error":
					severity = lints.SeverityError
				case "warning":
					severity = lints.SeverityWarning
				}
			}
		}
	}

	if pkg.Metadata != nil {
		hasProxied := false
		hasProxy := false

		for _, m := range pkg.Metadata.Maintainers {
			if m.Proxied == "yes" {
				hasProxied = true
			}
			if m.Proxied == "proxy" {
				hasProxy = true
			}
		}

		if hasProxied && !hasProxy {
			res := lints.LintResult{
				RuleMetadata: ruleProxiedMaintainer,
				Message:      fmt.Sprintf("[%s] Package has a proxied maintainer but no proxy developer/project is listed", cases.Title(language.Und, cases.NoLower).String(string(severity))),
				Package:      pkg.Category + "/" + pkg.Name,
			}
			res.RuleMetadata.Severity = severity
			results = append(results, res)
		}

		if hasProxy && !hasProxied {
			res := lints.LintResult{
				RuleMetadata: ruleProxiedMaintainer,
				Message:      fmt.Sprintf("[%s] Package has a proxy developer/project but no proxied maintainer is listed", cases.Title(language.Und, cases.NoLower).String(string(severity))),
				Package:      pkg.Category + "/" + pkg.Name,
			}
			res.RuleMetadata.Severity = severity
			results = append(results, res)
		}
	}

	return results
}
