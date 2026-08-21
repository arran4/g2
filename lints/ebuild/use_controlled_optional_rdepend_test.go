package ebuild

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestUseControlledOptionalRdependRule(t *testing.T) {
	rule := &UseControlledOptionalRdependRule{}

	tests := []struct {
		name     string
		pkg      *g2.PackageData
		qaPolicy *g2.QAPolicy
		expected int
	}{
		{
			name: "No USE-controlled RDEPENDs",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"RDEPEND": "dev-libs/A dev-libs/B",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 0,
		},
		{
			name: "USE-controlled RDEPEND (Pure)",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"RDEPEND": "a? ( dev-libs/A ) dev-libs/B",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 1, // Flag 'a' is only in RDEPEND
		},
		{
			name: "USE-controlled RDEPEND but used in DEPEND",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"DEPEND":  "a? ( dev-libs/A-headers )",
								"RDEPEND": "a? ( dev-libs/A )",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 0, // Flag 'a' is also in DEPEND, so it configures the build
		},
		{
			name: "USE-controlled RDEPEND but used in BDEPEND",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"BDEPEND": "a? ( dev-util/A-tools )",
								"RDEPEND": "a? ( dev-libs/A )",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 0, // Flag 'a' is also in BDEPEND
		},
		{
			name: "USE-controlled RDEPEND but used in ebuild body",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"RDEPEND": "ssl? ( dev-libs/openssl )",
							},
							RawText: `
src_configure() {
    econf $(use_with ssl)
}
`,
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 0, // Flag 'ssl' is used in src_configure
		},
		{
			name: "Multiple USE-controlled RDEPENDs, mixed",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"DEPEND":  "b? ( dev-libs/B )",
								"RDEPEND": "a? ( dev-libs/A ) !b? ( dev-libs/B ) c? ( dev-libs/C )",
							},
							RawText: `
src_compile() {
    if use c; then
        emake with-c
    fi
}
`,
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 1, // 'a' is pure RDEPEND, 'b' is in DEPEND, 'c' is in body
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := rule.LintWithQA("", tc.pkg, tc.qaPolicy)

			if len(results) != tc.expected {
				t.Errorf("Expected %d results, got %d", tc.expected, len(results))
				for _, res := range results {
					t.Logf("Result: %s", res.Message)
				}
			}
		})
	}
}

func TestUseControlledOptionalRdependRule_PG0001(t *testing.T) {
	rule := &UseControlledOptionalRdependRule{}
	pkg := &g2.PackageData{
		Category: "dev-libs",
		Name:     "foo",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{
						"RDEPEND": "flag? ( dev-libs/bar )",
					},
					RawText: "foo",
				},
			},
		},
	}

	qaErr := &g2.QAPolicy{Policies: map[string]string{"PG0001": "error"}}
	resultsErr := rule.LintWithQA("", pkg, qaErr)
	if len(resultsErr) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resultsErr))
	}
	if !strings.Contains(resultsErr[0].Message, "(PG0001)") {
		t.Errorf("expected message to contain (PG0001), got: %s", resultsErr[0].Message)
	}

	qaIgnore := &g2.QAPolicy{Policies: map[string]string{"PG0001": "ignore"}}
	resultsIgnore := rule.LintWithQA("", pkg, qaIgnore)
	if len(resultsIgnore) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(resultsIgnore))
	}
}
