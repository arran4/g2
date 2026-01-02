# Ebuild with variables
MY_PN="mypackage"
MY_PV="1.0"
S="${WORKDIR}/${MY_PN}-${MY_PV}"
SRC_URI="https://example.com/${MY_PN}-${MY_PV}.tar.gz -> ${P}.tar.gz"
