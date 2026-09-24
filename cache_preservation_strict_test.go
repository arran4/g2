package g2

import (
	"bytes"
	"io"
	"io/fs"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

// preservationSpy records attempted writes, including calls that are undone
// later or leave the bytes unchanged. This closes the gap left by comparing
// only the existing cache's before/after contents.
type preservationSpy struct {
	*MemCacheFS
	creates, removes, removeAlls []string
}

func (s *preservationSpy) Create(name string) (io.WriteCloser, error) {
	s.creates = append(s.creates, name)
	return s.MemCacheFS.Create(name)
}

func (s *preservationSpy) Remove(name string) error {
	s.removes = append(s.removes, name)
	return s.MemCacheFS.Remove(name)
}

func (s *preservationSpy) RemoveAll(name string) error {
	s.removeAlls = append(s.removeAlls, name)
	return s.MemCacheFS.RemoveAll(name)
}

func TestGenerateCacheDynamicFailureDoesNotAttemptWrites(t *testing.T) {
	fixture, err := cacheTestdataFS.ReadFile("testdata/cache/generate_dynamic_preservation.txtar")
	if err != nil {
		t.Fatal(err)
	}
	input, _ := SplitInputExpected(txtar.Parse(fixture))
	fsys := &preservationSpy{MemCacheFS: NewMemCacheFS(input)}

	// Clone the entire input, not merely the pre-existing cache entry, so a
	// failed generate cannot silently add, remove, or rewrite other files.
	before := make(map[string][]byte, len(fsys.Map))
	for name, file := range fsys.Map {
		before[name] = bytes.Clone(file.Data)
	}

	err = GenerateCacheFS(fsys, ".", nil, NewCachePolicy(CacheModeCI))
	if err == nil || !strings.Contains(err.Error(), "has unresolved BDEPEND") {
		t.Fatalf("expected unresolved BDEPEND generation failure; got %v", err)
	}
	if len(fsys.creates) != 0 || len(fsys.removes) != 0 || len(fsys.removeAlls) != 0 {
		t.Fatalf("generation attempted writes: creates=%v removes=%v removeAlls=%v", fsys.creates, fsys.removes, fsys.removeAlls)
	}
	if len(before) != len(fsys.Map) {
		t.Fatalf("file count changed during failed generation: before=%d after=%d", len(before), len(fsys.Map))
	}
	for name, original := range before {
		got, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("original file %s disappeared: %v", name, err)
		}
		if !bytes.Equal(original, got) {
			t.Fatalf("original file %s changed: before=%q after=%q", name, original, got)
		}
	}
}
