package g2

import (
	_ "embed"
	"golang.org/x/tools/txtar"
	"strings"
	"testing"
)

//go:embed testdata/ebuild_builder_golden.txtar
var ebuildBuilderGolden []byte

func TestEbuildBuilderIdiomatic(t *testing.T) {
	arc := txtar.Parse(ebuildBuilderGolden)

	var expected string
	for _, f := range arc.Files {
		if f.Name == "expected.ebuild" {
			expected = string(f.Data)
			break
		}
	}

	e := BuildEbuild(
		EbuildHeader("# Copyright 1999-2023 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2"),
		EbuildEAPI("8"),
		EbuildDescription("My awesome package"),
		EbuildHomepage("https://example.com"),
		EbuildLicense("GPL-2"),
		EbuildSlot("0/1"),
		EbuildKeywords{"~amd64", "~x86"},
		EbuildIUse{"doc", "test"},
		EbuildRestrict{"fetch", "test"},
		EbuildRequiredUse("foo? ( || ( bar baz quux ) )"),
		EbuildDepend("dev-libs/foo"),
		EbuildRDepend("dev-libs/bar"),
		EbuildInherit{"eutils", "user"},
		EbuildFunc{Name: "src_prepare", Body: "{\n\tdefault\n\tewarn \"This is a warning\"\n}"},
		EbuildFunc{Name: "pkg_setup", Body: "{\n\tenewgroup mygroup\n\tenewuser myuser -1 -1 -1 mygroup\n}"},
		EbuildSrcURIRaw("https://example.com/files/${P}-core.tar.bz2\n\tx86?   ( https://example.com/files/${P}/${P}-sse-asm.tar.bz2 )\n\tppc?   ( https://example.com/files/${P}/${P}-vmx-asm.tar.bz2 )\n\tsparc? ( https://example.com/files/${P}/${P}-vis-asm.tar.bz2 )\n\tdoc?   ( https://example.com/files/${P}/${P}-docs.tar.bz2 )"),
	)

	out := e.String()

	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, out)
	}
}
