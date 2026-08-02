package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMirrorsBytes(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mirrors SYSTEM "http://www.gentoo.org/dtd/mirrors.dtd">
<mirrors>
  <mirrorgroup region="North America" country="US" countryname="United States">
    <mirror city="New York" coordinates="40.7128,-74.0060" gentoo-bug="12345">
      <name>Test Mirror</name>
      <uri ipv4="Y" ipv6="N" partial="N" protocol="http">http://mirror.example.com/</uri>
      <uri ipv4="Y" ipv6="Y" partial="N" protocol="rsync">rsync://mirror.example.com/gentoo-portage</uri>
    </mirror>
  </mirrorgroup>
</mirrors>`)

	mirrors, err := ParseMirrorsBytes(xmlData)
	if err != nil {
		t.Fatalf("ParseMirrorsBytes failed: %v", err)
	}

	if len(mirrors.MirrorGroups) != 1 {
		t.Fatalf("Expected 1 mirror group, got %d", len(mirrors.MirrorGroups))
	}

	mg := mirrors.MirrorGroups[0]
	if mg.Region != "North America" || mg.Country != "US" || mg.CountryName != "United States" {
		t.Errorf("Unexpected mirrorgroup attributes: %+v", mg)
	}

	if len(mg.Mirrors) != 1 {
		t.Fatalf("Expected 1 mirror, got %d", len(mg.Mirrors))
	}

	m := mg.Mirrors[0]
	if m.City != "New York" || m.Coordinates != "40.7128,-74.0060" || m.GentooBug != "12345" || m.Name != "Test Mirror" {
		t.Errorf("Unexpected mirror attributes: %+v", m)
	}

	if len(m.URIs) != 2 {
		t.Fatalf("Expected 2 URIs, got %d", len(m.URIs))
	}

	u1 := m.URIs[0]
	if u1.Text != "http://mirror.example.com/" || u1.IPv4 != "Y" || u1.IPv6 != "N" || u1.Partial != "N" || u1.Protocol != "http" {
		t.Errorf("Unexpected URI 1: %+v", u1)
	}

	u2 := m.URIs[1]
	if u2.Text != "rsync://mirror.example.com/gentoo-portage" || u2.IPv4 != "Y" || u2.IPv6 != "Y" || u2.Partial != "N" || u2.Protocol != "rsync" {
		t.Errorf("Unexpected URI 2: %+v", u2)
	}
}

func TestParseMirrorsBytes_InvalidRoot(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0"?><notmirrors></notmirrors>`)
	_, err := ParseMirrorsBytes(xmlData)
	if err == nil {
		t.Error("Expected error for invalid XML root, got nil")
	}
}

func TestParseMirrorsBytes_InvalidXML(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0"?><mirrors><mirrorgroup>`)
	_, err := ParseMirrorsBytes(xmlData)
	if err == nil {
		t.Error("Expected error for malformed XML, got nil")
	}
}

func TestParseMirrors(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<mirrors>
  <mirrorgroup region="Test" country="TE">
    <mirror>
      <name>File Test Mirror</name>
      <uri protocol="http">http://file.example.com/</uri>
    </mirror>
  </mirrorgroup>
</mirrors>`)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mirrors.xml")
	if err := os.WriteFile(filePath, xmlData, 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	mirrors, err := ParseMirrors(filePath)
	if err != nil {
		t.Fatalf("ParseMirrors failed: %v", err)
	}

	if len(mirrors.MirrorGroups) != 1 || mirrors.MirrorGroups[0].Mirrors[0].Name != "File Test Mirror" {
		t.Errorf("Unexpected parsed mirrors from file")
	}
}

func TestParseMirrors_MissingFile(t *testing.T) {
	_, err := ParseMirrors("does_not_exist.xml")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}
