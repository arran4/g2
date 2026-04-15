package metadata

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMetadataMissing = lints.RuleMetadata{
	ID:          "MetadataMissing",
	Title:       "Metadata Missing or Invalid",
	Description: "Checks if metadata.xml is missing, invalid XML, or not parsable.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/misc-files/metadata.xml/",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"metadata.xml", "site-quality"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMetadataMissing)
	lints.RegisterLintRule(&MetadataLintRule{})
}

type MetadataLintRule struct{}

func (r *MetadataLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MetadataLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0701"]; ok {
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

	if pkg.MetadataError != nil {
		res := lints.LintResult{
			RuleMetadata: ruleMetadataMissing,
			Message:      fmt.Sprintf("[%s] Invalid or missing metadata.xml: %v", cases.Title(language.Und, cases.NoLower).String(string(severity)), pkg.MetadataError),
			Package:      pkg.Category + "/" + pkg.Name,
		}
		res.RuleMetadata.Severity = severity
		results = append(results, res)
	} else if pkg.Metadata == nil && len(pkg.Versions) > 0 {
		res := lints.LintResult{
			RuleMetadata: ruleMetadataMissing,
			Message:      fmt.Sprintf("[%s] Missing metadata.xml", cases.Title(language.Und, cases.NoLower).String(string(severity))),
			Package:      pkg.Category + "/" + pkg.Name,
		}
		res.RuleMetadata.Severity = severity
		results = append(results, res)
	}

	return results
}
