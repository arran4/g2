package ebuild_test

import (
	"embed"
	"testing"
    "testing/fstest"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

//go:embed testdata/*.ebuild
var testdataFS embed.FS

func TestNetworkSandboxLintRule(t *testing.T) {
	rule := &ebuild.NetworkSandboxLintRule{}

	tests := []struct {
		name     string
		category string
		version  string
		filename string
		want     int
	}{
		{"No network-sandbox", "app-misc", "1.0", "testdata/no-network-sandbox.ebuild", 0},
		{"With network-sandbox", "app-misc", "1.0", "testdata/with-network-sandbox.ebuild", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := testdataFS.ReadFile(tt.filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", tt.filename, err)
			}

            memFS := fstest.MapFS{
				tt.filename: {Data: content},
			}

			ebuildData, err := g2.ParseEbuild(memFS, tt.filename, g2.ParseVariables)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", tt.filename, err)
			}

			pkg := &g2.PackageData{
				Category: tt.category,
				Versions: []g2.VersionData{
					{
						Version: tt.version,
						Ebuild:  ebuildData,
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
