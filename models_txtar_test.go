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

//go:embed testdata/models/*.txtar testdata/models_json/*.txtar
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

			var inputXMLData []byte
			var inputJSONData []byte
			var expectedXMLData []byte

			for i := range ar.Files {
				switch ar.Files[i].Name {
				case "input.xml":
					inputXMLData = ar.Files[i].Data
				case "input.json":
					inputJSONData = ar.Files[i].Data
				case "expected.xml":
					expectedXMLData = ar.Files[i].Data
				}
			}

			if inputXMLData == nil && inputJSONData == nil {
				t.Fatalf("neither input.xml nor input.json found in fixture")
			}

			var model interface{}
			switch {
			case strings.Contains(fixture, "glsa"):
				model = &GLSA{}
			case strings.Contains(fixture, "pkgmetadata"):
				model = &PkgMetadata{}
			case strings.Contains(fixture, "mirrors"):
				model = &Mirrors{}
			case strings.Contains(fixture, "projects"):
				model = &Projects{}
			case strings.Contains(fixture, "repositories"):
				model = &Repositories{}
			case strings.Contains(fixture, "userinfo"):
				model = &UserList{}
			default:
				t.Fatalf("unhandled model type for %s", fixture)
			}

			if inputXMLData != nil {
				assertStableParse(t, inputXMLData, model)

				var expectedJSON []byte
				for i := range ar.Files {
					if ar.Files[i].Name == "expected.json" {
						expectedJSON = ar.Files[i].Data
						break
					}
				}

				if expectedJSON != nil {
					importJSON, _ := json.MarshalIndent(model, "", "\t")
					if strings.TrimSpace(string(importJSON)) != strings.TrimSpace(string(expectedJSON)) {
						t.Fatalf("JSON mismatch\nWanted:\n%s\nGot:\n%s", string(expectedJSON), string(importJSON))
					}
				}
			} else if inputJSONData != nil && expectedXMLData != nil {
				err = json.Unmarshal(inputJSONData, model)
				if err != nil {
					t.Fatalf("unmarshal error: %v", err)
				}

				importXML, err := xml.MarshalIndent(model, "", "\t")
				if err != nil {
					t.Fatalf("marshal error: %v", err)
				}

				if strings.TrimSpace(string(importXML)) != strings.TrimSpace(string(expectedXMLData)) {
					t.Fatalf("XML mismatch\nWanted:\n%s\nGot:\n%s", string(expectedXMLData), string(importXML))
				}
			}
		})
	}
}
