package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"github.com/arran4/g2/lints/ebuild"
)

func TestGenericCopyrightLintRule(t *testing.T) {
	rule := &ebuild.CopyrightHeaderLintRule{}

	tests := []struct {
		name    string
		rawText string
		want    int // number of warnings/notices
	}{
		{"Valid Gentoo Copyright", "# Copyright 1999-2024 Gentoo Authors\nEAPI=8\n", 0},
		{"Valid Another Project", "# Copyright 2026 Example Project Authors\nEAPI=8\n", 0},
		{"Valid Another Foundation", "# Copyright 2020-2026 Example Foundation\nEAPI=8\n", 0},
		{"Missing Copyright", "EAPI=8\n", 1},
		{"Missing Copyright 2", "# build this package carefully\nEAPI=8\n", 1},
		{"License Header", "# Distributed under the terms of the GNU General Public License v2\nEAPI=8\n", 0},
		{"SPDX Header", "# SPDX-License-Identifier: GPL-2.0-or-later\nEAPI=8\n", 0},
		{"Just year in shell comment", "# 2026\n", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: tt.rawText,
						},
					},
				},
			}
			warnings := rule.LintWithRuleSet(".", pkg, "default", nil)
			if len(warnings) != tt.want {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.want, warnings)
			}
		})
	}
}

func TestGentooCopyrightLintRule(t *testing.T) {
	rule := &ebuild.GentooCopyrightHeaderLintRule{}

	tests := []struct {
		name    string
		rawText string
		want    int // number of warnings/notices
	}{
		{"Valid Copyright", "# Copyright 1999-2024 Gentoo Authors\nEAPI=8\n", 0},
		{"Valid Copyright Single Year", "# Copyright 2024 Gentoo Authors\nEAPI=8\n", 0},
		{"Valid Copyright Foundation", "# Copyright 1999-2024 Gentoo Foundation\nEAPI=8\n", 0},
		{"Missing Copyright", "EAPI=8\n", 1},
		{"Malformed Copyright Year", "# Copyright 99-24 Gentoo Authors\nEAPI=8\n", 1},
		{"Malformed Copyright Name", "# Copyright 1999-2024 Some Dude\nEAPI=8\n", 1},
		{"Lowercase Copyright", "# copyright 1999-2024 Gentoo Authors\nEAPI=8\n", 1},
		{"Empty String", "", 0}, // no RawText means no lint since it's checked if != ""
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: tt.rawText,
						},
					},
				},
			}
			warnings := rule.LintWithRuleSet(".", pkg, "gentoo-main", nil)
			if len(warnings) != tt.want {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.want, warnings)
			}
		})
	}
}

func TestCopyrightLintRuleSetIntegration(t *testing.T) {
	ruleGeneric := &ebuild.CopyrightHeaderLintRule{}
	ruleGentoo := &ebuild.GentooCopyrightHeaderLintRule{}

	pkg := &g2.PackageData{
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "# Copyright 2024 Some Dude\nEAPI=8\n",
				},
			},
		},
	}

	// Default ruleset: generic passes, gentoo is not enabled
	warningsGenericDefault := ruleGeneric.LintWithRuleSet(".", pkg, "default", nil)
	if len(warningsGenericDefault) != 0 {
		t.Errorf("expected 0 generic warnings in default, got %d", len(warningsGenericDefault))
	}
	warningsGentooDefault := ruleGentoo.LintWithRuleSet(".", pkg, "default", nil)
	if len(warningsGentooDefault) != 0 {
		t.Errorf("expected 0 gentoo warnings in default, got %d", len(warningsGentooDefault))
	}

	// Gentoo-main ruleset: generic passes, gentoo fails
	warningsGenericGentoo := ruleGeneric.LintWithRuleSet(".", pkg, "gentoo-main", nil)
	if len(warningsGenericGentoo) != 0 {
		t.Errorf("expected 0 generic warnings in gentoo-main, got %d", len(warningsGenericGentoo))
	}
	warningsGentooGentoo := ruleGentoo.LintWithRuleSet(".", pkg, "gentoo-main", nil)
	if len(warningsGentooGentoo) != 1 {
		t.Errorf("expected 1 gentoo warning in gentoo-main, got %d", len(warningsGentooGentoo))
	} else if warningsGentooGentoo[0].RuleMetadata.Severity != lints.SeverityWarning {
		t.Errorf("expected severity Warning for GentooCopyrightHeader, got %s", warningsGentooGentoo[0].RuleMetadata.Severity)
	}
}
