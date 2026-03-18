EAPI={{.EAPI}}

PYTHON_COMPAT=( {{.PythonCompat}} )
DISTUTILS_USE_PEP517={{.Pep517}}

inherit distutils-r1

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{if ne .SrcURI "https://example.com/${P}.tar.gz"}}{{.SrcURI}}{{else}}mirror://pypi/${PN:0:1}/${PN}/${P}.tar.gz{{end}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"
