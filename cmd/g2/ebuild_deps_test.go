package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/deps/*.txtar
var depsTestFS embed.FS

type depsTestOptions struct {
	Args []string `json:"args"`
}

func TestEbuildDeps(t *testing.T) {
	entries, err := fs.Glob(depsTestFS, "testdata/deps/*.txtar")
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}

	for _, fixture := range entries {
		fixture := fixture
		t.Run(strings.TrimSuffix(path.Base(fixture), ".txtar"), func(t *testing.T) {
			raw, err := depsTestFS.ReadFile(fixture)
			if err != nil {
				t.Fatalf("read fixture %s: %v", fixture, err)
			}
			ar := txtar.Parse(raw)

			inputFS, expectedFS := SplitInputExpected(ar)

			// read options if any
			var opts depsTestOptions
			for _, f := range ar.Files {
				if f.Name == "options.json" {
					if err := json.Unmarshal(f.Data, &opts); err != nil {
						t.Fatalf("unmarshal options.json: %v", err)
					}
					break
				}
			}

			// write inputFS to a temp directory because our command logic relies on real files via flag.Args()
			tmpDir := t.TempDir()
			inputFiles, _ := WalkFiles(inputFS, ".")
			var ebuildFiles []string
			for _, name := range inputFiles {
				data, _ := fs.ReadFile(inputFS, name)
				fpath := filepath.Join(tmpDir, name)
				_ = os.MkdirAll(filepath.Dir(fpath), 0755)
				_ = os.WriteFile(fpath, data, 0644)
				if strings.HasSuffix(name, ".ebuild") {
					ebuildFiles = append(ebuildFiles, fpath)
				}
			}

			// run the command logic intercepting stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Because cmdEbuildDeps is a method on CmdEbuildArgConfig
			// We can initialize it
			cfg := &CmdEbuildArgConfig{
				MainArgConfig: &MainArgConfig{},
			}

			// append ebuilds to arguments
			args := append(opts.Args, ebuildFiles...)

			err = cfg.cmdEbuildDeps(args)

			// restore stdout and read result
			_ = w.Close()
			os.Stdout = oldStdout

			var outBuf bytes.Buffer
			_, _ = io.Copy(&outBuf, r)
			got := outBuf.String()

			if err != nil {
				t.Fatalf("cmdEbuildDeps error: %v", err)
			}

			// Compare against expected
			wantFiles, _ := WalkFiles(expectedFS, ".")
			if len(wantFiles) == 0 {
				t.Fatalf("no expected files found in fixture")
			}

			// We only expect one output file in these simple tests
			wantName := wantFiles[0]
			wantData, _ := fs.ReadFile(expectedFS, wantName)
			want := string(wantData)

			// Fix path differences
			// Our output prints the full temporary path when we pass temporary file path.
			// But the expected output has the base name.
			got = strings.ReplaceAll(got, tmpDir + string(filepath.Separator), "")

			if strings.TrimSpace(got) != strings.TrimSpace(want) {
				t.Errorf("mismatch\nwant:\n%s\n\ngot:\n%s\n", want, got)
			}
		})
	}
}
