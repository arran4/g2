EAPI={{.EAPI}}

# cmake eclass handles configuration, building, and installation of CMake-based projects.
inherit cmake

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"

# configure the CMake project. mycmakeargs can be used to pass specific options to CMake.
src_configure() {
	local mycmakeargs=(
		-DCMAKE_INSTALL_PREFIX="${EPREFIX}/usr"
	)
	cmake_src_configure
}

# Note: cmake_src_compile and cmake_src_install are exported by the cmake eclass
# and will be used automatically to build and install the project.
