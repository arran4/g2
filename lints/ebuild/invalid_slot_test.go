package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestInvalidSlotLintRule(t *testing.T) {
	rule := &ebuild.InvalidSlotLintRule{}

	tests := []struct {
		name string
		slot string
		want int
	}{
		{"Valid zero", "0", 0},
		{"Valid slot", "1", 0},
		{"Valid subslot", "0/1", 0},
		{"Valid complex", "1.2_alpha-r1", 0},
		{"Invalid character", "1:2", 1},
		{"Invalid character space", "1 2", 1},
		{"Invalid multiple slashes", "1/2/3", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"SLOT": tt.slot,
							},
						},
					},
				},
			}
			warnings := rule.Lint(".", pkg)
			if len(warnings) != tt.want {
				t.Errorf("got %d warnings, want %d", len(warnings), tt.want)
			}
		})
	}
}
