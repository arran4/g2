package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func TestUseDependenciesMetadata(t *testing.T) {
	rules := lints.GetAllRules()
	var found *lints.RuleMetadata
	for i := range rules {
		if rules[i].ID == "UseDependencies" {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected UseDependencies to be registered in rule metadata")
	}
	if len(found.References) == 0 || found.References[0].URL == "" {
		t.Errorf("expected UseDependencies to have valid references, got %v", found.References)
	}

	// Verify no fake execution results are returned
	pkg := &g2.PackageData{
		Category: "dev-libs",
		Name:     "foo",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{"DEPEND": "dev-libs/bar[ssl]"},
				},
			},
		},
	}
	results := lints.PerformLintingResults("", pkg)
	for _, res := range results {
		if res.RuleMetadata.ID == "UseDependencies" {
			t.Errorf("UseDependencies should not execute as a fake active rule: got %v", res)
		}
	}
}
