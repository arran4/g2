package ebuild

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func TestInstalledFilesLintRule(t *testing.T) {
	rule := &InstalledFilesLintRule{}

	testCases := []struct {
		name          string
		pkg           *g2.PackageData
		qaPolicy      *g2.QAPolicy
		expectedCount int
		expectedMsg   string
	}{
		{
			name: "Valid unconditional small files and manpages",
			pkg: &g2.PackageData{
				Category: "app-test",
				Name:     "good-files",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: `
src_install() {
	dobashcomp file.sh
	doman doc.1
	if use static-libs; then
		dolib.a lib.a
	fi
}
`,
						},
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Invalid conditional small file PG0301",
			pkg: &g2.PackageData{
				Category: "app-test",
				Name:     "bad-small",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: `
src_install() {
	use bash-completion && dobashcomp file.sh
}
`,
						},
					},
				},
			},
			expectedCount: 1,
			expectedMsg:   "Small files must be installed unconditionally",
		},
		{
			name: "Invalid conditional manpage PG0305",
			pkg: &g2.PackageData{
				Category: "app-test",
				Name:     "bad-manpage",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: `
src_install() {
	if use doc; then
		doman doc.1
	fi
}
`,
						},
					},
				},
			},
			expectedCount: 1,
			expectedMsg:   "Manpages must be installed unconditionally",
		},
		{
			name: "Invalid unconditional static lib PG0302",
			pkg: &g2.PackageData{
				Category: "app-test",
				Name:     "bad-static-lib",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: `
src_install() {
	dolib.a lib.a
}
`,
						},
					},
				},
			},
			expectedCount: 1,
			expectedMsg:   "calls 'dolib.a' unconditionally",
		},
		{
			name: "Ignored QA policies",
			pkg: &g2.PackageData{
				Category: "app-test",
				Name:     "ignored-qa",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: `
src_install() {
	use bash-completion && dobashcomp file.sh
	use doc && doman doc.1
	dolib.a lib.a
}
`,
						},
					},
				},
			},
			qaPolicy: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0301": "ignore",
					"PG0302": "ignore",
					"PG0305": "ignore",
				},
			},
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := rule.LintWithQA("", tc.pkg, tc.qaPolicy)

			if len(results) != tc.expectedCount {
				t.Fatalf("Expected %d results, got %d", tc.expectedCount, len(results))
			}

			if tc.expectedCount > 0 && tc.expectedMsg != "" {
				found := false
				for _, res := range results {
					if res.RuleMetadata.ID == "InstalledSmallFiles" || res.RuleMetadata.ID == "InstalledManpages" || res.RuleMetadata.ID == "InstalledStaticLibs" {
						if len(res.Message) > 0 && (res.RuleMetadata.Severity == lints.SeverityWarning || res.RuleMetadata.Severity == lints.SeverityError) {
							if strings.Contains(res.Message, tc.expectedMsg) {
								found = true
								break
							}
						}
					}
				}
				if !found {
					t.Errorf("Expected message containing '%s', got: %v", tc.expectedMsg, results)
				}
			}
		})
	}
}
