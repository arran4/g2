package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSearchEngine_Txtar(t *testing.T) {
	manifest1 := SearchManifest{
		DataFiles: []string{"data1.json"},
	}
	manifest2 := SearchManifest{
		DataFiles: []string{"data2.json"},
	}

	m1Bytes, _ := json.Marshal(manifest1)
	m2Bytes, _ := json.Marshal(manifest2)

	// In `SearchEngine.Search`, if the query isn't matching fields like "category:pkg", it falls back to `matchSequence`.
	// `matchSequence` checks `doc.SearchText` (and also `matchTerm` checks it).
	// Therefore, we must populate `SearchText` for basic queries to work.
	doc1 := []SearchDocument{{FullName: "cat1/pkg1", Category: "cat1", Package: "pkg1", Description: "desc1", SearchText: strings.ToLower("cat1/pkg1 desc1")}}
	doc2 := []SearchDocument{{FullName: "cat2/pkg2", Category: "cat2", Package: "pkg2", Description: "desc2", SearchText: strings.ToLower("cat2/pkg2 desc2")}}

	d1Bytes, _ := json.Marshal(doc1)
	d2Bytes, _ := json.Marshal(doc2)

	txtarContent := "-- manifest.json --\n" + string(m1Bytes) +
		"\n-- data1.json --\n" + string(d1Bytes) +
		"\n-- some/other/dir/manifest.json --\n" + string(m2Bytes) +
		"\n-- some/other/dir/data2.json --\n" + string(d2Bytes) + "\n"

	tmpFile := filepath.Join(t.TempDir(), "test.txtar")
	if err := os.WriteFile(tmpFile, []byte(txtarContent), 0644); err != nil {
		t.Fatalf("failed to write txtar: %v", err)
	}

	engine := NewSearchEngine()
	if err := LoadSearchEngine(tmpFile, engine); err != nil {
		t.Fatalf("LoadSearchEngine failed: %v", err)
	}

	res := engine.Search("pkg1")
	if len(res) != 1 || res[0].FullName != "cat1/pkg1" {
		t.Errorf("Expected to find pkg1 loaded from first manifest, got %v", res)
	}

	res = engine.Search("pkg2")
	if len(res) != 0 {
		t.Errorf("Expected not to find pkg2 since second manifest should be ignored, got %v", res)
	}
}

func TestLoadSearchEngine_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	manifest := SearchManifest{
		DataFiles: []string{"data1.json"},
	}
	mBytes, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dataDir, "manifest.json"), mBytes, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc1 := []SearchDocument{{FullName: "cat1/pkg1", Category: "cat1", Package: "pkg1", Description: "desc1", SearchText: "cat1/pkg1 desc1"}}
	d1Bytes, _ := json.Marshal(doc1)
	if err := os.WriteFile(filepath.Join(dataDir, "data1.json"), d1Bytes, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	engine := NewSearchEngine()
	if err := LoadSearchEngine(tmpDir, engine); err != nil {
		t.Fatalf("LoadSearchEngine failed: %v", err)
	}

	res := engine.Search("pkg1")
	if len(res) != 1 || res[0].FullName != "cat1/pkg1" {
		t.Errorf("Expected to find pkg1, got %v", res)
	}
}
