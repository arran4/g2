package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureOutput captures stdout printed during fn.
func captureOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		outChan <- buf.String()
	}()

	runErr := fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}
	os.Stdout = origStdout
	output := <-outChan

	return output, runErr
}

// setupTestPortage creates an isolated repos.conf and portage config root.
func setupTestPortage(t *testing.T) (configRoot, reposConf string) {
	t.Helper()
	tmpDir := t.TempDir()
	configRoot = filepath.Join(tmpDir, "portage")
	if err := os.MkdirAll(configRoot, 0755); err != nil {
		t.Fatalf("creating config root: %v", err)
	}

	reposConf = filepath.Join(configRoot, "repos.conf")
	reposConfContent := `[DEFAULT]
main-repo = gentoo

[arrans-overlay]
location = ` + filepath.Join(tmpDir, "repos", "arrans-overlay") + `

[guru]
location = ` + filepath.Join(tmpDir, "repos", "guru") + `

# [disabled-repo]
# location = /var/empty

[flag-disabled]
location = /var/empty
disabled = true
`
	if err := os.WriteFile(reposConf, []byte(reposConfContent), 0644); err != nil {
		t.Fatalf("writing repos.conf: %v", err)
	}

	return configRoot, reposConf
}

func TestConfOverlayCommands(t *testing.T) {
	cfg := &MainArgConfig{}

	// 1. package mask adds ::repo
	t.Run("package mask adds ::repo", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask failed: %v", err)
		}

		entries, err := os.ReadFile(filepath.Join(configRoot, "package.mask", "g2.conf"))
		if err != nil {
			t.Fatalf("reading package.mask: %v", err)
		}
		if !strings.Contains(string(entries), "app-misc/foo::arrans-overlay\n") {
			t.Errorf("expected app-misc/foo::arrans-overlay in %q", string(entries))
		}
	})

	// 2. package unmask adds ::repo
	t.Run("package unmask adds ::repo", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("unmask failed: %v", err)
		}

		entries, err := os.ReadFile(filepath.Join(configRoot, "package.unmask", "g2.conf"))
		if err != nil {
			t.Fatalf("reading package.unmask: %v", err)
		}
		if !strings.Contains(string(entries), "app-misc/foo::arrans-overlay\n") {
			t.Errorf("expected app-misc/foo::arrans-overlay in %q", string(entries))
		}
	})

	// 3. matching explicit ::repo is accepted
	t.Run("matching explicit ::repo is accepted", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo::arrans-overlay", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask failed: %v", err)
		}

		entries, err := os.ReadFile(filepath.Join(configRoot, "package.mask", "g2.conf"))
		if err != nil {
			t.Fatalf("reading package.mask: %v", err)
		}
		if !strings.Contains(string(entries), "app-misc/foo::arrans-overlay\n") {
			t.Errorf("expected app-misc/foo::arrans-overlay in %q", string(entries))
		}
	})

	// 4. conflicting ::repo is rejected for mask
	t.Run("conflicting ::repo is rejected for mask", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo::guru", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when mask has conflicting repo qualifier")
		}
		if !strings.Contains(err.Error(), "does not match selected repository") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// 5. conflicting ::repo is rejected for unmask
	t.Run("conflicting ::repo is rejected for unmask", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "app-misc/foo::guru", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when unmask has conflicting repo qualifier")
		}
		if !strings.Contains(err.Error(), "does not match selected repository") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// 6. conflicting ::repo is rejected for mask-reset
	t.Run("conflicting ::repo is rejected for mask-reset", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", "app-misc/foo::guru", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when mask-reset has conflicting repo qualifier")
		}
		if !strings.Contains(err.Error(), "does not match selected repository") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// 7. conflicting ::repo is rejected for unmask-reset
	t.Run("conflicting ::repo is rejected for unmask-reset", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask-reset", "app-misc/foo::guru", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when unmask-reset has conflicting repo qualifier")
		}
		if !strings.Contains(err.Error(), "does not match selected repository") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// 8. flat-file package.mask
	t.Run("flat-file package.mask", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskFile := filepath.Join(configRoot, "package.mask")
		if err := os.WriteFile(maskFile, []byte("# Existing flat file\napp-misc/bar::arrans-overlay\n"), 0644); err != nil {
			t.Fatalf("writing flat mask: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask failed: %v", err)
		}

		info, err := os.Stat(maskFile)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.IsDir() {
			t.Fatal("package.mask was converted to directory, want flat file")
		}

		content, _ := os.ReadFile(maskFile)
		expected := "# Existing flat file\napp-misc/bar::arrans-overlay\napp-misc/foo::arrans-overlay\n"
		if string(content) != expected {
			t.Errorf("content = %q, want %q", string(content), expected)
		}
	})

	// 9. directory package.mask
	t.Run("directory package.mask", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskDir := filepath.Join(configRoot, "package.mask")
		if err := os.MkdirAll(maskDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(maskDir, "custom"), []byte("app-misc/other::arrans-overlay\n"), 0644); err != nil {
			t.Fatalf("writing custom: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask failed: %v", err)
		}

		g2Content, err := os.ReadFile(filepath.Join(maskDir, "g2.conf"))
		if err != nil {
			t.Fatalf("reading g2.conf: %v", err)
		}
		if string(g2Content) != "app-misc/foo::arrans-overlay\n" {
			t.Errorf("g2.conf content = %q", string(g2Content))
		}
	})

	// 10. flat-file package.unmask
	t.Run("flat-file package.unmask", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		unmaskFile := filepath.Join(configRoot, "package.unmask")
		if err := os.WriteFile(unmaskFile, []byte("# Existing flat file\napp-misc/bar::arrans-overlay\n"), 0644); err != nil {
			t.Fatalf("writing flat unmask: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("unmask failed: %v", err)
		}

		content, _ := os.ReadFile(unmaskFile)
		expected := "# Existing flat file\napp-misc/bar::arrans-overlay\napp-misc/foo::arrans-overlay\n"
		if string(content) != expected {
			t.Errorf("content = %q, want %q", string(content), expected)
		}
	})

	// 11. directory package.unmask
	t.Run("directory package.unmask", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		unmaskDir := filepath.Join(configRoot, "package.unmask")
		if err := os.MkdirAll(unmaskDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("unmask failed: %v", err)
		}

		g2Content, err := os.ReadFile(filepath.Join(unmaskDir, "g2.conf"))
		if err != nil {
			t.Fatalf("reading g2.conf: %v", err)
		}
		if string(g2Content) != "app-misc/foo::arrans-overlay\n" {
			t.Errorf("g2.conf content = %q", string(g2Content))
		}
	})

	// 12. headerless user configuration
	t.Run("headerless user configuration", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskFile := filepath.Join(configRoot, "package.mask")
		// Headerless configuration without maintainer metadata
		if err := os.WriteFile(maskFile, []byte("app-misc/foo::arrans-overlay\n>=dev-util/bar-2.0::arrans-overlay\n"), 0644); err != nil {
			t.Fatalf("writing maskFile: %v", err)
		}

		out, err := captureOutput(t, func() error {
			return cfg.cmdConfOverlay([]string{"arrans-overlay", "list", "--config-root", configRoot, "--repos-conf", reposConf})
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		if !strings.Contains(out, "app-misc/foo::arrans-overlay") {
			t.Errorf("list output missing app-misc/foo: %s", out)
		}
		if !strings.Contains(out, ">=dev-util/bar-2.0::arrans-overlay") {
			t.Errorf("list output missing dev-util/bar: %s", out)
		}
	})

	// 13. comments/blank lines preserved
	t.Run("comments/blank lines preserved", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskFile := filepath.Join(configRoot, "package.mask")
		initial := `# Important header comment

# Group 1
app-misc/foo::arrans-overlay # inline note

# Group 2
dev-libs/bar::arrans-overlay
`
		if err := os.WriteFile(maskFile, []byte(initial), 0644); err != nil {
			t.Fatalf("writing maskFile: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask-reset failed: %v", err)
		}

		content, _ := os.ReadFile(maskFile)
		expected := `# Important header comment

# Group 1

# Group 2
dev-libs/bar::arrans-overlay
`
		if string(content) != expected {
			t.Errorf("content =\n%s\nwant =\n%s", string(content), expected)
		}
	})

	// 14. unrelated entries preserved
	t.Run("unrelated entries preserved", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskFile := filepath.Join(configRoot, "package.mask")
		initial := "app-misc/unrelated::guru\ndev-libs/bar::arrans-overlay\n"
		if err := os.WriteFile(maskFile, []byte(initial), 0644); err != nil {
			t.Fatalf("writing maskFile: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", "dev-libs/bar", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask-reset failed: %v", err)
		}

		content, _ := os.ReadFile(maskFile)
		if !strings.Contains(string(content), "app-misc/unrelated::guru\n") {
			t.Errorf("unrelated entry was lost: %s", string(content))
		}
	})

	// 15. unrelated fragments preserved
	t.Run("unrelated fragments preserved", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskDir := filepath.Join(configRoot, "package.mask")
		if err := os.MkdirAll(maskDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		userFile := filepath.Join(maskDir, "99-my-user-rules")
		if err := os.WriteFile(userFile, []byte("app-misc/keep::guru\n"), 0644); err != nil {
			t.Fatalf("writing userFile: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask failed: %v", err)
		}

		userContent, err := os.ReadFile(userFile)
		if err != nil {
			t.Fatalf("userFile missing: %v", err)
		}
		if string(userContent) != "app-misc/keep::guru\n" {
			t.Errorf("userFile was modified: %s", string(userContent))
		}
	})

	// 16. duplicate mask add is idempotent
	t.Run("duplicate mask add is idempotent", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err1 := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err1 != nil {
			t.Fatalf("first mask failed: %v", err1)
		}
		err2 := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err2 != nil {
			t.Fatalf("second mask failed: %v", err2)
		}

		content, _ := os.ReadFile(filepath.Join(configRoot, "package.mask", "g2.conf"))
		if strings.Count(string(content), "app-misc/foo::arrans-overlay") != 1 {
			t.Errorf("expected exactly 1 occurrence, got content:\n%s", string(content))
		}
	})

	// 17. duplicate unmask add is idempotent
	t.Run("duplicate unmask add is idempotent", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err1 := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err1 != nil {
			t.Fatalf("first unmask failed: %v", err1)
		}
		err2 := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err2 != nil {
			t.Fatalf("second unmask failed: %v", err2)
		}

		content, _ := os.ReadFile(filepath.Join(configRoot, "package.unmask", "g2.conf"))
		if strings.Count(string(content), "app-misc/foo::arrans-overlay") != 1 {
			t.Errorf("expected exactly 1 occurrence, got content:\n%s", string(content))
		}
	})

	// 18. repeated mask-reset is idempotent
	t.Run("repeated mask-reset is idempotent", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		_ = cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})

		err1 := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err1 != nil {
			t.Fatalf("first mask-reset failed: %v", err1)
		}
		err2 := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err2 != nil {
			t.Fatalf("second mask-reset failed: %v", err2)
		}
	})

	// 19. repeated unmask-reset is idempotent
	t.Run("repeated unmask-reset is idempotent", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		_ = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})

		err1 := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask-reset", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err1 != nil {
			t.Fatalf("first unmask-reset failed: %v", err1)
		}
		err2 := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask-reset", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err2 != nil {
			t.Fatalf("second unmask-reset failed: %v", err2)
		}
	})

	// 20. <repo> list sees a rule created by this implementation
	t.Run("<repo> list sees a rule created by this implementation", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		_ = cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		_ = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "dev-util/bar", "--config-root", configRoot, "--repos-conf", reposConf})

		out, err := captureOutput(t, func() error {
			return cfg.cmdConfOverlay([]string{"arrans-overlay", "list", "--config-root", configRoot, "--repos-conf", reposConf})
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		if !strings.Contains(out, "app-misc/foo::arrans-overlay") {
			t.Errorf("missing mask rule in list: %s", out)
		}
		if !strings.Contains(out, "dev-util/bar::arrans-overlay") {
			t.Errorf("missing unmask rule in list: %s", out)
		}
	})

	// 21. <repo> list sees rules in arbitrary fragments
	t.Run("<repo> list sees rules in arbitrary fragments", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskDir := filepath.Join(configRoot, "package.mask")
		if err := os.MkdirAll(maskDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(maskDir, "50-custom.conf"), []byte("net-misc/curl::arrans-overlay\n"), 0644); err != nil {
			t.Fatalf("writing custom fragment: %v", err)
		}

		out, err := captureOutput(t, func() error {
			return cfg.cmdConfOverlay([]string{"arrans-overlay", "list", "--config-root", configRoot, "--repos-conf", reposConf})
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		if !strings.Contains(out, "net-misc/curl::arrans-overlay") {
			t.Errorf("missing rule from custom fragment: %s", out)
		}
		if !strings.Contains(out, "50-custom.conf:1") {
			t.Errorf("missing line provenance in list: %s", out)
		}
	})

	// 22. <repo> list excludes another repository's rules
	t.Run("<repo> list excludes another repository's rules", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskDir := filepath.Join(configRoot, "package.mask")
		if err := os.MkdirAll(maskDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(maskDir, "local"), []byte("app-misc/foo::guru\n"), 0644); err != nil {
			t.Fatalf("writing local: %v", err)
		}

		out, err := captureOutput(t, func() error {
			return cfg.cmdConfOverlay([]string{"arrans-overlay", "list", "--config-root", configRoot, "--repos-conf", reposConf})
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		if strings.Contains(out, "app-misc/foo::guru") {
			t.Errorf("list unexpectedly included rule for another repo: %s", out)
		}
	})

	// 23. g2 conf overlay list lists configured repositories
	t.Run("g2 conf overlay list lists configured repositories", func(t *testing.T) {
		_, reposConf := setupTestPortage(t)
		out, err := captureOutput(t, func() error {
			return cfg.cmdConfOverlay([]string{"list", "--repos-conf", reposConf})
		})
		if err != nil {
			t.Fatalf("conf overlay list failed: %v", err)
		}

		if !strings.Contains(out, "arrans-overlay") {
			t.Errorf("missing arrans-overlay in repo list: %s", out)
		}
		if !strings.Contains(out, "guru") {
			t.Errorf("missing guru in repo list: %s", out)
		}
	})

	// 24. disabled repositories are rejected/excluded appropriately
	t.Run("disabled repositories are rejected/excluded appropriately", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		out, err := captureOutput(t, func() error {
			return cfg.cmdConfOverlay([]string{"list", "--repos-conf", reposConf})
		})
		if err != nil {
			t.Fatalf("conf overlay list failed: %v", err)
		}

		if strings.Contains(out, "disabled-repo") || strings.Contains(out, "flag-disabled") {
			t.Errorf("disabled repo found in repo list: %s", out)
		}

		// Attempting mutation on disabled repo fails
		err = cfg.cmdConfOverlay([]string{"disabled-repo", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error masking against disabled repository")
		}

		err = cfg.cmdConfOverlay([]string{"flag-disabled", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error masking against flag-disabled repository")
		}
	})

	// 25. directory-form repos.conf
	t.Run("directory-form repos.conf", func(t *testing.T) {
		tmpDir := t.TempDir()
		reposConfDir := filepath.Join(tmpDir, "repos.conf")
		if err := os.MkdirAll(reposConfDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		f1 := filepath.Join(reposConfDir, "arrans.conf")
		if err := os.WriteFile(f1, []byte("[arrans-overlay]\nlocation = /var/empty\n"), 0644); err != nil {
			t.Fatalf("write f1: %v", err)
		}

		configRoot := filepath.Join(tmpDir, "portage")
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConfDir})
		if err != nil {
			t.Fatalf("mask with directory repos.conf failed: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(configRoot, "package.mask", "g2.conf"))
		if !strings.Contains(string(content), "app-misc/foo::arrans-overlay\n") {
			t.Errorf("expected rule in g2.conf: %s", string(content))
		}
	})

	// 26. missing repo produces an error
	t.Run("missing repo produces an error", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		err := cfg.cmdConfOverlay([]string{"non-existent-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error for non-existent repository")
		}
		if !strings.Contains(err.Error(), "not found or not enabled") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// 27. explicit --repos-conf cannot accidentally fall through to host /var/db/repos
	t.Run("explicit --repos-conf cannot accidentally fall through to host /var/db/repos", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyReposConf := filepath.Join(tmpDir, "empty-repos.conf")
		if err := os.WriteFile(emptyReposConf, []byte("[DEFAULT]\nmain-repo = gentoo\n"), 0644); err != nil {
			t.Fatalf("writing empty repos.conf: %v", err)
		}

		// Even if gentoo is on host /var/db/repos/gentoo, explicit repos.conf without gentoo section fails
		err := cfg.cmdConfOverlay([]string{"gentoo", "mask", "sys-apps/portage", "--config-root", tmpDir, "--repos-conf", emptyReposConf})
		if err == nil {
			t.Fatal("expected error resolving gentoo from repos.conf without gentoo section")
		}
	})

	// 28. repository section name vs actual repository identity where applicable
	t.Run("repository section name vs actual repository identity", func(t *testing.T) {
		tmpDir := t.TempDir()
		overlayDir := filepath.Join(tmpDir, "overlay-dir")
		if err := os.MkdirAll(filepath.Join(overlayDir, "profiles"), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(overlayDir, "profiles", "repo_name"), []byte("actual-overlay-name\n"), 0644); err != nil {
			t.Fatalf("write repo_name: %v", err)
		}

		reposConf := filepath.Join(tmpDir, "repos.conf")
		confContent := `[section-alias]
location = ` + overlayDir + `
`
		if err := os.WriteFile(reposConf, []byte(confContent), 0644); err != nil {
			t.Fatalf("writing repos.conf: %v", err)
		}

		configRoot := filepath.Join(tmpDir, "portage")
		// Mask using section alias
		err := cfg.cmdConfOverlay([]string{"section-alias", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask failed: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(configRoot, "package.mask", "g2.conf"))
		// Must use actual repository identity
		if !strings.Contains(string(content), "app-misc/foo::actual-overlay-name\n") {
			t.Errorf("expected actual-overlay-name qualifier, got content:\n%s", string(content))
		}
	})

	// 29. cleanup/write errors where reasonably injectable
	t.Run("cleanup/write errors where reasonably injectable", func(t *testing.T) {
		tmpDir := t.TempDir()
		reposConf := filepath.Join(tmpDir, "repos.conf")
		if err := os.WriteFile(reposConf, []byte("[arrans-overlay]\nlocation = /var/empty\n"), 0644); err != nil {
			t.Fatalf("writing repos.conf: %v", err)
		}

		// Create a file where directory is expected (uncreatable directory)
		configRoot := filepath.Join(tmpDir, "portage")
		if err := os.WriteFile(configRoot, []byte("regular file blocking directory creation"), 0644); err != nil {
			t.Fatalf("writing blocking file: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/foo", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected write error when config-root is blocked")
		}
	})

	// 30. file permissions are preserved during replacement
	t.Run("file permissions are preserved during replacement", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)
		maskFile := filepath.Join(configRoot, "package.mask")
		if err := os.WriteFile(maskFile, []byte("app-misc/foo::arrans-overlay\n"), 0600); err != nil {
			t.Fatalf("writing maskFile: %v", err)
		}

		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "app-misc/bar", "--config-root", configRoot, "--repos-conf", reposConf})
		if err != nil {
			t.Fatalf("mask failed: %v", err)
		}

		info, err := os.Stat(maskFile)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("file mode = %v, want 0600", info.Mode().Perm())
		}
	})

	// 31. repo-wide wildcard atom rejection (regression test for safety bypass)
	t.Run("repo-wide wildcard atom is rejected from package operations", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)

		// unmask */* fails and does not create package.unmask
		err := cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "*/*", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when unmasking wildcard */*")
		}
		if !strings.Contains(err.Error(), "repo-wide wildcard atom") {
			t.Errorf("unexpected error message: %v", err)
		}
		if _, err := os.Stat(filepath.Join(configRoot, "package.unmask")); !os.IsNotExist(err) {
			t.Error("package.unmask should not have been created on rejected wildcard unmask")
		}

		// unmask */*::repo fails and does not create package.unmask
		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "*/*::arrans-overlay", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when unmasking explicitly qualified wildcard */*::arrans-overlay")
		}
		if _, err := os.Stat(filepath.Join(configRoot, "package.unmask")); !os.IsNotExist(err) {
			t.Error("package.unmask should not have been created on rejected wildcard unmask")
		}

		// mask */* fails and does not create package.mask
		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "*/*", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when masking wildcard */*")
		}
		if _, err := os.Stat(filepath.Join(configRoot, "package.mask")); !os.IsNotExist(err) {
			t.Error("package.mask should not have been created on rejected wildcard mask")
		}

		// mask-reset */* fails
		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", "*/*", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when mask-reset wildcard */*")
		}

		// unmask-reset */* fails
		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask-reset", "*/*", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error when unmask-reset wildcard */*")
		}
	})

	// 32. malformed and invalid atoms rejected at command level without modifying config
	t.Run("malformed atoms rejected without modifying config", func(t *testing.T) {
		invalidAtoms := []string{
			"bare-package",
			"/leading-slash",
			"category/",
			"category/pkg/extra",
			"cat / pkg",
			"category/pkg::",
			"category/pkg::repo1::repo2",
			">=cat/pkg",
			"cat/pkg-1.2",
			"+cat/pkg",
			".cat/pkg",
			"cat/+pkg",
			"cat/pkg.name",
			"cat/pkg@name",
			"=cat/pkg-1..2",
		}

		for _, invalidAtom := range invalidAtoms {
			configRoot, reposConf := setupTestPortage(t)

			// mask command
			err := cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", invalidAtom, "--config-root", configRoot, "--repos-conf", reposConf})
			if err == nil {
				t.Errorf("expected mask error for invalid atom %q, but got nil", invalidAtom)
			}
			if _, err := os.Stat(filepath.Join(configRoot, "package.mask")); !os.IsNotExist(err) {
				t.Errorf("package.mask should not have been created for invalid atom %q", invalidAtom)
			}

			// unmask command
			err = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", invalidAtom, "--config-root", configRoot, "--repos-conf", reposConf})
			if err == nil {
				t.Errorf("expected unmask error for invalid atom %q, but got nil", invalidAtom)
			}
			if _, err := os.Stat(filepath.Join(configRoot, "package.unmask")); !os.IsNotExist(err) {
				t.Errorf("package.unmask should not have been created for invalid atom %q", invalidAtom)
			}

			// mask-reset command
			err = cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", invalidAtom, "--config-root", configRoot, "--repos-conf", reposConf})
			if err == nil {
				t.Errorf("expected mask-reset error for invalid atom %q, but got nil", invalidAtom)
			}

			// unmask-reset command
			err = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask-reset", invalidAtom, "--config-root", configRoot, "--repos-conf", reposConf})
			if err == nil {
				t.Errorf("expected unmask-reset error for invalid atom %q, but got nil", invalidAtom)
			}
		}
	})

	// 33. surplus positional arguments produce clear errors
	t.Run("surplus positional arguments produce clear errors", func(t *testing.T) {
		configRoot, reposConf := setupTestPortage(t)

		err := cfg.cmdConfOverlay([]string{"list", "unexpected", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
			t.Errorf("expected unexpected argument error for conf overlay list, got: %v", err)
		}

		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "list", "unexpected", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
			t.Errorf("expected unexpected argument error for repo list, got: %v", err)
		}

		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "mask", "cat/pkg", "extra_pkg", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil || !strings.Contains(err.Error(), "unexpected extra argument") {
			t.Errorf("expected unexpected extra argument error for mask, got: %v", err)
		}

		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask", "cat/pkg", "extra_pkg", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil || !strings.Contains(err.Error(), "unexpected extra argument") {
			t.Errorf("expected unexpected extra argument error for unmask, got: %v", err)
		}

		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "mask-reset", "cat/pkg", "extra_pkg", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil || !strings.Contains(err.Error(), "unexpected extra argument") {
			t.Errorf("expected unexpected extra argument error for mask-reset, got: %v", err)
		}

		err = cfg.cmdConfOverlay([]string{"arrans-overlay", "unmask-reset", "cat/pkg", "extra_pkg", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil || !strings.Contains(err.Error(), "unexpected extra argument") {
			t.Errorf("expected unexpected extra argument error for unmask-reset, got: %v", err)
		}
	})

	// 34. malformed repository identity cannot cause package.mask / package.unmask write
	t.Run("malformed repository identity prevents configuration mutation", func(t *testing.T) {
		tmpDir := t.TempDir()
		overlayDir := filepath.Join(tmpDir, "overlay")
		if err := os.MkdirAll(filepath.Join(overlayDir, "profiles"), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// Injection attempt in profiles/repo_name
		if err := os.WriteFile(filepath.Join(overlayDir, "profiles", "repo_name"), []byte("real-repo\napp-misc/injected\n"), 0644); err != nil {
			t.Fatalf("write repo_name: %v", err)
		}

		reposConf := filepath.Join(tmpDir, "repos.conf")
		confContent := `[test-repo]
location = ` + overlayDir + `
`
		if err := os.WriteFile(reposConf, []byte(confContent), 0644); err != nil {
			t.Fatalf("write repos.conf: %v", err)
		}

		configRoot := filepath.Join(tmpDir, "portage")

		// mask attempt
		err := cfg.cmdConfOverlay([]string{"test-repo", "mask", "cat/pkg", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error on malformed repository identity, got nil")
		}
		if _, err := os.Stat(filepath.Join(configRoot, "package.mask")); !os.IsNotExist(err) {
			t.Error("package.mask should not have been created on malformed repo identity")
		}

		// unmask attempt
		err = cfg.cmdConfOverlay([]string{"test-repo", "unmask", "cat/pkg", "--config-root", configRoot, "--repos-conf", reposConf})
		if err == nil {
			t.Fatal("expected error on malformed repository identity, got nil")
		}
		if _, err := os.Stat(filepath.Join(configRoot, "package.unmask")); !os.IsNotExist(err) {
			t.Error("package.unmask should not have been created on malformed repo identity")
		}
	})
}
