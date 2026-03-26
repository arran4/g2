package g2

import (
	"embed"
	"encoding/json"
	"encoding/xml"
	"io/fs"
	"path"
	"strings"
	"testing"
	"reflect"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/models/*.txtar
var modelsTestdataFS embed.FS

func assertStableParse(t *testing.T, xmlData []byte, model interface{}) {
	err := xml.Unmarshal(xmlData, model)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	outData, err := xml.MarshalIndent(model, "", "\t")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	model2 := reflect.New(modelType).Interface()

	err = xml.Unmarshal(outData, model2)
	if err != nil {
		t.Fatalf("second unmarshal error: %v", err)
	}

	outData2, err := xml.MarshalIndent(model2, "", "\t")
	if err != nil {
		t.Fatalf("second marshal error: %v", err)
	}

	if string(outData) != string(outData2) {
		t.Fatalf("Mismatch:\nWanted:\n%s\nGot:\n%s", string(outData), string(outData2))
	}
}

func TestModelsTxtar(t *testing.T) {
	entries, err := fs.Glob(modelsTestdataFS, "testdata/models/*.txtar")
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}

	for _, fixture := range entries {
		fixture := fixture
		t.Run(strings.TrimSuffix(path.Base(fixture), ".txtar"), func(t *testing.T) {
			raw, err := modelsTestdataFS.ReadFile(fixture)
			if err != nil {
				t.Fatalf("read fixture %s: %v", fixture, err)
			}
			ar := txtar.Parse(raw)

			var inputData []byte
			for _, f := range ar.Files {
				if f.Name == "input.xml" {
					inputData = f.Data
					break
				}
			}

			if inputData == nil {
				t.Fatalf("input.xml not found in fixture")
			}

			var model interface{}
			if strings.Contains(fixture, "glsa") {
				model = &GLSA{}
			} else if strings.Contains(fixture, "pkgmetadata") {
				model = &PkgMetadata{}
			} else if strings.Contains(fixture, "mirrors") {
				model = &Mirrors{}
			} else if strings.Contains(fixture, "projects") {
				model = &Projects{}
			} else if strings.Contains(fixture, "repositories") {
				model = &Repositories{}
			} else if strings.Contains(fixture, "userinfo") {
				model = &UserList{}
			} else {
				t.Fatalf("unhandled model type for %s", fixture)
			}

			assertStableParse(t, inputData, model)

			var expectedJSON []byte
			for _, f := range ar.Files {
				if f.Name == "expected.json" {
					expectedJSON = f.Data
					break
				}
			}

			if expectedJSON != nil {
				importJSON, _ := json.MarshalIndent(model, "", "\t")
				if strings.TrimSpace(string(importJSON)) != strings.TrimSpace(string(expectedJSON)) {
					t.Fatalf("JSON mismatch\nWanted:\n%s\nGot:\n%s", string(expectedJSON), string(importJSON))
				}
			}
		})
	}
}
