package ebuild_test

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestBannedCommandsLintRule(t *testing.T) {
	rule := &ebuild.BannedCommandsLintRule{}

	tests := []struct {
		name         string
		eapi         string
		script       string
		expectedErrs int
		expectedWarn int
	}{
		{
			name:         "EAPI 5 valid",
			eapi:         "5",
			script:       "src_install() { einstall; dohtml foo.html; dolib foo.so; libopts -m0644; hasq foo bar; hasv foo bar; useq foo; assert \"error\"; domo foo.mo; }",
			expectedErrs: 0,
			expectedWarn: 0,
		},
		{
			name:         "EAPI 6 einstall banned",
			eapi:         "6",
			script:       "src_install() { einstall; }",
			expectedErrs: 1,
			expectedWarn: 0,
		},
		{
			name:         "EAPI 6 dohtml deprecated",
			eapi:         "6",
			script:       "src_install() { dohtml foo.html; }",
			expectedErrs: 0,
			expectedWarn: 1,
		},
		{
			name:         "EAPI 7 dohtml banned",
			eapi:         "7",
			script:       "src_install() { dohtml foo.html; }",
			expectedErrs: 1,
			expectedWarn: 0,
		},
		{
			name:         "EAPI 7 dolib and libopts banned",
			eapi:         "7",
			script:       "src_install() { dolib foo.so; libopts -m0644; }",
			expectedErrs: 2,
			expectedWarn: 0,
		},
		{
			name:         "EAPI 8 hasq, hasv, useq banned",
			eapi:         "8",
			script:       "src_install() { hasq foo bar; hasv foo bar; useq foo; }",
			expectedErrs: 3,
			expectedWarn: 0,
		},
		{
			name:         "EAPI 9 assert and domo banned",
			eapi:         "9",
			script:       "src_install() { assert \"error\"; domo foo.mo; }",
			expectedErrs: 2,
			expectedWarn: 0,
		},
		{
			name:         "EAPI 9 all banned",
			eapi:         "9",
			script:       "src_install() { einstall; dohtml foo.html; dolib foo.so; libopts -m0644; hasq foo bar; hasv foo bar; useq foo; assert \"error\"; domo foo.mo; }",
			expectedErrs: 9,
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
							Vars: map[string]string{
								"EAPI": tt.eapi,
							},
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
