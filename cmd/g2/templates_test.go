package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/tools/txtar"
	"testing"
)

func TestEbuildTemplatesTxtar(t *testing.T) {
	for _, tpl := range ebuildTemplates {
		t.Run(tpl.Name, func(t *testing.T) {
			archive, err := txtar.ParseFile(fmt.Sprintf("testdata/templates/%s.txtar", tpl.Name))
			if err != nil {
				t.Fatalf("failed to parse txtar file: %v", err)
			}

			var inputData []byte
			var expectedContent []byte

			for _, file := range archive.Files {
				switch file.Name {
				case "input.json":
					inputData = file.Data
				case "output.ebuild":
					expectedContent = file.Data
				}
			}

			if inputData == nil {
				t.Fatalf("missing input.json in txtar for %s", tpl.Name)
			}
			if expectedContent == nil {
				t.Fatalf("missing output.ebuild in txtar for %s", tpl.Name)
			}

			var params EbuildParams
			if err := json.Unmarshal(inputData, &params); err != nil {
				t.Fatalf("failed to unmarshal input.json: %v", err)
			}

			content, err := tpl.Execute(params)
			if err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			if !bytes.Equal([]byte(content), expectedContent) {
				t.Errorf("template %s output did not match expected.\nExpected:\n%s\nGot:\n%s\n", tpl.Name, expectedContent, content)
			}
		})
	}
}
