package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestCopyrightLintRule(t *testing.T) {
	rule := &ebuild.CopyrightLintRule{}

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
			warnings := rule.Lint(".", pkg)
			if len(warnings) != tt.want {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.want, warnings)
			}
		})
	}
}
