package g2

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"

	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/ebuild_parser/*.txtar
var testdata embed.FS

// Normalize spacing in expected array output for tests since parser might keep some newlines
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func TestParserTxtar(t *testing.T) {
	files, err := testdata.ReadDir("testdata/ebuild_parser")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".txtar") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			content, err := testdata.ReadFile(filepath.Join("testdata/ebuild_parser", file.Name()))
			if err != nil {
				t.Fatalf("reading file: %v", err)
			}

			archive := txtar.Parse(content)
			var ebuildData []byte
			var expectedData []byte

			for _, f := range archive.Files {
				switch f.Name {
				case "ebuild.ebuild":
					ebuildData = f.Data
				case "output.json":
					expectedData = f.Data
				}
			}

			if len(ebuildData) == 0 {
				t.Fatalf("no ebuild.ebuild found in %s", file.Name())
			}

			parser := NewEbuildParser(context.Background(), bytes.NewReader(ebuildData))
			parsedEbuild, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// Normalize spaces in arrays/values to make json assertions easier in txtar
			for k, v := range parsedEbuild.Variables {
				parsedEbuild.Variables[k] = normalize(v)
			}

			var expected map[string]string
			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatalf("unmarshal expected JSON: %v", err)
			}

			if !reflect.DeepEqual(parsedEbuild.Variables, expected) {
				t.Errorf("Mismatch.\nGot:\n%v\nExpected:\n%v", parsedEbuild.Variables, expected)
			}
		})
	}
}

func TestParserCatchesNullBytes(t *testing.T) {
	ebuildData := "LICENSE=\"GPL-2\x00\"\n"
	parser := NewEbuildParser(context.Background(), strings.NewReader(ebuildData))
	_, err := parser.Parse()
	if err == nil {
		t.Fatal("Expected parse to fail on null bytes, but it succeeded")
	}

	if !strings.Contains(err.Error(), "corrupted file: null byte encountered") {
		t.Errorf("Expected null byte error, got: %v", err)
	}
}

func TestEbuildParserEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		ebuild    string
		wantError bool
		wantVars  map[string]string
	}{
		{
			name: "Subshell function body",
			ebuild: `
src_prepare() (
	default
)
`,
			wantError: false,
			wantVars:  map[string]string{},
		},
		{
			name: "Array with mismatched parens in quotes",
			ebuild: `
MY_ARRAY=(
	"foo)("
)
`,
			wantError: false,
			wantVars:  map[string]string{"MY_ARRAY": "(\n\t\"foo)(\"\n)"},
		},
		{
			name: "Array with mismatched parens in single quotes",
			ebuild: `
MY_ARRAY=(
	'foo)('
)
`,
			wantError: false,
			wantVars:  map[string]string{"MY_ARRAY": "(\n\t'foo)('\n)"},
		},
		{
			name: "Array with comment inside",
			ebuild: `
MY_ARRAY=(
	"foo" # some comment
	"bar"
)
`,
			wantError: false,
			wantVars:  map[string]string{"MY_ARRAY": "(\n\t\"foo\" \t\"bar\"\n)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewEbuildParser(context.Background(), strings.NewReader(tt.ebuild))
			parsedEbuild, err := parser.Parse()

			if (err != nil) != tt.wantError {
				t.Errorf("expected error: %v, got: %v", tt.wantError, err)
			}

			if !tt.wantError {
				for k, v := range tt.wantVars {
					if parsedEbuild.Variables[k] != v {
						t.Errorf("var %s expected %q, got %q", k, v, parsedEbuild.Variables[k])
					}
				}
			}
		})
	}
}
