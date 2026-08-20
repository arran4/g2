package metadata

import (
	"fmt"
	"path/filepath"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleProjectsXML = lints.RuleMetadata{
	ID:          "ProjectsXML",
	Title:       "Projects XML Rules",
	Description: "Validates metadata/projects.xml structural integrity.",
	URLs:        []string{"https://www.gentoo.org/glep/glep-0067.html#projects-xml-and-herds-xml"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"projects.xml", "site-quality", "GLEP67"},
}

func init() {
	lints.RegisterRuleMetadata(ruleProjectsXML)
	lints.RegisterRepoLintRule(&ProjectsXMLLintRule{})
}

type ProjectsXMLLintRule struct{}

func (r *ProjectsXMLLintRule) LintRepo(repoDir string, site *g2.SiteData) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if site != nil && site.Projects != nil {
		projectEmails := make(map[string]bool)
		for _, proj := range site.Projects.Projects {
			projectEmails[proj.Email] = true
		}

		for _, proj := range site.Projects.Projects {
			for _, sub := range proj.Subprojects {
				if !projectEmails[sub.Ref] {
					res := lints.LintResult{
						RuleMetadata: ruleProjectsXML,
						Message:      fmt.Sprintf("[%s] Subproject ref '%s' in project '%s' does not exist", cases.Title(language.Und, cases.NoLower).String(string(severity)), sub.Ref, proj.Email),
						File:         filepath.Join("metadata", "projects.xml"),
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}
			}
		}
	}

	return results
}
