EAPI={{.EAPI}}

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"

src_compile() {
	emake
}

src_install() {
	emake DESTDIR="${D}" install
}
