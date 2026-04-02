package main

import (
	"github.com/arran4/g2"

	"os"
	"testing"
	"time"
)

func TestGenerateUsesPages(t *testing.T) {
	siteData, err := parseRepo(os.DirFS("../../testdata/test_overlay"), ".", "Test Overlay", false, nil)
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}

	outDir := t.TempDir()

	err = generateSite(outDir, []*g2.SiteData{siteData}, 90*24*time.Hour, "3 months", GenerationInfo{})
	if err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}

		if _, err := os.Stat(outDir + "/uses/index.html"); os.IsNotExist(err) {
		t.Errorf("Expected /uses/index.html to be generated, got error: %v", err)
	}
}
