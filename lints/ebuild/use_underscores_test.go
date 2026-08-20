package ebuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func TestUseUnderscoresLintRule(t *testing.T) {
	rule := &UseUnderscoresLintRule{}

	// Setup a temporary repo structure to test USE_EXPAND logic
	tempDir := t.TempDir()
	descDir := filepath.Join(tempDir, "profiles", "desc")
	err := os.MkdirAll(descDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create a dummy desc file
	videoCardsDesc := filepath.Join(descDir, "video_cards.desc")
	err = os.WriteFile(videoCardsDesc, []byte(`
nvidia - NVIDIA cards
radeon - AMD/ATI cards
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		iuse     string
		expected int // Number of expected lint results
		qa       *g2.QAPolicy
	}{
		{
			name:     "No underscores",
			iuse:     "test doc",
			expected: 0,
		},
		{
			name:     "Has underscore but it is a valid USE_EXPAND",
			iuse:     "video_cards_nvidia video_cards_radeon",
			expected: 0,
		},
		{
			name:     "Has underscore and not USE_EXPAND",
			iuse:     "my_flag doc",
			expected: 1,
		},
		{
			name:     "Multiple flags with underscore and not USE_EXPAND",
			iuse:     "+bad_flag1 -bad_flag2",
			expected: 2,
		},
		{
			name:     "Ignored via QA policy",
			iuse:     "bad_flag",
			expected: 0,
			qa: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0803": "ignore",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-test",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"IUSE": tt.iuse,
							},
						},
					},
				},
			}

			results := rule.LintWithQA(tempDir, pkg, tt.qa)
			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
				for _, r := range results {
					t.Errorf("Message: %s", r.Message)
				}
			}
		})
	}
}
