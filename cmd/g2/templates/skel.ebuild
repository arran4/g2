# Copyright 1999-2025 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

# Skeleton command:
# java-ebuilder --generate-ebuild --workdir . --pom pom.xml --download-uri https://github.com/Gentoo/gentoo/archive/refs/tags/v1.0.tar.gz --slot 0 --keywords "~amd64" --ebuild skel.ebuild

EAPI={{.EAPI}}

# QA check: ensure the EAPI is explicitly set above and do NOT set it again.
# The ebuild must be named <name>-<version>.ebuild.
# Avoid using ${P} or ${PN} in HOMEPAGE or SRC_URI if possible, use literal strings instead.

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

# IUSE should be defined if the ebuild uses any USE flags.
# IUSE=""

# DEPEND contains build-time dependencies.
DEPEND="{{.Depend}}"

# RDEPEND contains runtime dependencies.
# If RDEPEND is the same as DEPEND, you can use RDEPEND="${DEPEND}"
RDEPEND="{{.RDepend}}"

# BDEPEND contains tool dependencies required for building.
BDEPEND="{{.BDepend}}"

# The following functions are placeholders for the default ebuild phases.
# You can override them to customize the build process.
# Refer to the Gentoo Development Guide (devmanual) for detailed information.

# src_unpack() {
# 	default
# }

# src_prepare() {
# 	default
# }

# src_configure() {
# 	default
# }

# src_compile() {
# 	default
# }

# src_test() {
# 	default
# }

# src_install() {
# 	default
# }
