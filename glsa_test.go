package g2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var sampleGLSA = []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE glsa SYSTEM "http://www.gentoo.org/dtd/glsa.dtd">
<glsa id="202312-01">
  <title>Sample GLSA</title>
  <synopsis>This is a sample GLSA.</synopsis>
  <product type="ebuild">Sample Product</product>
  <announced>2023-12-01</announced>
  <revised count="1">2023-12-01</revised>
  <bug>123456</bug>
  <access>remote</access>
  <affected>
    <package name="app-admin/sample" auto="yes" arch="*">
      <vulnerable range="lt" slot="0">1.2.3</vulnerable>
      <unaffected range="ge" slot="0">1.2.3</unaffected>
    </package>
  </affected>
  <background>
    Background information.
  </background>
  <description>
    Description of the vulnerability.
  </description>
  <impact type="normal">
    Impact description.
  </impact>
  <workaround>
    Workaround instructions.
  </workaround>
  <resolution>
    Resolution instructions.
  </resolution>
  <references>
    <uri link="https://example.com/cve">CVE-2023-12345</uri>
  </references>
  <metadata tag="requester" timestamp="2023-12-01T00:00:00Z">Alice</metadata>
</glsa>`)

func TestParseGLSABytes(t *testing.T) {
	glsa, err := ParseGLSABytes(sampleGLSA)
	if err != nil {
		t.Fatalf("ParseGLSABytes failed: %v", err)
	}

	if glsa.ID != "202312-01" {
		t.Errorf("expected ID '202312-01', got %q", glsa.ID)
	}
	if glsa.Title != "Sample GLSA" {
		t.Errorf("expected Title 'Sample GLSA', got %q", glsa.Title)
	}
	if glsa.Synopsis != "This is a sample GLSA." {
		t.Errorf("expected Synopsis 'This is a sample GLSA.', got %q", glsa.Synopsis)
	}
	if glsa.Product.Text != "Sample Product" || glsa.Product.Type != "ebuild" {
		t.Errorf("unexpected Product: %+v", glsa.Product)
	}
	if glsa.Announced != "2023-12-01" {
		t.Errorf("expected Announced '2023-12-01', got %q", glsa.Announced)
	}
	if glsa.Revised.Text != "2023-12-01" || glsa.Revised.Count != "1" {
		t.Errorf("unexpected Revised: %+v", glsa.Revised)
	}
	if len(glsa.Bugs) != 1 || glsa.Bugs[0] != "123456" {
		t.Errorf("unexpected Bugs: %v", glsa.Bugs)
	}
	if glsa.Access != "remote" {
		t.Errorf("expected Access 'remote', got %q", glsa.Access)
	}
	if len(glsa.Affected.Packages) != 1 {
		t.Fatalf("expected 1 affected package, got %d", len(glsa.Affected.Packages))
	}
	pkg := glsa.Affected.Packages[0]
	if pkg.Name != "app-admin/sample" || pkg.Auto != "yes" || pkg.Arch != "*" {
		t.Errorf("unexpected Package: %+v", pkg)
	}
	if len(pkg.Vulnerable) != 1 || pkg.Vulnerable[0].Range != "lt" || pkg.Vulnerable[0].Slot != "0" || pkg.Vulnerable[0].Text != "1.2.3" {
		t.Errorf("unexpected Vulnerable: %+v", pkg.Vulnerable)
	}
	if len(pkg.Unaffected) != 1 || pkg.Unaffected[0].Range != "ge" || pkg.Unaffected[0].Slot != "0" || pkg.Unaffected[0].Text != "1.2.3" {
		t.Errorf("unexpected Unaffected: %+v", pkg.Unaffected)
	}
	if glsa.Background == nil || strings.TrimSpace(glsa.Background.Text) != "Background information." {
		t.Errorf("unexpected Background: %+v", glsa.Background)
	}
	if strings.TrimSpace(glsa.Description.Text) != "Description of the vulnerability." {
		t.Errorf("unexpected Description: %+v", glsa.Description)
	}
	if strings.TrimSpace(glsa.Impact.Text) != "Impact description." || glsa.Impact.Type != "normal" {
		t.Errorf("unexpected Impact: %+v", glsa.Impact)
	}
	if strings.TrimSpace(glsa.Workaround.Text) != "Workaround instructions." {
		t.Errorf("unexpected Workaround: %+v", glsa.Workaround)
	}
	if strings.TrimSpace(glsa.Resolution.Text) != "Resolution instructions." {
		t.Errorf("unexpected Resolution: %+v", glsa.Resolution)
	}
	if len(glsa.References.URIs) != 1 || glsa.References.URIs[0].Text != "CVE-2023-12345" || glsa.References.URIs[0].Link != "https://example.com/cve" {
		t.Errorf("unexpected References: %+v", glsa.References)
	}
	if len(glsa.Metadata) != 1 || glsa.Metadata[0].Tag != "requester" || glsa.Metadata[0].Timestamp != "2023-12-01T00:00:00Z" || glsa.Metadata[0].Text != "Alice" {
		t.Errorf("unexpected Metadata: %+v", glsa.Metadata)
	}
}

func TestParseGLSA(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "sample-glsa.xml")
	err := os.WriteFile(path, sampleGLSA, 0644)
	if err != nil {
		t.Fatalf("failed to write sample glsa: %v", err)
	}

	glsa, err := ParseGLSA(path)
	if err != nil {
		t.Fatalf("ParseGLSA failed: %v", err)
	}
	if glsa.ID != "202312-01" {
		t.Errorf("expected ID '202312-01', got %q", glsa.ID)
	}
}

func TestParseGLSAErrors(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		_, err := ParseGLSA("non-existent-file.xml")
		if err == nil {
			t.Error("expected error for non-existent file, got nil")
		}
	})

	t.Run("invalid XML", func(t *testing.T) {
		_, err := ParseGLSABytes([]byte("<not-xml"))
		if err == nil {
			t.Error("expected error for invalid XML, got nil")
		}
	})

	t.Run("invalid root element", func(t *testing.T) {
		_, err := ParseGLSABytes([]byte("<other></other>"))
		if err == nil {
			t.Error("expected error for invalid root element, got nil")
		} else if !strings.Contains(err.Error(), "expected element type <glsa> but have <other>") {
			t.Errorf("expected error to mention 'expected element type <glsa> but have <other>', got %v", err)
		}
	})
}

func TestParseGLSAFromReader_Error(t *testing.T) {
	// A reader that returns an error
	errReader := &failingReader{}
	_, err := ParseGLSAFromReader(errReader)
	if err == nil {
		t.Error("expected error from failing reader, got nil")
	}
}

type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, os.ErrPermission
}
