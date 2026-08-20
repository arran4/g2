package metadata_test

import (
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/metadata"
)

func TestProjectsXMLLintRule(t *testing.T) {
	rule := &metadata.ProjectsXMLLintRule{}

	t.Run("Valid projects.xml", func(t *testing.T) {
		site := &g2.SiteData{
			Projects: &g2.Projects{
				Projects: []g2.Project{
					{Email: "parent@example.com", Subprojects: []g2.Subproject{{Ref: "child@example.com"}}},
					{Email: "child@example.com"},
				},
			},
		}
		warnings := rule.LintRepo(".", site)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})

	t.Run("Invalid subproject ref", func(t *testing.T) {
		site := &g2.SiteData{
			Projects: &g2.Projects{
				Projects: []g2.Project{
					{Email: "parent@example.com", Subprojects: []g2.Subproject{{Ref: "missing@example.com"}}},
				},
			},
		}
		warnings := rule.LintRepo(".", site)
		if len(warnings) == 0 {
			t.Error("expected warning for missing subproject ref, got none")
		} else {
			if warnings[0].RuleMetadata.ID != "ProjectsXML" {
				t.Errorf("expected ProjectsXML rule, got %s", warnings[0].RuleMetadata.ID)
			}
			expectedFile := filepath.Join("metadata", "projects.xml")
			if warnings[0].File != expectedFile {
				t.Errorf("expected file %s, got %s", expectedFile, warnings[0].File)
			}
		}
	})
}
