package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseUserConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.mask")

	content := `# User package mask configuration
# Some maintainer comment here

app-misc/foo::arrans-overlay
>=dev-libs/bar-2.0::arrans-overlay # inline comment

# Another section
sys-apps/baz::other-overlay
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	ucf, entries, err := ParseUserConfigFile(filePath)
	if err != nil {
		t.Fatalf("ParseUserConfigFile failed: %v", err)
	}

	if ucf == nil {
		t.Fatal("expected non-nil UserConfigFile")
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].AtomString != "app-misc/foo::arrans-overlay" || entries[0].LineNumber != 4 {
		t.Errorf("entry 0 mismatch: %+v", entries[0])
	}
	if entries[0].Atom.Category != "app-misc" || entries[0].Atom.Name != "foo" || entries[0].Atom.Repo != "arrans-overlay" {
		t.Errorf("entry 0 atom mismatch: %+v", entries[0].Atom)
	}

	if entries[1].AtomString != ">=dev-libs/bar-2.0::arrans-overlay" || entries[1].LineNumber != 5 {
		t.Errorf("entry 1 mismatch: %+v", entries[1])
	}

	if entries[2].AtomString != "sys-apps/baz::other-overlay" || entries[2].LineNumber != 8 {
		t.Errorf("entry 2 mismatch: %+v", entries[2])
	}
}

func TestReadUserConfigEntries(t *testing.T) {
	t.Run("non-existent path returns empty slice", func(t *testing.T) {
		entries, err := ReadUserConfigEntries("/path/does/not/exist/package.mask")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("directory with multiple fragments", func(t *testing.T) {
		tmpDir := t.TempDir()
		maskDir := filepath.Join(tmpDir, "package.mask")
		if err := os.MkdirAll(maskDir, 0755); err != nil {
			t.Fatalf("creating dir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(maskDir, "00-local"), []byte("app-misc/foo::arrans-overlay\n"), 0644); err != nil {
			t.Fatalf("writing file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(maskDir, "10-g2.conf"), []byte("dev-libs/bar::arrans-overlay\n"), 0644); err != nil {
			t.Fatalf("writing file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(maskDir, ".hidden"), []byte("hidden/ignored::repo\n"), 0644); err != nil {
			t.Fatalf("writing file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(maskDir, "backup~"), []byte("backup/ignored::repo\n"), 0644); err != nil {
			t.Fatalf("writing file: %v", err)
		}

		entries, err := ReadUserConfigEntries(maskDir)
		if err != nil {
			t.Fatalf("ReadUserConfigEntries failed: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].AtomString != "app-misc/foo::arrans-overlay" {
			t.Errorf("entry 0 atom = %q, want app-misc/foo::arrans-overlay", entries[0].AtomString)
		}
		if entries[1].AtomString != "dev-libs/bar::arrans-overlay" {
			t.Errorf("entry 1 atom = %q, want dev-libs/bar::arrans-overlay", entries[1].AtomString)
		}
	})
}

func TestAddUserConfigAtom(t *testing.T) {
	t.Run("flat file target", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "package.mask")

		initial := "# User mask\napp-misc/foo::repo\n"
		if err := os.WriteFile(filePath, []byte(initial), 0600); err != nil {
			t.Fatalf("writing initial file: %v", err)
		}

		added, target, err := AddUserConfigAtom(filePath, "dev-libs/bar::repo")
		if err != nil {
			t.Fatalf("AddUserConfigAtom failed: %v", err)
		}
		if !added || target != filePath {
			t.Fatalf("expected added=true, target=%s; got added=%v, target=%s", filePath, added, target)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		expected := "# User mask\napp-misc/foo::repo\ndev-libs/bar::repo\n"
		if string(content) != expected {
			t.Errorf("content = %q, want %q", string(content), expected)
		}

		// Verify permission preserved
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("mode = %v, want 0600", info.Mode().Perm())
		}

		// Duplicate add is idempotent
		added, _, err = AddUserConfigAtom(filePath, "dev-libs/bar::repo")
		if err != nil {
			t.Fatalf("duplicate add failed: %v", err)
		}
		if added {
			t.Error("expected added=false for duplicate atom")
		}
		contentAfter, _ := os.ReadFile(filePath)
		if string(contentAfter) != expected {
			t.Errorf("duplicate add modified file: %q", string(contentAfter))
		}
	})

	t.Run("directory target creates g2.conf", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, "package.mask")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(dirPath, "local"), []byte("app-misc/foo::repo\n"), 0644); err != nil {
			t.Fatalf("writing local: %v", err)
		}

		// Adding app-misc/foo::repo should be idempotent because it exists in local
		added, target, err := AddUserConfigAtom(dirPath, "app-misc/foo::repo")
		if err != nil {
			t.Fatalf("add failed: %v", err)
		}
		if added {
			t.Errorf("expected added=false when atom exists in another fragment, target=%s", target)
		}

		// Adding a new atom should write to g2.conf
		added, target, err = AddUserConfigAtom(dirPath, "dev-libs/bar::repo")
		if err != nil {
			t.Fatalf("add failed: %v", err)
		}
		if !added || target != filepath.Join(dirPath, "g2.conf") {
			t.Fatalf("expected added=true to g2.conf; got added=%v, target=%s", added, target)
		}

		g2Content, err := os.ReadFile(filepath.Join(dirPath, "g2.conf"))
		if err != nil {
			t.Fatalf("reading g2.conf: %v", err)
		}
		if string(g2Content) != "dev-libs/bar::repo\n" {
			t.Errorf("g2.conf content = %q", string(g2Content))
		}
	})

	t.Run("non-existent path creates directory with g2.conf", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, "package.mask")

		added, target, err := AddUserConfigAtom(dirPath, "app-misc/new::repo")
		if err != nil {
			t.Fatalf("add failed: %v", err)
		}
		if !added || target != filepath.Join(dirPath, "g2.conf") {
			t.Fatalf("expected added=true to g2.conf; got added=%v, target=%s", added, target)
		}

		content, err := os.ReadFile(filepath.Join(dirPath, "g2.conf"))
		if err != nil {
			t.Fatalf("reading g2.conf: %v", err)
		}
		if string(content) != "app-misc/new::repo\n" {
			t.Errorf("content = %q", string(content))
		}
	})
}

func TestRemoveUserConfigAtom(t *testing.T) {
	t.Run("preserves comments, blank lines, and other entries", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "package.mask")

		initial := `# Important header comment

# Section 1
app-misc/foo::repo # inline note

# Section 2
dev-libs/bar::repo

>=sys-apps/baz-1.0::repo
`
		if err := os.WriteFile(filePath, []byte(initial), 0640); err != nil {
			t.Fatalf("writing file: %v", err)
		}

		removed, err := RemoveUserConfigAtom(filePath, "dev-libs/bar::repo")
		if err != nil {
			t.Fatalf("RemoveUserConfigAtom failed: %v", err)
		}
		if removed != 1 {
			t.Fatalf("expected 1 removed, got %d", removed)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}

		expected := `# Important header comment

# Section 1
app-misc/foo::repo # inline note

# Section 2

>=sys-apps/baz-1.0::repo
`
		if string(content) != expected {
			t.Errorf("content =\n%s\nwant =\n%s", string(content), expected)
		}

		// Verify permissions preserved
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0640 {
			t.Errorf("mode = %v, want 0640", info.Mode().Perm())
		}

		// Repeated remove is idempotent
		removed2, err := RemoveUserConfigAtom(filePath, "dev-libs/bar::repo")
		if err != nil {
			t.Fatalf("second remove failed: %v", err)
		}
		if removed2 != 0 {
			t.Errorf("expected 0 removed on second call, got %d", removed2)
		}
	})

	t.Run("directory sweeps across all fragments", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, "package.mask")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		f1 := filepath.Join(dirPath, "00-local")
		f2 := filepath.Join(dirPath, "g2.conf")
		if err := os.WriteFile(f1, []byte("app-misc/foo::repo\ndev-libs/keep::repo\n"), 0644); err != nil {
			t.Fatalf("writing f1: %v", err)
		}
		if err := os.WriteFile(f2, []byte("app-misc/foo::repo\n"), 0644); err != nil {
			t.Fatalf("writing f2: %v", err)
		}

		removed, err := RemoveUserConfigAtom(dirPath, "app-misc/foo::repo")
		if err != nil {
			t.Fatalf("remove failed: %v", err)
		}
		if removed != 2 {
			t.Fatalf("expected 2 removed across fragments, got %d", removed)
		}

		c1, _ := os.ReadFile(f1)
		if string(c1) != "dev-libs/keep::repo\n" {
			t.Errorf("f1 content = %q, want %q", string(c1), "dev-libs/keep::repo\n")
		}

		c2, _ := os.ReadFile(f2)
		if string(c2) != "" {
			t.Errorf("f2 content = %q, want empty", string(c2))
		}
	})
}
