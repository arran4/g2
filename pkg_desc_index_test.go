package g2

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParsePkgDescIndex(t *testing.T) {
	input := `app-admin/aerospike-amc-community 4.0.19-r2 5.0.0: Web UI based monitoring tool for Aerospike Community Edition Server
app-admin/amazon-ec2-init 20101127-r2: Init script to setup Amazon EC2 instance parameters
`

	reader := strings.NewReader(input)
	idx, err := ParsePkgDescIndex(reader)
	if err != nil {
		t.Fatalf("ParsePkgDescIndex failed: %v", err)
	}

	if len(idx.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(idx.Entries))
	}

	e1 := idx.Entries[0]
	if e1.Category != "app-admin" || e1.Package != "aerospike-amc-community" {
		t.Errorf("entry 1 path wrong: %s/%s", e1.Category, e1.Package)
	}
	if len(e1.Versions) != 2 || e1.Versions[0] != "4.0.19-r2" || e1.Versions[1] != "5.0.0" {
		t.Errorf("entry 1 versions wrong: %v", e1.Versions)
	}
	if e1.Description != "Web UI based monitoring tool for Aerospike Community Edition Server" {
		t.Errorf("entry 1 description wrong: %s", e1.Description)
	}

	e2 := idx.Entries[1]
	if e2.Category != "app-admin" || e2.Package != "amazon-ec2-init" {
		t.Errorf("entry 2 path wrong: %s/%s", e2.Category, e2.Package)
	}
	if len(e2.Versions) != 1 || e2.Versions[0] != "20101127-r2" {
		t.Errorf("entry 2 versions wrong: %v", e2.Versions)
	}
	if e2.Description != "Init script to setup Amazon EC2 instance parameters" {
		t.Errorf("entry 2 description wrong: %s", e2.Description)
	}
}

func TestSerializePkgDescIndex(t *testing.T) {
	idx := &PkgDescIndex{
		Entries: []PkgDescIndexEntry{
			{
				Category:    "app-admin",
				Package:     "aerospike-amc-community",
				Versions:    []string{"4.0.19-r2", "5.0.0"},
				Description: "Web UI based monitoring tool for Aerospike Community Edition Server",
			},
			{
				Category:    "app-admin",
				Package:     "amazon-ec2-init",
				Versions:    []string{"20101127-r2"},
				Description: "Init script to setup Amazon EC2 instance parameters",
			},
		},
	}

	var buf bytes.Buffer
	err := idx.Serialize(&buf)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	expected := `app-admin/aerospike-amc-community 4.0.19-r2 5.0.0: Web UI based monitoring tool for Aerospike Community Edition Server
app-admin/amazon-ec2-init 20101127-r2: Init script to setup Amazon EC2 instance parameters
`
	if buf.String() != expected {
		t.Errorf("Serialize mismatch.\nExpected:\n%s\nGot:\n%s", expected, buf.String())
	}
}

func TestParsePkgDescIndexFS(t *testing.T) {
	input := `app-admin/aerospike-amc-community 4.0.19-r2 5.0.0: Web UI based monitoring tool for Aerospike Community Edition Server
app-admin/amazon-ec2-init 20101127-r2: Init script to setup Amazon EC2 instance parameters
`
	fsys := fstest.MapFS{
		"pkg_desc_index": &fstest.MapFile{Data: []byte(input)},
	}

	idx, err := ParsePkgDescIndexFS(fsys, "pkg_desc_index")
	if err != nil {
		t.Fatalf("ParsePkgDescIndexFS failed: %v", err)
	}

	if len(idx.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(idx.Entries))
	}

	_, err = ParsePkgDescIndexFS(fsys, "non_existent")
	if err == nil {
		t.Errorf("expected error for non_existent file, got nil")
	}
}

func TestParsePkgDescIndexFile(t *testing.T) {
	// Test with a valid file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "pkg_desc_index")

	input := `app-admin/aerospike-amc-community 4.0.19-r2 5.0.0: Web UI based monitoring tool for Aerospike Community Edition Server
app-admin/amazon-ec2-init 20101127-r2: Init script to setup Amazon EC2 instance parameters
`
	err := os.WriteFile(filePath, []byte(input), 0644)
	if err != nil {
		t.Fatalf("Failed to write temporary file: %v", err)
	}

	idx, err := ParsePkgDescIndexFile(filePath)
	if err != nil {
		t.Fatalf("ParsePkgDescIndexFile failed: %v", err)
	}

	if len(idx.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(idx.Entries))
	}

	e1 := idx.Entries[0]
	if e1.Category != "app-admin" || e1.Package != "aerospike-amc-community" {
		t.Errorf("entry 1 path wrong: %s/%s", e1.Category, e1.Package)
	}
	if len(e1.Versions) != 2 || e1.Versions[0] != "4.0.19-r2" || e1.Versions[1] != "5.0.0" {
		t.Errorf("entry 1 versions wrong: %v", e1.Versions)
	}
	if e1.Description != "Web UI based monitoring tool for Aerospike Community Edition Server" {
		t.Errorf("entry 1 description wrong: %s", e1.Description)
	}

	// Test with non-existent file
	invalidFilePath := filepath.Join(tempDir, "non_existent_file")
	_, err = ParsePkgDescIndexFile(invalidFilePath)
	if err == nil {
		t.Errorf("ParsePkgDescIndexFile should fail for non-existent file")
	}
}
