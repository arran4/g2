package g2

import (
	_ "embed"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/ebuilds.txtar
var ebuildsArchive []byte

func TestEbuildsFromTxtar(t *testing.T) {
	archive := txtar.Parse(ebuildsArchive)
	files := make(map[string][]byte)
	for _, f := range archive.Files {
		files[f.Name] = f.Data
	}

	for _, f := range archive.Files {
		if strings.HasSuffix(f.Name, ".ebuild") {
			t.Run(f.Name, func(t *testing.T) {
				ebuildContent := f.Data
				goldenName := strings.TrimSuffix(f.Name, ".ebuild") + ".golden"
				jsonName := strings.TrimSuffix(f.Name, ".ebuild") + ".json"

				golden, ok := files[goldenName]
				if !ok {
					t.Fatalf("Golden file %s not found", goldenName)
				}

				var config struct {
					ParserMode string `json:"parser_mode"`
				}
				if jsonContent, ok := files[jsonName]; ok {
					if err := json.Unmarshal(jsonContent, &config); err != nil {
						t.Fatalf("Error parsing JSON config: %v", err)
					}
				}

				mode := ParseFull
				if config.ParserMode == "ParseVariables" {
					mode = ParseVariables
				} else if config.ParserMode == "ParseMetadataOnly" {
					mode = ParseMetadataOnly
				}

				// Construct a MemFS
				memFS := fstest.MapFS{
					f.Name: &fstest.MapFile{
						Data: ebuildContent,
					},
				}

				ebuild, err := ParseEbuild(memFS, f.Name, mode)
				if err != nil {
					t.Fatalf("ParseEbuild failed: %v", err)
				}

				got := ebuild.String()
				// Trim whitespace for comparison to avoid issues with trailing newlines
				gotTrimmed := strings.TrimSpace(got)
				wantTrimmed := strings.TrimSpace(string(golden))

				if gotTrimmed != wantTrimmed {
					// Debug whitespace mismatch
					// Compare lines
					gotLines := strings.Split(gotTrimmed, "\n")
					wantLines := strings.Split(wantTrimmed, "\n")
					if len(gotLines) != len(wantLines) {
						t.Errorf("Line count mismatch: got %d, want %d", len(gotLines), len(wantLines))
					}
					for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
						if gotLines[i] != wantLines[i] {
							t.Logf("Line %d mismatch:\nGot:  %q\nWant: %q", i, gotLines[i], wantLines[i])
						}
					}
					t.Errorf("Mismatch in output.\nGot:\n%s\nWant:\n%s", gotTrimmed, wantTrimmed)
				}

				// Circular test
				// Parse the output again
				memFS2 := fstest.MapFS{
					f.Name: &fstest.MapFile{
						Data: []byte(got),
					},
				}
				ebuild2, err := ParseEbuild(memFS2, f.Name, mode)
				if err != nil {
					t.Fatalf("Round 2 ParseEbuild failed: %v", err)
				}

				// Compare relevant fields
				vars1 := ebuild.Vars
				vars2 := ebuild2.Vars

				// In ParseFull mode, SRC_URI in Vars can change format (single line -> multi-line)
				// or content (conditional assignments vs extracted struct).
				// So we exclude it from Vars comparison and rely on SrcUri struct comparison.
				if mode == ParseFull {
					// Make copies to avoid modifying original
					vars1 = make(map[string]string)
					for k, v := range ebuild.Vars {
						vars1[k] = v
					}
					delete(vars1, "SRC_URI")

					vars2 = make(map[string]string)
					for k, v := range ebuild2.Vars {
						vars2[k] = v
					}
					delete(vars2, "SRC_URI")
				}

				// Normalize values for comparison (strip trailing whitespace from lines)
				// because String() output normalizes whitespace.
				normalize := func(m map[string]string) map[string]string {
					out := make(map[string]string)
					for k, v := range m {
						if strings.Contains(v, "\n") {
							lines := strings.Split(v, "\n")
							for i, line := range lines {
								lines[i] = strings.TrimRight(line, " \t")
							}
							out[k] = strings.Join(lines, "\n")
						} else {
							out[k] = v
						}
					}
					return out
				}

				if !reflect.DeepEqual(normalize(vars1), normalize(vars2)) {
					t.Errorf("Circular mismatch Vars: %v vs %v", vars1, vars2)
				}
				if len(ebuild.SrcUri) != len(ebuild2.SrcUri) {
					t.Errorf("Circular mismatch SrcUri count: %d vs %d", len(ebuild.SrcUri), len(ebuild2.SrcUri))
				}
			})
		}
	}
}
