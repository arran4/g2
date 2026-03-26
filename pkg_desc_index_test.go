package g2

import (
	"bytes"
	"strings"
	"testing"
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
