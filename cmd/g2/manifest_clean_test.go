package main

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/g2"
	"golang.org/x/tools/txtar"
)

//go:embed testdata/txtar/manifest_clean/*.txtar
var cleanManifestCases embed.FS

func ArchiveToMapFS(ar *txtar.Archive) fstest.MapFS {
	out := fstest.MapFS{}
	for _, f := range ar.Files {
		name := path.Clean(strings.TrimPrefix(f.Name, "/"))
		if name == "." {
			continue
		}
		out[name] = &fstest.MapFile{Data: append([]byte(nil), f.Data...)}
	}
	return out
}


func TestCleanManifest(t *testing.T) {
	var cases []string
	err := fs.WalkDir(cleanManifestCases, "testdata/txtar/manifest_clean", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".txtar") {
			return nil
		}
		cases = append(cases, p)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk testdata: %v", err)
	}
	sort.Strings(cases)

	for _, tc := range cases {
		tc := tc
		t.Run(strings.TrimSuffix(path.Base(tc), ".txtar"), func(t *testing.T) {
			raw, err := cleanManifestCases.ReadFile(tc)
			if err != nil {
				t.Fatalf("failed to read testcase %s: %v", tc, err)
			}
			ar := txtar.Parse(raw)
			inputFS, expectedFS := SplitInputExpected(ar)

			manifestData, err := inputFS.ReadFile("Manifest")
			if err != nil {
				t.Fatalf("could not read input Manifest: %v", err)
			}

			manifest, err := g2.ParseManifestContent(string(manifestData))
			if err != nil {
				t.Fatalf("could not parse input Manifest: %v", err)
			}

			err = CleanManifest(inputFS, ".", manifest)
			if err != nil {
				t.Fatalf("CleanManifest failed: %v", err)
			}

			gotManifestData := manifest.String()

			expectedManifestData, err := expectedFS.ReadFile("Manifest")
			if err != nil {
				t.Fatalf("could not read expected Manifest: %v", err)
			}

			if strings.TrimSpace(gotManifestData) != strings.TrimSpace(string(expectedManifestData)) {
				t.Errorf("manifest mismatch.\nExpected:\n%s\nGot:\n%s", string(expectedManifestData), gotManifestData)
			}
		})
	}
}
