package ebuild_test

import (
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
	"github.com/stretchr/testify/assert"
)

func TestKeywordIUSELintRule(t *testing.T) {
	rule := &ebuild.KeywordIUSELintRule{}
	repoDir := filepath.Join("testdata", "repo")

	tests := []struct {
		name          string
		version       string
		vars          map[string]string
		rawText       string
		expectedCount int
	}{
		{
			name:          "No issues",
			version:       "1.0",
			vars:          map[string]string{"IUSE": "debug test"},
			rawText:       `use debug && echo "debug"`,
			expectedCount: 0,
		},
		{
			name:          "Keyword in IUSE",
			version:       "1.1",
			vars:          map[string]string{"IUSE": "debug x86 test"},
			rawText:       ``,
			expectedCount: 1,
		},
		{
			name:          "Keyword in use check",
			version:       "1.2",
			vars:          map[string]string{"IUSE": "debug test"},
			rawText:       `if use x86; then echo "x86"; fi`,
			expectedCount: 1,
		},
		{
			name:          "Tilde keyword in use check",
			version:       "1.3",
			vars:          map[string]string{"IUSE": "debug test"},
			rawText:       `if use ~x86; then echo "x86"; fi`,
			expectedCount: 1,
		},
		{
			name:          "Keyword in usex check",
			version:       "1.4",
			vars:          map[string]string{"IUSE": "debug test"},
			rawText:       `usex x86`,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgData := &g2.PackageData{
				Name:     "app-test/testpkg",
				Category: "app-test",
				Versions: []g2.VersionData{
					{
						Version: tt.version,
						Ebuild: &g2.Ebuild{
							Vars:    tt.vars,
							RawText: tt.rawText,
						},
					},
				},
			}

			results := rule.Lint(repoDir, pkgData)
			assert.Len(t, results, tt.expectedCount)
		})
	}
}
