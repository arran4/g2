package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/stretchr/testify/assert"
)

func TestIndentingWhitespaceLintRule(t *testing.T) {
	rule := &IndentingWhitespaceLintRule{}

	t.Run("Valid ebuild", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "good-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						RawText: "# Copyright 1999-2024 Gentoo Authors\n" +
							"# Distributed under the terms of the GNU General Public License v2\n\n" +
							"EAPI=8\n\n" +
							"DESCRIPTION=\"A valid package\"\n" +
							"HOMEPAGE=\"https://example.com\"\n" +
							"SRC_URI=\"\"\n\n" +
							"LICENSE=\"GPL-2\"\n" +
							"SLOT=\"0\"\n" +
							"KEYWORDS=\"~amd64\"\n\n" +
							"src_prepare() {\n" +
							"\tdefault\n" +
							"}\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Empty(t, results)
	})

	t.Run("Trailing whitespace", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "trailing-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						RawText: "EAPI=8 \n" +
							"DESCRIPTION=\"A valid package\"\t\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Len(t, results, 3)
		assert.Contains(t, results[0].Message, "trailing whitespace on line 1")
		assert.Contains(t, results[1].Message, "trailing whitespace on line 2")
		assert.Contains(t, results[2].Message, "tab character outside of indentation on line 2")
	})

	t.Run("Leading spaces for indentation", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "spaces-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						RawText: "src_prepare() {\n" +
							"  default\n" +
							"}\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Message, "uses spaces for indentation on line 2")
	})

	t.Run("Mixed spaces and tabs for indentation", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "mixed-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						RawText: "src_prepare() {\n" +
							"\t  default\n" +
							"}\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Message, "uses spaces for indentation on line 2")
	})

	t.Run("Ignores spaces after other text", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "spaces-after-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						RawText: "src_prepare() {\n" +
							"\tdefault # comments can have spaces\n" +
							"}\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Empty(t, results)
	})

	t.Run("Tab after indent", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "tab-after-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						RawText: "DESCRIPTION=\"A\tpackage\"\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Message, "has a tab character outside of indentation on line 1")
	})

	t.Run("Line length > 80 positions", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "long-line-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						// 81 'a's
						RawText: "DESCRIPTION=\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Message, "has a line exceeding 80 positions on line 1")
	})

	t.Run("Line length with tabs", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "long-line-tabs-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						// 20 tabs (80 positions) + 1 char = 81 positions
						RawText: "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\ta\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Message, "has a line exceeding 80 positions on line 1")
	})
}
