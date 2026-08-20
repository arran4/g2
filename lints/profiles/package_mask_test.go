package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestGlep84FormatLintRule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-lint-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	profilesDir := filepath.Join(tempDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name     string
		content  string
		expected []string // substrings expected in error messages
	}{
		{
			name: "Valid GLEP 84",
			content: `# Copyright 2023 Gentoo Authors
# Uses GLEP 84 format

# Author Name <author@example.com> (2023-10-21)
# Some reason for masking.
cat/pkg

# Another Name <test@test.com> (2023-10-22)
# Reason
#
# Removal on 2023-11-22.  Bugs #123, #456.
dev-util/tool
`,
			expected: nil,
		},
		{
			name: "Invalid missing blank line between entries",
			content: `# Uses GLEP 84 format
# Author Name <author@example.com> (2023-10-21)
# Some reason for masking.
cat/pkg
# Another Name <test@test.com> (2023-10-22)
# Reason
dev-util/tool
`,
			expected: []string{"mandatory blank line"},
		},
		{
			name: "Invalid blank line between comments and packages",
			content: `# Uses GLEP 84 format
# Author Name <author@example.com> (2023-10-21)
# Some reason for masking.

cat/pkg
`,
			expected: []string{"No blank line is allowed between comments block and packages list"},
		},
		{
			name: "Invalid multiple blank lines in comments",
			content: `# Uses GLEP 84 format
# Author Name <author@example.com> (2023-10-21)
# Some reason for masking.
#
#
# Another paragraph.
cat/pkg
`,
			expected: []string{"Multiple blank lines between paragraphs are prohibited"},
		},
		{
			name: "Invalid > 80 char comment line",
			content: `# Uses GLEP 84 format
# Author Name <author@example.com> (2023-10-21)
# Some reason for masking. 12345678901234567890123456789012345678901234567890123456789012345678901234567890
cat/pkg
`,
			expected: []string{"Comment line exceeds 80 characters"},
		},
		{
			name: "Invalid trailing space in package list",
			content: `# Uses GLEP 84 format
# Author Name <author@example.com> (2023-10-21)
# Reason
cat/pkg ` + `
`,
			expected: []string{"leading or trailing whitespace"},
		},
		{
			name: "Invalid last rite format",
			content: `# Uses GLEP 84 format
# Author Name <author@example.com> (2023-10-21)
# Reason
#
# Removal on November 22nd. Bug #123
cat/pkg
`,
			expected: []string{"Invalid last-rite epilogue format"},
		},
		{
			name: "Invalid last rite bugs list",
			content: `# Uses GLEP 84 format
# Author Name <author@example.com> (2023-10-21)
# Reason
#
# Removal on 2023-11-22.  Bug #123, 456
cat/pkg
`,
			expected: []string{"Invalid bugs list format"},
		},
		{
			name: "Valid GLEP 84 with Separation Lines",
			content: `# Copyright 2023 Gentoo Authors
# Uses GLEP 84 format

# Some intro text that should be ignored
# -{5,}.*-{5,}
# -------------------------------------

# Author Name <author@example.com> (2023-10-21)
# Some reason for masking.
cat/pkg
`,
			expected: nil, // Note: Need to adjust regex to actually match # -------
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			maskPath := filepath.Join(profilesDir, "package.mask")
			if err := os.WriteFile(maskPath, []byte(tc.content), 0644); err != nil {
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
