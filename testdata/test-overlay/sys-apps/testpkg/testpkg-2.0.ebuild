EAPI=8

DESCRIPTION="Test Package 2.0"
HOMEPAGE="https://example.com/testpkg"
SRC_URI="https://example.com/testpkg-2.0.tar.gz"

LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~amd64 ~x86"
IUSE="test doc"

DEPEND="dev-libs/libfoo
    test? ( dev-util/testframework )"
RDEPEND="${DEPEND}
    sys-apps/sed"
BDEPEND="virtual/pkgconfig"

inherit git-r3

pkg_setup() {
    echo "setup"
}

src_unpack() {
    git-r3_src_unpack
}

src_compile() {
    emake
}

src_install() {
    emake DESTDIR="${D}" install
}
