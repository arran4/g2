package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManifestEntry(t *testing.T) {
	entry := NewManifestEntry("DIST", "foo.tar.gz", 12345, Hash{Type: "SHA512", Value: "abc"})
	if entry.Type != "DIST" {
		t.Errorf("Expected Type DIST, got %s", entry.Type)
	}
	if entry.Filename != "foo.tar.gz" {
		t.Errorf("Expected Filename foo.tar.gz, got %s", entry.Filename)
	}
	if entry.Size != 12345 {
		t.Errorf("Expected Size 12345, got %d", entry.Size)
	}
	if len(entry.Hashes) != 1 {
		t.Errorf("Expected 1 hash, got %d", len(entry.Hashes))
	}
	if entry.Hashes[0].Type != "SHA512" || entry.Hashes[0].Value != "abc" {
		t.Errorf("Hash mismatch")
	}
}

func TestParseManifestEntry_Errors(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"Empty", ""},
		{"Missing fields", "DIST filename"},
		{"Invalid size", "DIST filename invalid"},
		{"Odd number of hash parts", "DIST filename 123 SHA512"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifestEntry(tt.line)
			if err == nil {
				t.Errorf("Expected error for line: %q", tt.line)
			}
		})
	}
}

func TestParseManifestEntry_Valid(t *testing.T) {
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

func TestAddOrReplace(t *testing.T) {
	m := &Manifest{}
	e1 := NewManifestEntry("DIST", "file1", 100)
	m.AddOrReplace(e1)
	if len(m.Entries) != 1 || m.Entries[0].Size != 100 {
		t.Errorf("Add failed")
	}

	e2 := NewManifestEntry("DIST", "file1", 200)
	m.AddOrReplace(e2)
	if len(m.Entries) != 1 {
		t.Errorf("Duplicate added instead of replace")
	}
	if m.Entries[0].Size != 200 {
		t.Errorf("Replace failed, size not updated")
	}

	e3 := NewManifestEntry("DIST", "file2", 300)
	m.AddOrReplace(e3)
	if len(m.Entries) != 2 {
		t.Errorf("Second entry add failed")
	}
}

func TestRemove(t *testing.T) {
	m := &Manifest{}
	m.AddOrReplace(NewManifestEntry("DIST", "file1", 100))
	m.AddOrReplace(NewManifestEntry("DIST", "file2", 200))

	m.Remove("file3") // Non-existent
	if len(m.Entries) != 2 {
		t.Errorf("Remove non-existent affected entries")
	}

	m.Remove("file1")
	if len(m.Entries) != 1 {
		t.Errorf("Remove failed")
	}
	if m.Entries[0].Filename != "file2" {
		t.Errorf("Wrong entry removed")
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
	// We sort by Type then Filename.
	// AUX < DIST < EBUILD < MISC

	// Add a DIST entry that comes after A
	entry2 := &ManifestEntry{Type: "DIST", Filename: "B", Size: 300}
	if err := UpsertManifest(manifestPath, entry2); err != nil {
		t.Fatalf("Second upsert failed: %v", err)
	}

	// Add an AUX entry
	entry3 := &ManifestEntry{Type: "AUX", Filename: "Z", Size: 50}
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

	// Check sorting: AUX Z, DIST A, DIST B
	if m.Entries[0].Type != "AUX" || m.Entries[0].Filename != "Z" {
		t.Errorf("Expected AUX Z to be first, got %s %s", m.Entries[0].Type, m.Entries[0].Filename)
	}
	if m.Entries[1].Filename != "A" {
		t.Errorf("Expected A to be second, got %s", m.Entries[1].Filename)
	}
	if m.Entries[2].Filename != "B" {
		t.Errorf("Expected B to be third, got %s", m.Entries[2].Filename)
	}
}

func TestSort(t *testing.T) {
	m := &Manifest{
		Entries: []*ManifestEntry{
			{Type: "EBUILD", Filename: "b"},
			{Type: "DIST", Filename: "a"},
			{Type: "AUX", Filename: "c"},
			{Type: "DIST", Filename: "d"},
		},
	}
	m.Sort()

	if m.Entries[0].Type != "AUX" { t.Error("Sort failed: expected AUX first") }
	if m.Entries[1].Type != "DIST" || m.Entries[1].Filename != "a" { t.Error("Sort failed: expected DIST a second") }
	if m.Entries[2].Type != "DIST" || m.Entries[2].Filename != "d" { t.Error("Sort failed: expected DIST d third") }
	if m.Entries[3].Type != "EBUILD" { t.Error("Sort failed: expected EBUILD last") }
}
