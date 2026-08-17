package ebuild

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestPythonSingleR1UseDep(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		expected []lints.LintResult
	}{
		{
			name:     "Valid python_gen_cond_dep usage",
			filename: "foo-1.0.ebuild",
			content: `EAPI=8
inherit python-single-r1
RDEPEND="$(python_gen_cond_dep 'dev-python/bar[${PYTHON_USEDEP}]')"
`,
			expected: nil,
		},
		{
			name:     "Invalid PYTHON_USEDEP usage",
			filename: "foo-2.0.ebuild",
			content: `EAPI=8
inherit python-single-r1
RDEPEND="dev-python/bar[${PYTHON_USEDEP}]"
`,
			expected: []lints.LintResult{
				{
					RuleMetadata: rulePythonSingleR1UseDep,
					Message:      "[Error] Ebuild 2.0 inherits python-single-r1 and uses PYTHON_USEDEP without python_gen_cond_dep.",
					Package:      "app-misc/foo",
				},
			},
		},
		{
			name:     "Both valid and invalid usage",
			filename: "foo-3.0.ebuild",
			content: `EAPI=8
inherit python-single-r1
RDEPEND="
	$(python_gen_cond_dep 'dev-python/valid[${PYTHON_USEDEP}]')
	dev-python/invalid[${PYTHON_USEDEP}]
"
`,
			expected: []lints.LintResult{
				{
					RuleMetadata: rulePythonSingleR1UseDep,
					Message:      "[Error] Ebuild 3.0 inherits python-single-r1 and uses PYTHON_USEDEP without python_gen_cond_dep.",
					Package:      "app-misc/foo",
				},
			},
		},
		{
			name:     "No python-single-r1 inheritance",
			filename: "foo-4.0.ebuild",
			content: `EAPI=8
inherit python-r1
RDEPEND="dev-python/bar[${PYTHON_USEDEP}]"
`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memFS := fstest.MapFS{
				tt.filename: &fstest.MapFile{Data: []byte(tt.content)},
			}

			ebuildData, err := g2.ParseEbuild(memFS, tt.filename, g2.ParseFull)
			if err != nil {
				t.Fatalf("Failed to parse ebuild %s: %v", tt.filename, err)
			}

			pkg := &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: strings.TrimSuffix(tt.filename[4:], ".ebuild"),
						Ebuild:  ebuildData,
					},
				},
			}

			rule := &PythonSingleR1UseDepLintRule{}
			results := rule.Lint(".", pkg)

			if diff := cmp.Diff(tt.expected, results, cmpopts.IgnoreFields(lints.RuleMetadata{}, "Tags")); diff != "" {
				t.Errorf("Lint results mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
