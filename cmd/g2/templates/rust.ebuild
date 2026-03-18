EAPI={{.EAPI}}

CRATES="
{{.Crates}}"

inherit cargo

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}
	${CARGO_CRATE_URIS}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"

src_unpack() {
	cargo_src_unpack
}

src_compile() {
	cargo_src_compile
}

src_install() {
	cargo_src_install
}
