package g2

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/metadata_extra/*.txtar
var metadataExtraFS embed.FS

func TestMetadataExtraTxtar(t *testing.T) {
	entries, err := fs.Glob(metadataExtraFS, "testdata/metadata_extra/*.txtar")
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}

	for _, fixture := range entries {
		fixture := fixture
		t.Run(strings.TrimSuffix(path.Base(fixture), ".txtar"), func(t *testing.T) {
			raw, err := metadataExtraFS.ReadFile(fixture)
			if err != nil {
				t.Fatalf("read fixture %s: %v", fixture, err)
			}
			ar := txtar.Parse(raw)

			var inputXML []byte
			var expectedJSON []byte

			for _, f := range ar.Files {
				switch f.Name {
				case "input.xml":
					inputXML = f.Data
				case "expected.json":
					expectedJSON = f.Data
				}
			}

			if inputXML == nil || expectedJSON == nil {
				t.Fatalf("fixture %s missing input.xml or expected.json", fixture)
			}

			if strings.HasPrefix(fixture, "testdata/metadata_extra/glsa_") {
				glsa, err := ParseGLSABytes(inputXML)
				if err != nil {
					t.Fatalf("ParseGLSABytes failed: %v", err)
				}
				// create a map representation for comparison to ignore XMLName
				actualMap := make(map[string]interface{})
				b, _ := json.Marshal(glsa)
				if err := json.Unmarshal(b, &actualMap); err != nil {
					t.Fatalf("Unmarshal failed: %v", err)
				}
				delete(actualMap, "XMLName")
				compareJSON(t, expectedJSON, actualMap)
			} else if strings.HasPrefix(fixture, "testdata/metadata_extra/mirrors_") {
				mirrors, err := ParseMirrorsBytes(inputXML)
				if err != nil {
					t.Fatalf("ParseMirrorsBytes failed: %v", err)
				}
				actualMap := make(map[string]interface{})
				b, _ := json.Marshal(mirrors)
				if err := json.Unmarshal(b, &actualMap); err != nil {
					t.Fatalf("Unmarshal failed: %v", err)
				}
				delete(actualMap, "XMLName")
				compareJSON(t, expectedJSON, actualMap)
			} else if strings.HasPrefix(fixture, "testdata/metadata_extra/projects_") {
				projects, err := ParseProjectsBytes(inputXML)
				if err != nil {
					t.Fatalf("ParseProjectsBytes failed: %v", err)
				}
				actualMap := make(map[string]interface{})
				b, _ := json.Marshal(projects)
				if err := json.Unmarshal(b, &actualMap); err != nil {
					t.Fatalf("Unmarshal failed: %v", err)
				}
				delete(actualMap, "XMLName")
				compareJSON(t, expectedJSON, actualMap)
			} else if strings.HasPrefix(fixture, "testdata/metadata_extra/repositories_") {
				repositories, err := ParseRepositoriesBytes(inputXML)
				if err != nil {
					t.Fatalf("ParseRepositoriesBytes failed: %v", err)
				}
				actualMap := make(map[string]interface{})
				b, _ := json.Marshal(repositories)
				if err := json.Unmarshal(b, &actualMap); err != nil {
					t.Fatalf("Unmarshal failed: %v", err)
				}
				delete(actualMap, "XMLName")
				compareJSON(t, expectedJSON, actualMap)
			} else if strings.HasPrefix(fixture, "testdata/metadata_extra/userinfo_") {
				userinfo, err := ParseUserInfoBytes(inputXML)
				if err != nil {
					t.Fatalf("ParseUserInfoBytes failed: %v", err)
				}
				actualMap := make(map[string]interface{})
				b, _ := json.Marshal(userinfo)
				if err := json.Unmarshal(b, &actualMap); err != nil {
					t.Fatalf("Unmarshal failed: %v", err)
				}
				delete(actualMap, "XMLName")
				compareJSON(t, expectedJSON, actualMap)
			} else {
				t.Fatalf("unknown fixture prefix: %s", fixture)
			}
		})
	}
}

func compareJSON(t *testing.T, expectedJSON []byte, actual interface{}) {
	t.Helper()

	var expected interface{}

	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected json: %v", err)
	}

	if !reflect.DeepEqual(expected, actual) {
		expectedStr, _ := json.MarshalIndent(expected, "", "  ")
		actualStr, _ := json.MarshalIndent(actual, "", "  ")
		t.Errorf("mismatch.\nExpected:\n%s\nActual:\n%s\n", expectedStr, actualStr)
	}
}
