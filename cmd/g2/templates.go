package main

import (
	"bytes"
	"text/template"
)

type EbuildParams struct {
	EAPI        string
	Description string
	Homepage    string
	SrcURI      string
	License     string
	Slot        string
	Keywords    string
	Depend      string
	RDepend     string
	BDepend     string

	// Specific variants
	PythonCompat string
	Pep517       string
	GoImportPath string
	Crates       string

	// Arbitrary overrides
	Vars map[string]string
}

type Template struct {
	Name        string
	Description string
	Content     string
}

func (t *Template) Execute(params EbuildParams) (string, error) {
	tmpl, err := template.New(t.Name).Parse(t.Content)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var ebuildTemplates = []Template{
	{
		Name:        "default",
		Description: "Default generic ebuild",
		Content: `EAPI={{.EAPI}}

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"

src_compile() {
	emake
}

src_install() {
	emake DESTDIR="${D}" install
}
`,
	},
	{
		Name:        "make",
		Description: "Standard make based ebuild",
		Content: `EAPI={{.EAPI}}

DESCRIPTION="{{.Description}}"
HOMEPAGE="{{.Homepage}}"
SRC_URI="{{.SrcURI}}"

LICENSE="{{.License}}"
SLOT="{{.Slot}}"
KEYWORDS="{{.Keywords}}"

DEPEND="{{.Depend}}"
RDEPEND="{{.RDepend}}"
BDEPEND="{{.BDepend}}"

src_compile() {
	emake
}

src_install() {
	emake DESTDIR="${D}" install
}
`,
	},
	{
		Name:        "go",
		Description: "Go modules ebuild",
		Content: `EAPI={{.EAPI}}

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

src_compile() {
	ego build -o bin/app
}

src_install() {
	dobin bin/app
}
`,
	},
	{
		Name:        "rust",
		Description: "Rust cargo ebuild",
		Content: `EAPI={{.EAPI}}

CRATES="
{{.Crates}}"

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

src_unpack() {
	cargo_src_unpack
}

src_compile() {
	cargo_src_compile
}

src_install() {
	cargo_src_install
}
`,
	},
	{
		Name:        "cmake",
		Description: "CMake ebuild",
		Content: `EAPI={{.EAPI}}

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

src_configure() {
	local mycmakeargs=(
		-DCMAKE_INSTALL_PREFIX="${EPREFIX}/usr"
	)
	cmake_src_configure
}
`,
	},
	{
		Name:        "cmake-kde",
		Description: "CMake KDE ebuild",
		Content: `EAPI={{.EAPI}}

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
`,
	},
	{
		Name:        "python",
		Description: "Python distutils-r1 ebuild",
		Content: `EAPI={{.EAPI}}

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
`,
	},
	{
		Name:        "python-single",
		Description: "Python single target ebuild",
		Content: `EAPI={{.EAPI}}

PYTHON_COMPAT=( {{.PythonCompat}} )

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

src_install() {
	python_doscript myscript.py
}
`,
	},
}

func getTemplate(name string) *Template {
	for i := range ebuildTemplates {
		if ebuildTemplates[i].Name == name {
			return &ebuildTemplates[i]
		}
	}
	return nil
}
