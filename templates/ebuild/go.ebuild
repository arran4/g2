EAPI={{.EAPI}}

# go-module handles downloading and unpacking go modules based on go.mod and go.sum.
inherit go-module

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"

# ego is a wrapper around the standard `go` command that sets up the proper environment variables
# for offline building using the downloaded modules.
src_compile() {
	ego build -o bin/app
}

# dobin installs the compiled binary into /usr/bin.
src_install() {
	dobin bin/app
}
