package g2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertManifest(t *testing.T) {
	t.Run("Create new manifest", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "Manifest")

		entry := NewManifestEntry("DIST", "foo.tar.gz", 12345, Hash{Type: "SHA512", Value: "abc"})
		err := UpsertManifest(manifestPath, entry)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		content, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("Failed to read created manifest: %v", err)
		}

		expected := "DIST foo.tar.gz 12345 SHA512 abc\n"
		if string(content) != expected {
			t.Errorf("Expected %q, got %q", expected, string(content))
		}
	})

	t.Run("Add new entry and sort", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "Manifest")

		// Initial content with one entry
		initialContent := "DIST zzz.tar.gz 100 SHA512 def\n"
		err := os.WriteFile(manifestPath, []byte(initialContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write initial manifest: %v", err)
		}

		entry := NewManifestEntry("DIST", "foo.tar.gz", 12345, Hash{Type: "SHA512", Value: "abc"})
		err = UpsertManifest(manifestPath, entry)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		content, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("Failed to read updated manifest: %v", err)
		}

		// foo.tar.gz should come before zzz.tar.gz because Sort() sorts by Type then Filename
		expected := "DIST foo.tar.gz 12345 SHA512 abc\nDIST zzz.tar.gz 100 SHA512 def\n"
		if string(content) != expected {
			t.Errorf("Expected %q, got %q", expected, string(content))
		}
	})

	t.Run("Replace existing entry", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "Manifest")

		// Initial content with an entry
		initialContent := "DIST foo.tar.gz 100 SHA512 def\n"
		err := os.WriteFile(manifestPath, []byte(initialContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write initial manifest: %v", err)
		}

		// New entry with same type and filename but different size and hash
		entry := NewManifestEntry("DIST", "foo.tar.gz", 12345, Hash{Type: "SHA512", Value: "abc"})
		err = UpsertManifest(manifestPath, entry)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		content, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("Failed to read updated manifest: %v", err)
		}

		expected := "DIST foo.tar.gz 12345 SHA512 abc\n"
		if string(content) != expected {
			t.Errorf("Expected %q, got %q", expected, string(content))
		}
	})

	t.Run("Invalid existing manifest", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "Manifest")

		// Initial content that is invalid (e.g., missing size field)
		initialContent := "DIST foo.tar.gz\n"
		err := os.WriteFile(manifestPath, []byte(initialContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write initial manifest: %v", err)
		}

		entry := NewManifestEntry("DIST", "bar.tar.gz", 12345, Hash{Type: "SHA512", Value: "abc"})
		err = UpsertManifest(manifestPath, entry)
		if err == nil {
			t.Fatalf("Expected error for invalid manifest, got nil")
		}

		if !strings.Contains(err.Error(), "not enough fields") {
			t.Errorf("Expected 'not enough fields' error, got %v", err)
		}
	})
}
