package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestPythonCompatLintRule(t *testing.T) {
	rule := &PythonCompatLintRule{}

	tests := []struct {
		name          string
		pkg           *g2.PackageData
		qaPolicy      *g2.QAPolicy
		expectedCount int
		expectedMsgs  []string
	}{
		{
			name: "Valid Python ebuild",
			pkg: &g2.PackageData{
				Category: "dev-python",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"INHERITED":     "python-r1",
								"PYTHON_COMPAT": "python3_9 python3_10",
							},
						},
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Missing PYTHON_COMPAT (PG0501)",
			pkg: &g2.PackageData{
				Category: "dev-python",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"INHERITED": "python-single-r1",
							},
						},
					},
				},
			},
			expectedCount: 1,
			expectedMsgs:  []string{"[Error] Ebuild 1.0 inherits a python eclass but does not define PYTHON_COMPAT (PG0501)."},
		},
		{
			name: "Python 2 usage (PG0502)",
			pkg: &g2.PackageData{
				Category: "dev-python",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"INHERITED":     "python-any-r1",
								"PYTHON_COMPAT": "python2_7 python3_9",
							},
						},
					},
				},
			},
			expectedCount: 1,
			expectedMsgs:  []string{"[Warning] Ebuild 1.0 defines Python 2 compatibility 'python2_7' in PYTHON_COMPAT (PG0502)."},
		},
		{
			name: "Missing PYTHON_COMPAT with QA ignore",
			pkg: &g2.PackageData{
				Category: "dev-python",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"INHERITED": "python-single-r1",
							},
						},
					},
				},
			},
			qaPolicy: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0501": "ignore",
				},
			},
			expectedCount: 0,
		},
		{
			name: "Python 2 usage with QA error",
			pkg: &g2.PackageData{
				Category: "dev-python",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"INHERITED":     "python-r1",
								"PYTHON_COMPAT": "python2_7 python3_9",
							},
						},
					},
				},
			},
			qaPolicy: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0502": "error",
				},
			},
			expectedCount: 1,
			expectedMsgs:  []string{"[Error] Ebuild 1.0 defines Python 2 compatibility 'python2_7' in PYTHON_COMPAT (PG0502)."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := rule.LintWithQA("", tt.pkg, tt.qaPolicy)

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}

			if len(tt.expectedMsgs) > 0 {
				for i, expectedMsg := range tt.expectedMsgs {
					if i < len(results) {
						if results[i].Message != expectedMsg {
							t.Errorf("Expected message: %s\nGot: %s", expectedMsg, results[i].Message)
						}
					}
				}
			}
		})
	}
}
