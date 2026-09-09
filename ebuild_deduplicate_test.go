package g2

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/txtar/ebuild_deduplicate/*.txtar
var deduplicateEbuildsCases embed.FS

func writeFS(t *testing.T, targetDir string, srcFS fstest.MapFS) {
	t.Helper()
	for name, f := range srcFS {
		targetPath := filepath.Join(targetDir, name)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, f.Data, 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", targetPath, err)
		}
	}
}

func readDirFS(t *testing.T, dir string) fstest.MapFS {
	t.Helper()
	res := fstest.MapFS{}
	err := filepath.Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		res[filepath.ToSlash(rel)] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk %s: %v", dir, err)
	}
	return res
}

func TestDeduplicateEbuildsTxtar(t *testing.T) {
	var cases []string
	err := fs.WalkDir(deduplicateEbuildsCases, "testdata/txtar/ebuild_deduplicate", func(p string, d fs.DirEntry, err error) error {
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
			raw, err := deduplicateEbuildsCases.ReadFile(tc)
			if err != nil {
				t.Fatalf("failed to read testcase %s: %v", tc, err)
			}
			ar := txtar.Parse(raw)
			inputFS, expectedFS := SplitInputExpected(ar)

			tmpDir := t.TempDir()
			writeFS(t, tmpDir, inputFS)

			_, err = DeduplicateEbuilds([]string{tmpDir})
			if err != nil {
				t.Fatalf("DeduplicateEbuilds failed: %v", err)
			}

			gotFS := readDirFS(t, tmpDir)

			// verify gotFS has same files as expectedFS
			for name, f := range expectedFS {
				got, ok := gotFS[name]
				if !ok {
					t.Errorf("Expected file %s not found in output", name)
					continue
				}
				if string(got.Data) != string(f.Data) {
					t.Errorf("File %s content mismatch\nExpected:\n%s\nGot:\n%s", name, string(f.Data), string(got.Data))
				}
			}

			for name := range gotFS {
				if _, ok := expectedFS[name]; !ok {
					t.Errorf("Unexpected file %s found in output", name)
				}
			}
		})
	}
}

func TestDeduplicateEbuildsRevisionComparison(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "app-test", "dummy")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Two identical content ebuilds where one has a higher revision (-r1).
	// Deduplication must retain the higher revision (dummy-1.2-r1.ebuild) and remove dummy-1.2.ebuild.
	content := []byte("EAPI=8\nSLOT=\"0\"\n# Generated via: tool\nDESCRIPTION=\"Test\"\n")
	if err := os.WriteFile(filepath.Join(pkgDir, "dummy-1.2.ebuild"), content, 0644); err != nil {
		t.Fatalf("write dummy-1.2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "dummy-1.2-r1.ebuild"), content, 0644); err != nil {
		t.Fatalf("write dummy-1.2-r1: %v", err)
	}

	removed, err := DeduplicateEbuilds([]string{tmpDir})
	if err != nil {
		t.Fatalf("DeduplicateEbuilds failed: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed ebuild, got %d", len(removed))
	}
	if filepath.Base(removed[0]) != "dummy-1.2.ebuild" {
		t.Errorf("expected dummy-1.2.ebuild to be removed, got: %s", removed[0])
	}
	if _, err := os.Stat(filepath.Join(pkgDir, "dummy-1.2-r1.ebuild")); err != nil {
		t.Errorf("expected dummy-1.2-r1.ebuild to be retained: %v", err)
	}
}

func TestDeduplicateEbuildsCacheRemoval(t *testing.T) {
	dir := t.TempDir()

	// Setup repo
	cat := "sys-apps"
	pkg := "test"
	if err := os.MkdirAll(filepath.Join(dir, cat, pkg), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	ebuildPaths := []string{
		filepath.Join(dir, cat, pkg, "test-1.0.ebuild"),
		filepath.Join(dir, cat, pkg, "test-1.0-r1.ebuild"), // Revision should be kept, 1.0 removed
		filepath.Join(dir, cat, pkg, "test-2.0.ebuild"),    // Kept (different version)
	}

	if err := os.WriteFile(ebuildPaths[0], []byte(`DESCRIPTION="test1"
`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(ebuildPaths[1], []byte(`DESCRIPTION="test1"
`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(ebuildPaths[2], []byte(`DESCRIPTION="test2"
`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Create cache entries
	cacheDir := filepath.Join(dir, "metadata", "md5-cache", cat)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	cachePaths := []string{
		filepath.Join(cacheDir, "test-1.0"),
		filepath.Join(cacheDir, "test-1.0-r1"),
		filepath.Join(cacheDir, "test-2.0"),
	}
	for _, p := range cachePaths {
		if err := os.WriteFile(p, []byte(`_md5_=123
`), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	// Deduplicate absolute paths
	removed, err := DeduplicateEbuilds([]string{filepath.Join(dir, cat, pkg)})
	if err != nil {
		t.Fatalf("DeduplicateEbuilds failed: %v", err)
	}

	if len(removed) != 2 {
		t.Fatalf("Expected 2 files to be removed, got %v", removed)
	}

	// Check cache
	if _, err := os.Stat(cachePaths[0]); !os.IsNotExist(err) {
		t.Errorf("Cache for 1.0 should have been removed")
	}
	if _, err := os.Stat(cachePaths[1]); !os.IsNotExist(err) {
		t.Errorf("Cache for 1.0-r1 should have been removed")
	}
	if _, err := os.Stat(cachePaths[2]); os.IsNotExist(err) {
		t.Errorf("Cache for 2.0 should have been kept")
	}
}

func TestDeduplicateEbuildsCacheRemovalRelative(t *testing.T) {
	dir := t.TempDir()

	// Set up repo structure and chdir to it to test relative paths properly
	cat := "sys-apps"
	pkg := "test"
	if err := os.MkdirAll(filepath.Join(dir, cat, pkg), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	ebuildPaths := []string{
		filepath.Join(dir, cat, pkg, "test-1.0.ebuild"),
		filepath.Join(dir, cat, pkg, "test-1.0-r1.ebuild"),
		filepath.Join(dir, cat, pkg, "test-2.0.ebuild"),
	}

	if err := os.WriteFile(ebuildPaths[0], []byte(`DESCRIPTION="test1"
`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(ebuildPaths[1], []byte(`DESCRIPTION="test1"
`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(ebuildPaths[2], []byte(`DESCRIPTION="test2"
`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cacheDir := filepath.Join(dir, "metadata", "md5-cache", cat)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	cachePaths := []string{
		filepath.Join(cacheDir, "test-1.0"),
		filepath.Join(cacheDir, "test-1.0-r1"),
		filepath.Join(cacheDir, "test-2.0"),
	}
	for _, p := range cachePaths {
		if err := os.WriteFile(p, []byte(`_md5_=123
`), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	// Chdir to use relative paths
	cwd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(cwd)

	// Deduplicate relative paths
	removed, err := DeduplicateEbuilds([]string{filepath.Join(cat, pkg)})
	if err != nil {
		t.Fatalf("DeduplicateEbuilds failed: %v", err)
	}

	if len(removed) != 2 {
		t.Fatalf("Expected 2 files to be removed, got %v", removed)
	}

	// Check cache via original absolute paths
	if _, err := os.Stat(cachePaths[0]); !os.IsNotExist(err) {
		t.Errorf("Cache for 1.0 should have been removed")
	}
	if _, err := os.Stat(cachePaths[1]); !os.IsNotExist(err) {
		t.Errorf("Cache for 1.0-r1 should have been removed")
	}
	if _, err := os.Stat(cachePaths[2]); os.IsNotExist(err) {
		t.Errorf("Cache for 2.0 should have been kept")
	}
}
