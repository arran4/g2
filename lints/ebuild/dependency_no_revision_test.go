package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/stretchr/testify/assert"
)

func TestDependencyNoRevisionLintRule(t *testing.T) {
	rule := &DependencyNoRevisionLintRule{}

	tests := []struct {
		name          string
		ebuildVars    map[string]string
		expectedCount int
	}{
		{
			name: "No warning for explicit revision",
			ebuildVars: map[string]string{
				"DEPEND": "=dev-libs/libfrobnicate-1.2.3-r0",
			},
			expectedCount: 0,
		},
		{
			name: "No warning for explicit revision >0",
			ebuildVars: map[string]string{
				"DEPEND": "=dev-libs/libfrobnicate-1.2.3-r3",
			},
			expectedCount: 0,
		},
		{
			name: "No warning for tilde operator",
			ebuildVars: map[string]string{
				"DEPEND": "~dev-libs/libfrobnicate-1.2.3",
			},
			expectedCount: 0,
		},
		{
			name: "No warning for wildcard",
			ebuildVars: map[string]string{
				"DEPEND": "=dev-libs/libfrobnicate-1.2*",
			},
			expectedCount: 0,
		},
		{
			name: "Warning for non-wildcard equals dependency without revision",
			ebuildVars: map[string]string{
				"DEPEND": "=dev-libs/libfrobnicate-1.2.3",
			},
			expectedCount: 1,
		},
		{
			name: "Warning for multiple dependencies without revision",
			ebuildVars: map[string]string{
				"DEPEND": "=dev-libs/libfrobnicate-1.2.3\n	=dev-libs/another-1.0",
			},
			expectedCount: 2,
		},
		{
			name: "Warning inside block",
			ebuildVars: map[string]string{
				"DEPEND": "\n	|| (\n		=dev-libs/libfrobnicate-1.2.3\n		=dev-libs/another-1.0-r1\n	)\n",
			},
			expectedCount: 1,
		},
		{
			name: "No warning for non-equals",
			ebuildVars: map[string]string{
				"DEPEND": ">=dev-libs/libfrobnicate-1.2.3",
			},
			expectedCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-test",
				Name:     "testpkg",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: tc.ebuildVars,
						},
					},
				},
			}

			results := rule.Lint("", pkg)
			assert.Len(t, results, tc.expectedCount)
			if tc.expectedCount > 0 {
				assert.Contains(t, results[0].Message, "uses a non-wildcard '=' dependency without an explicit revision")
			}
		})
	}
}

func TestDependencyNoRevisionLintRule_FalsePositives(t *testing.T) {
	rule := &DependencyNoRevisionLintRule{}

	tests := []struct {
		name          string
		ebuildVars    map[string]string
		expectedCount int
	}{
		{
			name: "Warning for package with -r in category name but no revision",
			ebuildVars: map[string]string{
				"DEPEND": "=app-runner/libfrobnicate-1.2.3",
			},
			expectedCount: 1,
		},
		{
			name: "Warning for package with -r in package name but no revision",
			ebuildVars: map[string]string{
				"DEPEND": "=dev-libs/runner-1.2.3",
			},
			expectedCount: 1,
		},
		{
			name: "Warning for package with -r in version suffix but no revision",
			ebuildVars: map[string]string{
				"DEPEND": "=dev-libs/libfrobnicate-1.2.3_rc1",
			},
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-test",
				Name:     "testpkg",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: tc.ebuildVars,
						},
					},
				},
			}

			results := rule.Lint("", pkg)
			assert.Len(t, results, tc.expectedCount)
			if tc.expectedCount > 0 {
				assert.Contains(t, results[0].Message, "uses a non-wildcard '=' dependency without an explicit revision")
			}
		})
	}
}
