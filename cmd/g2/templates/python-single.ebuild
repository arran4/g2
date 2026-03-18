EAPI={{.EAPI}}

# Specifies the supported Python implementations.
PYTHON_COMPAT=( {{.PythonCompat}} )

# python-single-r1 eclass is used for packages that are meant to be built against a single
# Python implementation (typically tools or applications, rather than libraries).
inherit python-single-r1

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

REQUIRED_USE="${PYTHON_REQUIRED_USE}"
DEPEND="${PYTHON_DEPS} {{.Depend}}"
RDEPEND="${PYTHON_DEPS} {{.RDepend}}"
BDEPEND="{{.BDepend}}"

# Use python_doscript to install executable Python scripts, ensuring they get the
# correct shebang for the active Python implementation.
src_install() {
	python_doscript myscript.py
}
