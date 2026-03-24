package g2

import (
	"embed"
	"io/fs"
	"path"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/txtar/*.txtar
var metadataTxtarFS embed.FS

func TestMetadataFromTxtar(t *testing.T) {
	entries, err := fs.Glob(metadataTxtarFS, "testdata/txtar/*.txtar")
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}

	for _, fixture := range entries {
		fixture := fixture
		t.Run(strings.TrimSuffix(path.Base(fixture), ".txtar"), func(t *testing.T) {
			raw, err := metadataTxtarFS.ReadFile(fixture)
			if err != nil {
				t.Fatalf("read fixture %s: %v", fixture, err)
			}
			ar := txtar.Parse(raw)

			var input, expected []byte
			for _, f := range ar.Files {
				if f.Name == "input.xml" {
					input = f.Data
				} else if f.Name == "expected.xml" {
					expected = f.Data
				}
			}

			if input == nil || expected == nil {
				t.Fatalf("fixture %s must contain input.xml and expected.xml", fixture)
			}

			got, err := ParseMetadataBytes(input)
			if err != nil {
				t.Fatalf("ParseMetadataBytes() error: %v", err)
			}

			var gotStr string
			switch v := got.(type) {
			case *PkgMetadata:
				gotStr = v.String()
			case *CatMetadata:
				gotStr = v.String()
			default:
				t.Fatalf("Unknown metadata type: %T", got)
			}

			expectedStr := strings.TrimSpace(string(expected))
			if strings.TrimSpace(gotStr) != expectedStr {
				t.Errorf("Mismatch in parsing output.\nGot:\n%s\nWant:\n%s", gotStr, expectedStr)
			}

			// Circularity test
			got2, err := ParseMetadataBytes([]byte(gotStr))
			if err != nil {
				t.Fatalf("ParseMetadataBytes() on stringified data error: %v", err)
			}

			var got2Str string
			switch v := got2.(type) {
			case *PkgMetadata:
				got2Str = v.String()
			case *CatMetadata:
				got2Str = v.String()
			}

			if gotStr != got2Str {
				t.Errorf("Mismatch in circular test.\nGot1:\n%s\nGot2:\n%s", gotStr, got2Str)
			}
		})
	}
}
