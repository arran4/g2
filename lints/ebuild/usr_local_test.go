package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestUsrLocalLintRule(t *testing.T) {
	tests := []struct {
		name     string
		ebuild   string
		expected int
	}{
		{
			name: "Valid Installation",
			ebuild: `
src_install() {
	into /usr
	insinto /usr/bin
	dodir /usr/share
}
			`,
			expected: 0,
		},
		{
			name: "Invalid into /usr/local",
			ebuild: `
src_install() {
	into /usr/local
}
			`,
			expected: 1,
		},
		{
			name: "Invalid insinto /usr/local/bin",
			ebuild: `
src_install() {
	insinto /usr/local/bin
}
			`,
			expected: 1,
		},
		{
			name: "Invalid dodir with EPREFIX",
			ebuild: `
src_install() {
	dodir "${EPREFIX}/usr/local/share"
}
			`,
			expected: 1,
		},
		{
			name: "Valid docinto",
			ebuild: `
src_install() {
	docinto html
}
			`,
			expected: 0,
		},
	}

	rule := &UsrLocalLintRule{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-misc",
				Name:     "test",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Path:    "app-misc/test/test-1.0.ebuild",
							RawText: tc.ebuild,
						},
					},
				},
			}

			results := rule.Lint(".", pkg, nil)

			if len(results) != tc.expected {
				t.Fatalf("expected %d issues, got %d. Results: %v", tc.expected, len(results), results)
			}

			for _, result := range results {
				if result.RuleMetadata.ID != ruleUsrLocal.ID {
					t.Errorf("expected rule ID %s, got %s", ruleUsrLocal.ID, result.RuleMetadata.ID)
				}
			}
		})
	}
}
