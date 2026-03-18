EAPI={{.EAPI}}

# CRATES defines the specific versions of all Rust crates this package depends on.
# These will be automatically downloaded by the cargo eclass.
CRATES="
{{.Crates}}"

# cargo eclass sets up the environment to build Rust projects using Cargo.
inherit cargo

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}
	${CARGO_CRATE_URIS}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"

# Unpack the main source tarball and all the specified Rust crates.
src_unpack() {
	cargo_src_unpack
}

# Compile the project using cargo build.
src_compile() {
	cargo_src_compile
}

# Install the built binaries into the destination directory.
src_install() {
	cargo_src_install
}
