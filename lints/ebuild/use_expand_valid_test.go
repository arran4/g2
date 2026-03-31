package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func TestUseExpandLintRule_Lint(t *testing.T) {
	rule := &UseExpandLintRule{}

	// We can't easily mock the ParseUseExpandDescDir because it reads the filesystem directly using g2.ParseUseExpandDescDir.
	// But we can test the structure. Since we don't have a mock filesystem set up in the parameters of this Lint rule,
	// if useExpandDescs is empty, the rule returns no results.
	// This is a known limitation of rules that read global files during linting without an injected FS.

	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "testpkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{
						"IUSE": "foo_bar -foo_baz",
						"REQUIRED_USE": "foo_bar? ( !foo_baz )",
						"DEPEND": "foo_qux? ( dev-libs/libfoo )",
					},
				},
			},
		},
	}

	results := rule.Lint("/nonexistent", pkg)

	// Since /nonexistent/profiles/desc does not exist, g2.ParseUseExpandDescDir returns an empty map,
	// meaning no known prefixes, so no warnings should be generated.
	if len(results) != 0 {
		t.Errorf("Expected 0 results for non-existent profiles/desc, got %d", len(results))
	}
}

func TestUseExpandLintRule_Metadata(t *testing.T) {
	if ruleUseExpandValid.ID != "UseExpandValid" {
		t.Errorf("Expected ID UseExpandValid, got %s", ruleUseExpandValid.ID)
	}
	if ruleUseExpandValid.Severity != lints.SeverityWarning {
		t.Errorf("Expected Warning severity, got %v", ruleUseExpandValid.Severity)
	}
}
