package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleIUSEDocumented = lints.RuleMetadata{
	ID:          "IuseDocumented",
	Title:       "IUSE Documented",
	Description: "Verifies that all flags in IUSE have corresponding descriptions in metadata.xml or global/local use desc.",
	URL:         "https://devmanual.gentoo.org/general-concepts/use-flags/",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"metadata.xml", "ebuild", "site-quality"},
}

func init() {
	lints.RegisterLintRule(&IUSELintRule{})
}

type IUSELintRule struct{}

func (r *IUSELintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *IUSELintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

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

	metadataIUSE := make(map[string]bool)
	if pkg.Metadata != nil {
		for _, useBlock := range pkg.Metadata.Use {
			for _, flag := range useBlock.Flags {
				metadataIUSE[flag.Name] = true
			}
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			iuse := ver.Ebuild.Vars["IUSE"]
			if iuse != "" {
				flags := strings.Fields(iuse)
				for _, flag := range flags {
					flag = strings.TrimPrefix(flag, "+")
					flag = strings.TrimPrefix(flag, "-")
					if flag != "" {
						if _, exists := metadataIUSE[flag]; !exists {
							res := lints.LintResult{
								RuleMetadata: ruleIUSEDocumented,
								Message:      fmt.Sprintf("[%s] USE flag '%s' in ebuild %s is not documented in metadata.xml", cases.Title(language.English).String(string(severity)), flag, ver.Version),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				}
			}
		}
	}

	return results
}
