package g2

import (
	"strings"
	"testing"
)

func TestEbuildBuilder(t *testing.T) {
	b := NewEbuildBuilder()
	b.SetHeader("# Copyright 1999-2023 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2")
	b.SetEAPI("8")
	b.SetVariable("DESCRIPTION", "My awesome package")
	b.SetVariable("HOMEPAGE", "https://example.com")
	b.AddSrcURI("https://example.com/foo.tar.gz", "foo-1.0.tar.gz")
	b.SetFunction("src_prepare", "{\n\tdefault\n}")

	e := b.Build()
	out := e.String()

	expected := `# Copyright 1999-2023 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI="8"
DESCRIPTION="My awesome package"
HOMEPAGE="https://example.com"
src_prepare() {
	default
}
SRC_URI="
	https://example.com/foo.tar.gz -> foo-1.0.tar.gz
"`

	if strings.TrimSpace(out) != strings.TrimSpace(expected) {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, out)
	}
}
