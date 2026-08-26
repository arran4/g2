package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"github.com/stretchr/testify/assert"
)

func TestNilAssignmentRules(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		rule             lints.LintRule
		expectError      bool
		expectedFindings int
	}{
		{
			name: "SRC_URI empty",
			content: `EAPI=8
local SRC_URI
SRC_URI=
SRC_URI=()
SRC_URI=""`,
			rule:             &SrcUriHomepageLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name: "SRC_URI normal",
			content: `EAPI=8
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/foo.tar.gz"`,
			rule:             &SrcUriHomepageLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name: "SRC_URI homepage",
			content: `EAPI=8
HOMEPAGE="https://example.com"
SRC_URI="${HOMEPAGE}/foo.tar.gz"`,
			rule:             &SrcUriHomepageLintRule{},
			expectError:      true,
			expectedFindings: 1,
		},
		{
			name: "HOMEPAGE empty",
			content: `EAPI=8
local HOMEPAGE
HOMEPAGE=
HOMEPAGE=()
HOMEPAGE=""`,
			rule:             &HomepageVariablesLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name: "HOMEPAGE normal",
			content: `EAPI=8
HOMEPAGE="https://example.com"`,
			rule:             &HomepageVariablesLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name: "HOMEPAGE var",
			content: `EAPI=8
HOMEPAGE="${MY_HOMEPAGE}"`,
			rule:             &HomepageVariablesLintRule{},
			expectError:      true,
			expectedFindings: 1,
		},
		{
			name: "LICENSE empty",
			content: `EAPI=8
local LICENSE
LICENSE=
LICENSE=()
LICENSE=""`,
			rule:             &LicenseVariablesLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name: "LICENSE normal",
			content: `EAPI=8
LICENSE="GPL-2"`,
			rule:             &LicenseVariablesLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name: "LICENSE var",
			content: `EAPI=8
LICENSE="${MY_LICENSE}"`,
			rule:             &LicenseVariablesLintRule{},
			expectError:      true,
			expectedFindings: 1,
		},
		{
			name: "KEYWORDS empty",
			content: `EAPI=8
local KEYWORDS
KEYWORDS=
KEYWORDS=()
KEYWORDS=""`,
			rule:             &KeywordsSingleLineLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name: "KEYWORDS normal",
			content: `EAPI=8
KEYWORDS="amd64"`,
			rule:             &KeywordsSingleLineLintRule{},
			expectError:      false,
			expectedFindings: 0,
		},
		{
			name:             "KEYWORDS multiline",
			content:          "EAPI=8\nKEYWORDS=\"amd64\nx86\"",
			rule:             &KeywordsSingleLineLintRule{},
			expectError:      true,
			expectedFindings: 1, // Only the newline warning
		},
		{
			name: "KEYWORDS append",
			content: `EAPI=8
KEYWORDS="amd64"
KEYWORDS+=" x86"`,
			rule:             &KeywordsSingleLineLintRule{},
			expectError:      true,
			expectedFindings: 2,
		},
		{
			name: "KEYWORDS var",
			content: `EAPI=8
KEYWORDS="${MY_KEYWORDS}"`,
			rule:             &KeywordsSingleLineLintRule{},
			expectError:      true,
			expectedFindings: 1,
		},
		{
			name: "KEYWORDS multiple combined with empty",
			content: `EAPI=8
local KEYWORDS
KEYWORDS=
KEYWORDS=()
KEYWORDS="amd64"
KEYWORDS="x86"`,
			rule:             &KeywordsSingleLineLintRule{},
			expectError:      true,
			expectedFindings: 1, // At most once warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: tt.content,
						},
					},
				},
			}

			// We can call either Lint or LintWithQA depending on what is implemented
			var results []lints.LintResult
			if qaRule, ok := tt.rule.(lints.QAAwareLintRule); ok {
				results = qaRule.LintWithQA("", pkg, nil)
			} else {
				results = tt.rule.Lint("", pkg)
			}

			if tt.expectError {
				assert.NotEmpty(t, results, "Expected lint findings but got none")
				assert.Len(t, results, tt.expectedFindings, "Expected %d findings, got %d", tt.expectedFindings, len(results))
			} else {
				assert.Empty(t, results, "Expected no lint findings, but got %v", results)
			}
		})
	}
}
