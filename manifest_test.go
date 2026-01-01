package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseManifestEntry(t *testing.T) {
	line := "DIST example.tar.gz 12345 BLAKE2B 1234 SHA512 5678"
	entry, err := ParseManifestEntry(line)
	if err != nil {
		t.Fatalf("ParseManifestEntry failed: %v", err)
	}

	if entry.Type != "DIST" {
		t.Errorf("Expected Type 'DIST', got '%s'", entry.Type)
	}
	if entry.Filename != "example.tar.gz" {
		t.Errorf("Expected Filename 'example.tar.gz', got '%s'", entry.Filename)
	}
	if entry.Size != 12345 {
		t.Errorf("Expected Size 12345, got %d", entry.Size)
	}

	if len(entry.Hashes) != 2 {
		t.Fatalf("Expected 2 hashes, got %d", len(entry.Hashes))
	}

	if entry.GetHash("BLAKE2B") != "1234" {
		t.Errorf("Expected BLAKE2B 1234, got %s", entry.GetHash("BLAKE2B"))
	}
	if entry.GetHash("SHA512") != "5678" {
		t.Errorf("Expected SHA512 5678, got %s", entry.GetHash("SHA512"))
	}
}

func TestManifestString(t *testing.T) {
	entry := &ManifestEntry{
		Type:     "DIST",
		Filename: "example.tar.gz",
		Size:     12345,
	}
	entry.AddHash("BLAKE2B", "1234")
	entry.AddHash("SHA512", "5678")

	m := &Manifest{
		Entries: []*ManifestEntry{entry},
	}

	expected := "DIST example.tar.gz 12345 BLAKE2B 1234 SHA512 5678\n"
	if m.String() != expected {
		t.Errorf("Expected manifest string:\n%q\nGot:\n%q", expected, m.String())
	}
}

func TestParseManifestContent_Comments(t *testing.T) {
	content := `# This is a comment
DIST example.tar.gz 12345 BLAKE2B 1234

# Another comment
DIST other.tar.gz 67890 SHA512 5678
`
	m, err := ParseManifestContent(content)
	if err != nil {
		t.Fatalf("ParseManifestContent failed: %v", err)
	}

	if len(m.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(m.Entries))
	}
	if m.Entries[0].Filename != "example.tar.gz" {
		t.Errorf("Expected first entry to be example.tar.gz")
	}
	if m.Entries[1].Filename != "other.tar.gz" {
		t.Errorf("Expected second entry to be other.tar.gz")
	}
}

func TestUpsert(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	manifestPath := filepath.Join(tmpDir, "Manifest")

	// Create initial manifest
	entry1 := &ManifestEntry{Type: "DIST", Filename: "A", Size: 100}
	entry1.AddHash("SHA1", "123")
	if err := UpsertManifest(manifestPath, entry1); err != nil {
		t.Fatalf("First upsert failed: %v", err)
	}

	m, err := ParseManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 1 {
		t.Errorf("Expected 1 entry")
	}

	// Update existing
	entry1Updated := &ManifestEntry{Type: "DIST", Filename: "A", Size: 200}
	entry1Updated.AddHash("SHA1", "999")
	if err := UpsertManifest(manifestPath, entry1Updated); err != nil {
		t.Fatalf("Update upsert failed: %v", err)
	}

	m, err = ParseManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 1 {
		t.Errorf("Expected 1 entry")
	}
	if m.Entries[0].Size != 200 {
		t.Errorf("Expected updated size 200, got %d", m.Entries[0].Size)
	}

	// Add new (sorting check)
	entry2 := &ManifestEntry{Type: "DIST", Filename: "B", Size: 300}
	if err := UpsertManifest(manifestPath, entry2); err != nil {
		t.Fatalf("Second upsert failed: %v", err)
	}

	entry3 := &ManifestEntry{Type: "DIST", Filename: "0_First", Size: 50}
	if err := UpsertManifest(manifestPath, entry3); err != nil {
		t.Fatalf("Third upsert failed: %v", err)
	}

	m, err = ParseManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 3 {
		t.Errorf("Expected 3 entries")
	}

	// Check sorting
	if m.Entries[0].Filename != "0_First" {
		t.Errorf("Expected 0_First to be first, got %s", m.Entries[0].Filename)
	}
	if m.Entries[1].Filename != "A" {
		t.Errorf("Expected A to be second, got %s", m.Entries[1].Filename)
	}
}
