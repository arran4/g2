package ebuild_test

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestUseConditionalCodeLintRule(t *testing.T) {
	rule := &ebuild.UseConditionalCodeLintRule{}

	tests := []struct {
		name         string
		script       string
		expectedErrs int
		expectedWarn int
	}{
		{
			name:         "Standard use",
			script:       "if use foo; then\n  echo \"foo\"\nfi",
			expectedErrs: 0,
			expectedWarn: 0,
		},
		{
			name:         "Inverse use",
			script:       "if ! use foo; then\n  echo \"no foo\"\nfi",
			expectedErrs: 0,
			expectedWarn: 0,
		},
		{
			name:         "Deprecated useq",
			script:       "if useq foo; then\n  echo \"foo\"\nfi",
			expectedErrs: 0,
			expectedWarn: 1,
		},
		{
			name:         "use inside backticks (invalid)",
			script:       "if [ \"`use foo`\" ]; then\n  echo \"foo\"\nfi",
			expectedErrs: 1,
			expectedWarn: 0,
		},
		{
			name:         "use inside $() (invalid)",
			script:       "if [ -n \"$(use foo)\" ]; then\n  echo \"foo\"\nfi",
			expectedErrs: 1,
			expectedWarn: 0,
		},
		{
			name:         "Both useq and use inside command substitution",
			script:       "useq foo\n[ \"`use foo`\" ]",
			expectedErrs: 1,
			expectedWarn: 1,
		},
		{
			name:         "Valid use inside cmd substitution (exit code check)",
			script:       "MY_CONF=\"$(use foo && echo \"--enable-foo\")\"",
			expectedErrs: 0,
			expectedWarn: 0,
		},
		{
			name:         "Valid use inside if in cmd substitution",
			script:       "VAR=$(if use qt5; then echo \"qt5\"; fi)",
			expectedErrs: 0,
			expectedWarn: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "sys-apps",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars:    map[string]string{},
							RawText: tt.script,
						},
					},
				},
			}
			results := rule.Lint(".", pkg)

			errs := 0
			warns := 0
			for _, res := range results {
				if strings.HasPrefix(res.Message, "[Error]") {
					errs++
				} else if strings.HasPrefix(res.Message, "[Warning]") {
					warns++
				}
			}

			if errs != tt.expectedErrs {
				t.Errorf("expected %d errors, got %d. Results: %v", tt.expectedErrs, errs, results)
			}
			if warns != tt.expectedWarn {
				t.Errorf("expected %d warnings, got %d. Results: %v", tt.expectedWarn, warns, results)
			}
		})
	}
}
