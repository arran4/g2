package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestDieSubshellLintRule(t *testing.T) {
	rule := &DieSubshellLintRule{}

	tests := []struct {
		name        string
		eapi        string
		ebuildStr   string
		expectError bool
	}{
		{
			name: "EAPI 6 without die in subshell",
			eapi: "6",
			ebuildStr: `
EAPI=6
die "msg"
`,
			expectError: false,
		},
		{
			name: "EAPI 6 with die in subshell",
			eapi: "6",
			ebuildStr: `
EAPI=6
( die "msg" )
`,
			expectError: true,
		},
		{
			name: "EAPI 7 with die in subshell",
			eapi: "7",
			ebuildStr: `
EAPI=7
( die "msg" )
`,
			expectError: false,
		},
		{
			name: "EAPI 6 with die in pipeline",
			eapi: "6",
			ebuildStr: `
EAPI=6
echo "test" | die "msg"
`,
			expectError: true,
		},
		{
			name: "EAPI 7 with die in pipeline",
			eapi: "7",
			ebuildStr: `
EAPI=7
echo "test" | die "msg"
`,
			expectError: false,
		},
		{
			name: "EAPI 6 with die in command substitution",
			eapi: "6",
			ebuildStr: `
EAPI=6
VAR=$(die "msg")
`,
			expectError: true,
		},
		{
			name: "EAPI 6 with die in background",
			eapi: "6",
			ebuildStr: `
EAPI=6
die "msg" &
`,
			expectError: true,
		},
		{
			name: "EAPI 6 with die in process substitution",
			eapi: "6",
			ebuildStr: `
EAPI=6
diff <(die "1") <(echo bar)
`,
			expectError: true,
		},
		{
			name: "EAPI 6 with die in coproc",
			eapi: "6",
			ebuildStr: `
EAPI=6
coproc foo { die "7"; }
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := map[string]string{}
			if tt.eapi != "" {
				vars["EAPI"] = tt.eapi
			}

			pkg := &g2.PackageData{
				Category: "app-test",
				Name:     "test",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Path:    "/test.ebuild",
							RawText: tt.ebuildStr,
							Vars:    vars,
						},
					},
				},
			}

			results := rule.Lint("", pkg)

			if tt.expectError && len(results) == 0 {
				t.Errorf("expected warning, but got none")
			} else if !tt.expectError && len(results) > 0 {
				t.Errorf("expected no warning, but got one or more")
			}
		})
	}
}
