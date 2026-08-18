package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestInvalidVirtual(t *testing.T) {
	tests := []struct {
		name     string
		category string
		version  string
		vars     map[string]string
		want     int
	}{
		{"Valid virtual", "virtual", "1.0", map[string]string{"EAPI": "8"}, 0},
		{"Non-virtual with homepage", "app-misc", "1.0", map[string]string{"HOMEPAGE": "https://example.com"}, 0},
		{"Virtual with homepage", "virtual", "1.0", map[string]string{"HOMEPAGE": "https://example.com"}, 1},
		{"Virtual with license", "virtual", "1.0", map[string]string{"LICENSE": "GPL-2"}, 1},
		{"Virtual with SRC_URI", "virtual", "1.0", map[string]string{"SRC_URI": "https://example.com/file.tar.gz"}, 1},
		{"Virtual with multiple invalid", "virtual", "1.0", map[string]string{"HOMEPAGE": "https://example.com", "LICENSE": "GPL-2"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: tt.category,
				Name:     "testpkg",
				Versions: []g2.VersionData{
					{
						Version: tt.version,
						Ebuild: &g2.Ebuild{
							Vars: tt.vars,
						},
					},
				},
			}

			rule := &InvalidVirtualLintRule{}
			results := rule.Lint("", pkg)

			if len(results) != tt.want {
				t.Errorf("Lint() returned %d results, want %d", len(results), tt.want)
			}
		})
	}
}
