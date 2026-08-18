package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestEbuildHeaderLintRule(t *testing.T) {
	rule := &EbuildHeaderLintRule{}

	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "test",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					EbuildHeader: "# Copyright 1999-2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2",
				},
			},
			{
				Version: "2.0",
				Ebuild: &g2.Ebuild{
					EbuildHeader: "# Copyright 1999-2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n# $Id$",
				},
			},
			{
				Version: "3.0",
				Ebuild: &g2.Ebuild{
					EbuildHeader: "random string",
				},
			},
		},
	}

	results := rule.Lint("", pkg)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, result := range results {
		if result.RuleMetadata.ID != "EbuildHeader" {
			t.Errorf("expected rule ID EbuildHeader, got %s", result.RuleMetadata.ID)
		}
	}
}
