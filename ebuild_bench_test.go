package g2

import (
	"os"
	"testing"
)

func BenchmarkParseEbuildFull_WithWeak(b *testing.B) {
	fsys := os.DirFS(".")

	ebuildContent := []byte(`
EAPI=8
DESCRIPTION="Dummy package"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/file.tar.gz"
LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~amd64 ~x86"
IUSE="test"

DEPEND="
	dev-libs/libxml2
	test? ( dev-util/cppunit )
"
RDEPEND="${DEPEND}"
`)
	_ = os.WriteFile("dummy-1.0.ebuild", ebuildContent, 0644)
	defer func() { _ = os.Remove("dummy-1.0.ebuild") }()

	EnableWeakEbuildContent = true
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseEbuild(fsys, "dummy-1.0.ebuild", ParseFull)
	}
}

func BenchmarkParseEbuildFull_WithoutWeak(b *testing.B) {
	fsys := os.DirFS(".")

	ebuildContent := []byte(`
EAPI=8
DESCRIPTION="Dummy package"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/file.tar.gz"
LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~amd64 ~x86"
IUSE="test"

DEPEND="
	dev-libs/libxml2
	test? ( dev-util/cppunit )
"
RDEPEND="${DEPEND}"
`)
	_ = os.WriteFile("dummy-1.0.ebuild", ebuildContent, 0644)
	defer func() { _ = os.Remove("dummy-1.0.ebuild") }()

	EnableWeakEbuildContent = false
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseEbuild(fsys, "dummy-1.0.ebuild", ParseFull)
	}
}

func BenchmarkResolveVariables(b *testing.B) {
    vars := map[string]string{
        "A": "B",
        "C": "D",
        "E": "F",
        "P": "pkg-1.0",
        "PN": "pkg",
        "PV": "1.0",
    }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        ResolveVariables("This is a test with $P and ${PN} variables", vars)
    }
}

func BenchmarkExtractPackageNameFromDep(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ExtractPackageNameFromDep("dev-lang/python")
		ExtractPackageNameFromDep(">=dev-lang/python-3.10.4-r1:0/3.10[sqlite,xml]")
	}
}
