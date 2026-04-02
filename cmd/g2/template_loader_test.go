package main

import (
	"testing"
)

func TestGetSiteTemplates(t *testing.T) {
	// Need to clear the sync.Once for testing? No, since it's a singleton,
	// just calling it once and verifying is fine.
	tmpl, err := GetSiteTemplates()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tmpl == nil {
		t.Fatal("Expected templates to be loaded, got nil")
	}

	// Make sure layout_header.html and layout_footer.html are loaded
	if tmpl.Lookup("layout_header.html") == nil {
		t.Error("Expected layout_header.html to be loaded")
	}
	if tmpl.Lookup("layout_footer.html") == nil {
		t.Error("Expected layout_footer.html to be loaded")
	}

	// Make sure dashboard.html is loaded
	if tmpl.Lookup("dashboard.html") == nil {
		t.Error("Expected dashboard.html to be loaded")
	}

	// Test caching/singleton behavior
	tmpl2, err2 := GetSiteTemplates()
	if err2 != nil {
		t.Fatalf("Expected no error on second call, got %v", err2)
	}
	if tmpl != tmpl2 {
		t.Errorf("Expected singleton template registry, got different pointers")
	}
}
