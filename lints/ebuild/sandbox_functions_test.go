package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func TestSandboxFunctionsLintRule(t *testing.T) {
	rule := &SandboxFunctionsLintRule{}

	tests := []struct {
		name     string
		ebuild   string
		expected []lints.LintResult
	}{
		{
			name: "Valid Usage",
			ebuild: `
src_compile() {
	addpredict /foo
	addread /bar
}
`,
			expected: nil,
		},
		{
			name: "Addwrite Error",
			ebuild: `
src_compile() {
	addwrite /foo
}
`,
			expected: []lints.LintResult{
				{
					Message: "[Error] Ebuild /path/to/ebuild uses addwrite, which is not an appropriate alternative to making the package build sandbox-friendly. Use addpredict instead.",
				},
			},
		},
		{
			name: "Multiple Arguments Warning",
			ebuild: `
src_compile() {
	addpredict /foo /bar
}
`,
			expected: []lints.LintResult{
				{
					Message: "[Warning] Ebuild /path/to/ebuild calls addpredict with multiple arguments. Sandbox functions do not accept multiple arguments in one call.",
				},
			},
		},
		{
			name: "Addwrite Error and Multiple Arguments Warning",
			ebuild: `
src_compile() {
	addwrite /foo /bar
}
`,
			expected: []lints.LintResult{
				{
					Message: "[Error] Ebuild /path/to/ebuild uses addwrite, which is not an appropriate alternative to making the package build sandbox-friendly. Use addpredict instead.",
				},
				{
					Message: "[Warning] Ebuild /path/to/ebuild calls addwrite with multiple arguments. Sandbox functions do not accept multiple arguments in one call.",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgData := &g2.PackageData{
				Category: "sys-apps",
				Name:     "test",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "/path/to/ebuild",
							RawText: tt.ebuild,
						},
					},
				},
			}

			results := rule.Lint("", pkgData)

			if len(results) != len(tt.expected) {
				t.Fatalf("expected %d results, got %d", len(tt.expected), len(results))
			}

			for i, res := range results {
				if res.Message != tt.expected[i].Message {
					t.Errorf("expected message '%s', got '%s'", tt.expected[i].Message, res.Message)
				}
			}
		})
	}
}
