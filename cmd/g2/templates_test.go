package main

import (
	"bytes"
	"testing"
	"golang.org/x/tools/txtar"
)

func TestEbuildTemplatesTxtar(t *testing.T) {
	archive, err := txtar.ParseFile("testdata/templates.txtar")
	if err != nil {
		t.Fatalf("failed to parse txtar file: %v", err)
	}

	for _, tpl := range ebuildTemplates {
		t.Run(tpl.Name, func(t *testing.T) {
			params := EbuildParams{
				EAPI:         "8",
				Description:  "A short description",
				Homepage:     "https://example.com",
				SrcURI:       "https://example.com/${P}.tar.gz",
				License:      "GPL-2",
				Slot:         "0",
				Keywords:     "~amd64",
				Depend:       "",
				RDepend:      "",
				BDepend:      "",
				PythonCompat: "python3_{10..12}",
				Pep517:       "setuptools",
				Crates:       "rand-0.8.5",
			}

			if tpl.Name == "cmake-kde" {
			    params.Homepage = ""
			} else if tpl.Name == "python" || tpl.Name == "python-single" {
			    params.SrcURI = ""
			}

			content, err := tpl.Execute(params)
			if err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			var expectedContent []byte
			for _, file := range archive.Files {
				if file.Name == tpl.Name+".ebuild" {
					expectedContent = file.Data
					break
				}
			}

			if expectedContent == nil {
				t.Fatalf("no expected content found in txtar for %s.ebuild", tpl.Name)
			}

			if !bytes.Equal([]byte(content), expectedContent) {
				t.Errorf("template %s output did not match expected.\nExpected:\n%s\nGot:\n%s\n", tpl.Name, expectedContent, content)
			}
		})
	}
}
