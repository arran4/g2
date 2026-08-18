package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestRespectVariablesLintRule(t *testing.T) {
	tests := []struct {
		name          string
		ebuildContent string
		expectError   bool
		variable      string
	}{
		{
			name: "Overwrite CFLAGS unconditionally",
			ebuildContent: `
src_prepare() {
	CFLAGS="-O2"
}`,
			expectError: true,
			variable:    "CFLAGS",
		},
		{
			name: "Overwrite CXXFLAGS unconditionally",
			ebuildContent: `
src_prepare() {
	CXXFLAGS="-O3"
}`,
			expectError: true,
			variable:    "CXXFLAGS",
		},
		{
			name: "Overwrite LDFLAGS unconditionally",
			ebuildContent: `
src_prepare() {
	LDFLAGS="-Wl,-O1"
}`,
			expectError: true,
			variable:    "LDFLAGS",
		},
		{
			name: "Append to CFLAGS",
			ebuildContent: `
src_prepare() {
	CFLAGS+=" -O2"
}`,
			expectError: false,
		},
		{
			name: "Self-reference CFLAGS",
			ebuildContent: `
src_prepare() {
	CFLAGS="${CFLAGS} -O2"
}`,
			expectError: false,
		},
		{
			name: "Self-reference CXXFLAGS",
			ebuildContent: `
src_prepare() {
	CXXFLAGS="$(filter-flags "-O2" "${CXXFLAGS}")"
}`,
			expectError: false,
		},
		{
			name: "Export CFLAGS without value",
			ebuildContent: `
src_prepare() {
	export CFLAGS
}`,
			expectError: false,
		},
		{
			name: "Local CFLAGS overwrite",
			ebuildContent: `
src_prepare() {
	local CFLAGS="-O2"
}`,
			expectError: true,
			variable:    "CFLAGS",
		},
		{
			name: "Export CFLAGS overwrite",
			ebuildContent: `
src_prepare() {
	export CFLAGS="-O2"
}`,
			expectError: true,
			variable:    "CFLAGS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &RespectVariablesLintRule{}
			pkgData := &g2.PackageData{
				Category: "app-misc",
				Name:     "testpkg",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "app-misc/testpkg/testpkg-1.0.ebuild",
							RawText: tt.ebuildContent,
						},
					},
				},
			}

			results := rule.Lint(".", pkgData)

			if tt.expectError {
				if len(results) == 0 {
					t.Fatalf("Expected error for overwriting %s, got none.", tt.variable)
				}
				found := false
				for _, res := range results {
					if res.RuleMetadata.ID == ruleRespectVariables.ID {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected RespectVariables lint error, got: %v", results)
				}
			} else {
				if len(results) > 0 {
					for _, res := range results {
						if res.RuleMetadata.ID == ruleRespectVariables.ID {
							t.Fatalf("Expected no errors, got: %v", res.Message)
						}
					}
				}
			}
		})
	}
}
