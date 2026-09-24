package md5cache

import (
	"crypto/md5"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/g2"
)

func regressionPackage() *g2.PackageData {
	return &g2.PackageData{
		Category: "app-misc", Name: "foo",
		Versions: []g2.VersionData{{
			Version: "1.0", PVR: "1.0",
			Ebuild: &g2.Ebuild{Path: "app-misc/foo/foo-1.0.ebuild"},
		}},
	}
}

func TestMD5CacheInvalidLintRule_MultilineReportsContinuation(t *testing.T) {
	const ebuild = "EAPI=8\n"
	cache := fmt.Sprintf("BDEPEND=\n    app-arch/unzip\n_md5_=%x\n", md5.Sum([]byte(ebuild)))
	fsys := fstest.MapFS{
		"metadata/md5-cache/app-misc/foo-1.0": &fstest.MapFile{Data: []byte(cache)},
		"app-misc/foo/foo-1.0.ebuild":         &fstest.MapFile{Data: []byte(ebuild)},
	}
	results := (&MD5CacheInvalidLintRule{}).lintFS(fsys, ".", regressionPackage(), nil, nil)
	const want = "Invalid format in md5-cache for foo-1.0:     app-arch/unzip"
	if len(results) != 1 || !strings.Contains(results[0].Message, want) {
		t.Fatalf("expected exactly the malformed continuation diagnostic %q; got %#v", want, results)
	}
}

func TestMD5CacheInvalidLintRule_InjectedEclassHashes(t *testing.T) {
	const ebuild = "EAPI=8\n"
	const eclass = "# shared eclass\n"
	eclassMD5 := fmt.Sprintf("%x", md5.Sum([]byte(eclass)))
	ebuildMD5 := fmt.Sprintf("%x", md5.Sum([]byte(ebuild)))
	const cachePath = "metadata/md5-cache/app-misc/foo-1.0"
	fsys := fstest.MapFS{
		cachePath:                     &fstest.MapFile{},
		"app-misc/foo/foo-1.0.ebuild": &fstest.MapFile{Data: []byte(ebuild)},
		"eclass/shared.eclass":        &fstest.MapFile{Data: []byte(eclass)},
	}

	for _, tc := range []struct {
		name, cachedHash string
		wantMismatch     bool
	}{
		{name: "matching", cachedHash: eclassMD5},
		{name: "mismatching", cachedHash: "incorrect", wantMismatch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys[cachePath].Data = []byte(fmt.Sprintf("EAPI=8\n_eclasses_=shared\t%s\n_md5_=%s\n", tc.cachedHash, ebuildMD5))
			calls := 0
			hashEclass := func(path string) (string, error) {
				calls++
				if path != "eclass/shared.eclass" {
					t.Fatalf("unexpected eclass path: %q", path)
				}
				return eclassMD5, nil
			}
			results := (&MD5CacheInvalidLintRule{}).lintFS(fsys, ".", regressionPackage(), nil, hashEclass)
			if calls != 1 {
				t.Fatalf("expected one eclass hash callback, got %d", calls)
			}
			if tc.wantMismatch {
				if len(results) != 1 || !strings.Contains(results[0].Message, "Incorrect eclass md5 for shared in md5-cache") {
					t.Fatalf("expected eclass hash mismatch only; got %#v", results)
				}
			} else if len(results) != 0 {
				t.Fatalf("matching eclass hash produced findings: %#v", results)
			}
		})
	}
}
