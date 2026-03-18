EAPI={{.EAPI}}

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
