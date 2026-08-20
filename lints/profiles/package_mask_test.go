package profiles

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

//go:embed testdata/*.mask
var testDataFS embed.FS

func TestGlep84FormatLintRule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-lint-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	profilesDir := filepath.Join(tempDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name     string
		file     string
		expected []string // substrings expected in error messages
	}{
		{
			name:     "Valid GLEP 84",
			file:     "testdata/valid_glep84.mask",
			expected: nil,
		},
		{
			name:     "Invalid missing blank line between entries",
			file:     "testdata/invalid_missing_blank_line.mask",
			expected: []string{"mandatory blank line"},
		},
		{
			name:     "Invalid blank line between comments and packages",
			file:     "testdata/invalid_blank_line_comments_packages.mask",
			expected: []string{"No blank line is allowed between comments block and packages list"},
		},
		{
			name:     "Invalid multiple blank lines in comments",
			file:     "testdata/invalid_multiple_blank_lines_comments.mask",
			expected: []string{"Multiple blank lines between paragraphs are prohibited"},
		},
		{
			name:     "Invalid > 80 char comment line",
			file:     "testdata/invalid_80_char.mask",
			expected: []string{"Comment line exceeds 80 characters"},
		},
		{
			name:     "Invalid trailing space in package list",
			file:     "testdata/invalid_trailing_space.mask",
			expected: []string{"leading or trailing whitespace"},
		},
		{
			name:     "Invalid last rite format",
			file:     "testdata/invalid_last_rite_format.mask",
			expected: []string{"Invalid last-rite epilogue format"},
		},
		{
			name:     "Invalid last rite bugs list",
			file:     "testdata/invalid_last_rite_bugs_list.mask",
			expected: []string{"Invalid bugs list format"},
		},
		{
			name:     "Valid GLEP 84 with Separation Lines",
			file:     "testdata/valid_glep84_with_separation.mask",
			expected: nil, // Note: Need to adjust regex to actually match # -------
		},
		{
			name:     "Invalid missing packages list",
			file:     "testdata/invalid_missing_packages_list.mask",
			expected: []string{"Missing packages list in entry"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := testDataFS.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("Failed to read testdata file: %v", err)
			}
			maskPath := filepath.Join(profilesDir, "package.mask")
			if err := os.WriteFile(maskPath, content, 0644); err != nil {
				t.Fatal(err)
			}

			rule := &Glep84FormatLintRule{}
			results := rule.LintRepo(tempDir, &g2.SiteData{})

			if len(tc.expected) == 0 {
				if len(results) > 0 {
					t.Errorf("Expected 0 errors, got %d", len(results))
					for _, r := range results {
						t.Errorf("Error: %s", r.Message)
					}
				}
			} else {
				for _, exp := range tc.expected {
					found := false
					for _, r := range results {
						if strings.Contains(r.Message, exp) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error containing %q, but not found", exp)
						for _, r := range results {
							t.Errorf("Actual error: %s", r.Message)
						}
					}
				}
			}
		})
	}
}
