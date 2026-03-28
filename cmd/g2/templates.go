package main

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/arran4/g2/templates"
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
}

func (t *Template) Execute(params EbuildParams) (string, error) {
	content, err := templates.EbuildFS.ReadFile(fmt.Sprintf("ebuild/%s.ebuild", t.Name))
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(t.Name).Parse(string(content))
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
	},
	{
		Name:        "skel",
		Description: "Gentoo standard skeleton ebuild",
	},
	{
		Name:        "make",
		Description: "Standard make based ebuild",
	},
	{
		Name:        "go",
		Description: "Go modules ebuild",
	},
	{
		Name:        "rust",
		Description: "Rust cargo ebuild",
	},
	{
		Name:        "cmake",
		Description: "CMake ebuild",
	},
	{
		Name:        "cmake-kde",
		Description: "CMake KDE ebuild",
	},
	{
		Name:        "python",
		Description: "Python distutils-r1 ebuild",
	},
	{
		Name:        "python-single",
		Description: "Python single target ebuild",
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
