package g2

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestShParseEbuild(t *testing.T) {
	testfiles, err := filepath.Glob("testdata/sh-parse/*.txtar")
	if err != nil {
		t.Fatalf("Failed to find testfiles: %v", err)
	}

	for _, file := range testfiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			archive, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatalf("Failed to parse txtar file %s: %v", file, err)
			}

			var ebuildContent []byte
			var expectedOutput []byte
			skipTest := false

			for _, f := range archive.Files {
				switch f.Name {
				case "ebuild.ebuild":
					ebuildContent = f.Data
				case "output.json":
					expectedOutput = f.Data
				case "skip":
					skipTest = true
				}
			}

			if skipTest {
				t.Skipf("Skipping test %s due to 'skip' file presence", file)
			}

			if len(ebuildContent) == 0 {
				t.Fatalf("No ebuild.ebuild file found in %s", file)
			}

			reader := bytes.NewReader(ebuildContent)
			data, err := ShParseEbuild(reader, "ebuild.ebuild")
			if err != nil {
				t.Fatalf("ShParseEbuild failed: %v", err)
			}

			jsonBytes, err := ShParseDataToJSON(data)
			if err != nil {
				t.Fatalf("ShParseDataToJSON failed: %v", err)
			}

			// We write to a temporary output so we can see what actually got generated if tests fail
			// or compare directly:
			expectedStr := strings.TrimSpace(string(expectedOutput))
			actualStr := strings.TrimSpace(string(jsonBytes))

			if expectedStr != actualStr {
				t.Errorf("JSON mismatch in %s:\nExpected:\n%s\nGot:\n%s", file, expectedStr, actualStr)
			}
		})
	}
}
