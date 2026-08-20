package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestConfOverlay(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-conf-overlay-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	portageDir := filepath.Join(tempDir, "etc/portage")
	if err := os.MkdirAll(portageDir, 0755); err != nil {
		t.Fatal(err)
	}

	reposConf := filepath.Join(portageDir, "repos.conf")
	err = os.WriteFile(reposConf, []byte("[test-repo]\nlocation = /tmp/test-repo\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &MainArgConfig{
		Args: []string{"g2"},
	}

	t.Run("mask adds successfully", func(t *testing.T) {
		err := cfg.cmdConf([]string{"overlay", "test-repo", "mask", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/foo"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(portageDir, "package.mask", "g2.conf"))
		if err != nil {
			t.Fatalf("failed to read created file: %v", err)
		}

		if string(content) != "app-misc/foo::test-repo\n" {
			t.Errorf("unexpected content: %s", string(content))
		}
	})

	t.Run("unmask adds successfully", func(t *testing.T) {
		err := cfg.cmdConf([]string{"overlay", "test-repo", "unmask", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/foo"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(portageDir, "package.unmask", "g2.conf"))
		if err != nil {
			t.Fatalf("failed to read created file: %v", err)
		}

		if string(content) != "app-misc/foo::test-repo\n" {
			t.Errorf("unexpected content: %s", string(content))
		}
	})

	t.Run("mask-reset works", func(t *testing.T) {
		err := cfg.cmdConf([]string{"overlay", "test-repo", "mask-reset", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/foo"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		content, _ := os.ReadFile(filepath.Join(portageDir, "package.mask", "g2.conf"))
		if string(content) != "" {
			t.Errorf("expected empty mask, got %s", string(content))
		}
	})

	t.Run("unmask-reset works", func(t *testing.T) {
		err := cfg.cmdConf([]string{"overlay", "test-repo", "unmask-reset", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/foo"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		content, _ := os.ReadFile(filepath.Join(portageDir, "package.unmask", "g2.conf"))
		if string(content) != "" {
			t.Errorf("expected empty unmask, got %s", string(content))
		}
	})

	t.Run("preserves existing content", func(t *testing.T) {
		existingContent := "# some comment\napp-misc/bar\n"
		os.MkdirAll(filepath.Join(portageDir, "package.mask"), 0755)
		os.WriteFile(filepath.Join(portageDir, "package.mask", "g2.conf"), []byte(existingContent), 0644)

		err := cfg.cmdConf([]string{"overlay", "test-repo", "mask", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/foo"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(portageDir, "package.mask", "g2.conf"))
		if string(content) != existingContent+"app-misc/foo::test-repo\n" {
			t.Errorf("unexpected content: %s", string(content))
		}
	})

	t.Run("preserves existing content on reset", func(t *testing.T) {
		err := cfg.cmdConf([]string{"overlay", "test-repo", "mask-reset", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/foo"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(portageDir, "package.mask", "g2.conf"))
		if string(content) != "# some comment\napp-misc/bar\n" {
			t.Errorf("unexpected content: %s", string(content))
		}
	})

	t.Run("conflict rejects", func(t *testing.T) {
		err := cfg.cmdConf([]string{"overlay", "test-repo", "mask", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/foo::guru"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("accepts explicit ::repo", func(t *testing.T) {
		err := cfg.cmdConf([]string{"overlay", "test-repo", "mask", "--config-root", portageDir, "--repos-conf", reposConf, "app-misc/abc::test-repo"})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
	})

	t.Run("list outputs", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := cfg.cmdConf([]string{"overlay", "test-repo", "list", "--config-root", portageDir, "--repos-conf", reposConf})
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		buf.ReadFrom(r)

		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}

		expected := "Masks:\n  app-misc/abc::test-repo\nUnmasks:\n"
		if buf.String() != expected {
			fmt.Printf("expected %q, got %q", expected, buf.String())
		}
	})
}
