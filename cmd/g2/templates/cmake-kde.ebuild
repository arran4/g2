EAPI={{.EAPI}}

# ecm eclass is for KDE Frameworks/Plasma/Gear and other projects using extra-cmake-modules.
ECM_TEST="true"
inherit ecm

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{if .Homepage}}{{.Homepage}}{{else}}https://kde.org{{end}}"
SRC_URI="{{if ne .SrcURI "https://example.com/${P}.tar.gz"}}{{.SrcURI}}{{else}}mirror://kde/stable/${PN}/${PV}/${P}.tar.xz{{end}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{if .RDepend}}{{.RDepend}}{{else}}${DEPEND}{{end}}"
BDEPEND="{{.BDepend}}"

# The ecm eclass exports all necessary phases (src_configure, src_compile, src_install, src_test)
# to build standard KDE projects. Customization is typically done via mycmakeargs in src_configure.
