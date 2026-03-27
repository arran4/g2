EAPI={{.EAPI}}

# Specifies the supported Python implementations.
PYTHON_COMPAT=( {{.PythonCompat}} )
# Defines the PEP 517 build backend to use (e.g., setuptools, flit_core, hatchling).
DISTUTILS_USE_PEP517={{.Pep517}}

# distutils-r1 eclass manages the building and installation of Python packages
# across multiple Python implementations.
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

# The distutils-r1 eclass automatically handles the configure, compile, and install phases
# using the specified PEP 517 backend.
