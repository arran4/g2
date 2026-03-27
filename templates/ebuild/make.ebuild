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

# The default src_compile function calls emake, which runs `make` in the source directory.
# This requires a Makefile to be present in the unpacked source tree.
src_compile() {
	emake
}

# The default src_install function installs the compiled files into the destination directory.
# It expects the Makefile to support the `install` target and respect the `DESTDIR` variable.
src_install() {
	emake DESTDIR="${D}" install
}
