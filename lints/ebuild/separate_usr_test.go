package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func TestSeparateUsrPolicyMetadata(t *testing.T) {
	// Verify PG0202 is registered as metadata
	rules := lints.GetAllRules()
	var found *lints.RuleMetadata
	for i := range rules {
		if rules[i].ID == "SeparateUsr" {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected SeparateUsr to be registered in rule metadata")
	}
	if len(found.References) == 0 || found.References[0].URL == "" {
		t.Errorf("expected SeparateUsr to have valid references, got %v", found.References)
	}

	// Verify installing into /bin, /sbin, /lib is not flagged as a violation
	pkg := &g2.PackageData{
		Category: "dev-libs",
		Name:     "foo",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "dodir /bin\ninto /usr\ninsinto /lib64\n",
				},
			},
		},
	}

	results := lints.PerformLintingResults("", pkg)
	for _, res := range results {
		if res.RuleMetadata.ID == "SeparateUsr" {
			t.Errorf("SeparateUsr should not emit violations for valid /bin or /lib usage: got %v", res)
		}
	}
}
