package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func writeTestEbuild(t *testing.T, dir, name, content string) {
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing ebuild %s: %v", name, err)
	}
}

func writeTestManifest(t *testing.T, dir, content string) {
	err := os.WriteFile(filepath.Join(dir, "Manifest"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing manifest: %v", err)
	}
}

func TestCmdVerifyTransactional(t *testing.T) {
	// Start fake HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail.tar.gz" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/good.tar.gz" {
			w.Write([]byte("good data"))
			return
		}
		if r.URL.Path == "/missing.tar.gz" {
			w.Write([]byte("missing data"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	hashes := []string{g2.HashSha512}

	t.Run("Missing + Obsolete DIST: Fix + Clean writes exact set once", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/good.tar.gz -> good.tar.gz"
`, ts.URL))

		// Manifest has obsolete entry, missing good.tar.gz
		manifestContent := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, manifestContent)

		cfg := &CmdManifestArgConfig{
			MainArgConfig: &MainArgConfig{},
		}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err != nil {
			t.Fatalf("cmdVerify failed: %v", err)
		}

		b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
		if err != nil {
			t.Fatalf("reading manifest: %v", err)
		}
		result := string(b)
		if !strings.Contains(result, "DIST good.tar.gz") {
			t.Errorf("Expected good.tar.gz in manifest, got: %s", result)
		}
		if strings.Contains(result, "obsolete.tar.gz") {
			t.Errorf("Expected obsolete.tar.gz to be removed, got: %s", result)
		}
	})

	t.Run("Failed download aborts transaction", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/fail.tar.gz -> fail.tar.gz"
`, ts.URL))

		originalManifest := "DIST other.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, originalManifest)

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err == nil {
			t.Fatalf("Expected cmdVerify to fail")
		}

		b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
		if err != nil {
			t.Fatalf("reading manifest: %v", err)
		}
		if string(b) != originalManifest {
			t.Errorf("Manifest was modified on failure! Got: %s", string(b))
		}
	})

	t.Run("Uncertain ebuild aborts clean and fix", func(t *testing.T) {
		dir := t.TempDir()
		// Uses inherit without explicit SRC_URI => uncertain
		writeTestEbuild(t, dir, "foo-1.0.ebuild", `EAPI=8
inherit someclass
DESCRIPTION="foo"
SLOT="0"
`)
		originalManifest := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, originalManifest)

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err == nil {
			t.Fatalf("Expected error about uncertainty")
		}

		b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
		if err != nil {
			t.Fatalf("reading manifest: %v", err)
		}
		if string(b) != originalManifest {
			t.Errorf("Manifest was modified despite uncertainty! Got: %s", string(b))
		}
	})

	t.Run("Revisioned P vs PF filenames", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0-r1.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/good.tar.gz -> ${PF}.tar.gz
%s/missing.tar.gz -> ${P}.tar.gz"
`, ts.URL, ts.URL))

		writeTestManifest(t, dir, "")

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", dir}, hashes)
		if err != nil {
			t.Fatalf("cmdVerify failed: %v", err)
		}

		b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
		if err != nil {
			t.Fatalf("reading manifest: %v", err)
		}
		result := string(b)
		if !strings.Contains(result, "foo-1.0-r1.tar.gz") {
			t.Errorf("Expected PF resolution (foo-1.0-r1.tar.gz): %s", result)
		}
		if !strings.Contains(result, "foo-1.0.tar.gz") {
			t.Errorf("Expected P resolution (foo-1.0.tar.gz): %s", result)
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/good.tar.gz -> good.tar.gz"
`, ts.URL))

		writeTestManifest(t, dir, "")

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err != nil {
			t.Fatalf("cmdVerify failed: %v", err)
		}

		b1, _ := os.ReadFile(filepath.Join(dir, "Manifest"))

		err = cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err != nil {
			t.Fatalf("cmdVerify failed 2nd time: %v", err)
		}

		b2, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		if string(b1) != string(b2) {
			t.Errorf("Idempotency failed")
		}
	})
}

func TestCmdUpsertFromUrl(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("good data"))
	}))
	defer ts.Close()

	hashes := []string{g2.HashSha512}

	t.Run("Preserves revision text", func(t *testing.T) {
		dir := t.TempDir()
		writeTestManifest(t, dir, "")

		cfg := &CmdManifestArgConfig{
			MainArgConfig: &MainArgConfig{},
		}
		// Notice filename is "foo-1.0-r1.tar.gz"
		err := cfg.cmdUpsertFromUrl([]string{ts.URL + "/good.tar.gz", "foo-1.0-r1.tar.gz", dir}, hashes)
		if err != nil {
			t.Fatalf("cmdUpsertFromUrl failed: %v", err)
		}

		b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
		if err != nil {
			t.Fatalf("reading manifest: %v", err)
		}
		result := string(b)
		if !strings.Contains(result, "DIST foo-1.0-r1.tar.gz") {
			t.Errorf("Expected exact filename to be preserved, got: %s", result)
		}
	})
}
