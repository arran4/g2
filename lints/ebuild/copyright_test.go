package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"github.com/arran4/g2/lints/ebuild"
)

func TestGenericCopyrightHeaderLintRule(t *testing.T) {
	rule := &ebuild.CopyrightHeaderLintRule{}

	tests := []struct {
		name    string
		rawText string
		want    int // expected number of results
	}{
		{
			name:    "Gentoo Copyright Header",
			rawText: "# Copyright 1999-2024 Gentoo Authors\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Third-Party Authors",
			rawText: "# Copyright 2026 Example Project Authors\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Third-Party Foundation",
			rawText: "# Copyright 2020-2026 Example Foundation\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "License terms header without year",
			rawText: "# Distributed under the terms of the GNU General Public License v2\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "SPDX header without year",
			rawText: "# SPDX-License-Identifier: GPL-2.0-or-later\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Just year in initial comment block",
			rawText: "# 2026\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Explicit copyright symbol and year",
			rawText: "# Copyright (c) 2024 Acme Corp\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "MIT license declaration in header",
			rawText: "# MIT License\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Missing header (starts with EAPI)",
			rawText: "EAPI=8\n",
			want:    1,
		},
		{
			name:    "Random comment without copyright or license evidence",
			rawText: "# build this package carefully\nEAPI=8\n",
			want:    1,
		},
		{
			name:    "Avoid substring false positive: commit containing mit",
			rawText: "# do not commit this file\nEAPI=8\n",
			want:    1,
		},
		{
			name:    "Avoid substring false positive: smith containing mit",
			rawText: "# smith's custom build\nEAPI=8\n",
			want:    1,
		},
		{
			name:    "Year outside initial comment block does not count",
			rawText: "EAPI=8\n# 2026\n",
			want:    1,
		},
		{
			name:    "Copyright outside initial comment block does not count",
			rawText: "EAPI=8\n# Copyright 2026 Gentoo Authors\n",
			want:    1,
		},
		{
			name:    "License keyword in code/variable does not count",
			rawText: "EAPI=8\nDESCRIPTION=\"MIT licensed utility\"\n",
			want:    1,
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
							RawText: tt.rawText,
						},
					},
				},
			}
			results := rule.Lint("", pkg)
			if len(results) != tt.want {
				t.Errorf("got %d results, want %d: %v", len(results), tt.want, results)
			}
			if len(results) > 0 && results[0].RuleMetadata.Severity != lints.SeverityNotice {
				t.Errorf("expected Notice severity, got %s", results[0].RuleMetadata.Severity)
			}
		})
	}
}

func TestGentooCopyrightHeaderLintRule(t *testing.T) {
	rule := &ebuild.GentooCopyrightHeaderLintRule{}

	tests := []struct {
		name    string
		rawText string
		want    int
	}{
		{
			name:    "Valid Gentoo Authors with year range",
			rawText: "# Copyright 1999-2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Valid Gentoo Authors single year",
			rawText: "# Copyright 2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Valid Gentoo Authors multiple years",
			rawText: "# Copyright 1999, 2001, 2003-2005 Gentoo Authors\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Valid Gentoo Foundation historical",
			rawText: "# Copyright 1999-2020 Gentoo Foundation\nEAPI=8\n",
			want:    0,
		},
		{
			name:    "Non-Gentoo authors fails Gentoo policy",
			rawText: "# Copyright 2026 Example Project Authors\nEAPI=8\n",
			want:    1,
		},
		{
			name:    "Arbitrary non-Gentoo person fails Gentoo policy",
			rawText: "# Copyright 2024 Jane Doe\nEAPI=8\n",
			want:    1,
		},
		{
			name:    "Missing header fails Gentoo policy",
			rawText: "EAPI=8\n",
			want:    1,
		},
		{
			name:    "Gentoo Authors with trailing garbage rejected",
			rawText: "# Copyright 1999-2024 Gentoo Authors trailing garbage\nEAPI=8\n",
			want:    1,
		},
		{
			name:    "Gentoo Foundation historical with trailing garbage rejected",
			rawText: "# Copyright 1999-2020 Gentoo Foundation trailing garbage\nEAPI=8\n",
			want:    1,
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
							RawText: tt.rawText,
						},
					},
				},
			}
			results := rule.Lint("", pkg)
			if len(results) != tt.want {
				t.Errorf("got %d results, want %d: %v", len(results), tt.want, results)
			}
			if len(results) > 0 && results[0].RuleMetadata.Severity != lints.SeverityWarning {
				t.Errorf("expected Warning severity for GentooCopyrightHeader, got %s", results[0].RuleMetadata.Severity)
			}
		})
	}
}

func TestCopyrightHeaderRuleSetDispatchIntegration(t *testing.T) {
	pkgNonGentoo := &g2.PackageData{
		Category: "app-misc",
		Name:     "custom-pkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "# Copyright 2026 Jane Doe\n# Distributed under the terms of the BSD 3-Clause License\n\nEAPI=8\n",
				},
			},
		},
	}

	// 1. Under "default": non-Gentoo copyright passes cleanly.
	resDefault, err := lints.PerformLintingResultsWithRuleSet("", pkgNonGentoo, "default")
	if err != nil {
		t.Fatalf("unexpected error running default ruleset: %v", err)
	}
	for _, res := range resDefault {
		if res.RuleMetadata.ID == "CopyrightHeader" || res.RuleMetadata.ID == "GentooCopyrightHeader" {
			t.Errorf("unexpected copyright failure under default ruleset: %v", res.Message)
		}
	}

	// 2. Under "guru": non-Gentoo copyright passes cleanly.
	resGuru, err := lints.PerformLintingResultsWithRuleSet("", pkgNonGentoo, "guru")
	if err != nil {
		t.Fatalf("unexpected error running guru ruleset: %v", err)
	}
	for _, res := range resGuru {
		if res.RuleMetadata.ID == "CopyrightHeader" || res.RuleMetadata.ID == "GentooCopyrightHeader" {
			t.Errorf("unexpected copyright failure under guru ruleset: %v", res.Message)
		}
	}

	// 3. Under "gentoo-main": generic passes, but GentooCopyrightHeader produces a Warning.
	resGentoo, err := lints.PerformLintingResultsWithRuleSet("", pkgNonGentoo, "gentoo-main")
	if err != nil {
		t.Fatalf("unexpected error running gentoo-main ruleset: %v", err)
	}
	var foundGentooWarning bool
	for _, res := range resGentoo {
		if res.RuleMetadata.ID == "CopyrightHeader" {
			t.Errorf("generic CopyrightHeader should pass for valid non-Gentoo header")
		}
		if res.RuleMetadata.ID == "GentooCopyrightHeader" {
			foundGentooWarning = true
			if res.RuleMetadata.Severity != lints.SeverityWarning {
				t.Errorf("expected Warning severity, got %s", res.RuleMetadata.Severity)
			}
		}
	}
	if !foundGentooWarning {
		t.Errorf("expected GentooCopyrightHeader warning under gentoo-main")
	}
}
