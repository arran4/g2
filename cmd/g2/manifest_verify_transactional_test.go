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
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/fail.tar.gz" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/trunc.tar.gz" {
			w.Header().Set("Content-Length", "1000") // Claim 1000 bytes
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("truncated")) // Send only 9 bytes
			return
		}
		if r.URL.Path == "/good.tar.gz" {
			_, _ = w.Write([]byte("good data"))
			return
		}
		if r.URL.Path == "/missing.tar.gz" {
			_, _ = w.Write([]byte("missing data"))
			return
		}
		if r.URL.Path == "/firstgood.tar.gz" {
			_, _ = w.Write([]byte("good data 1"))
			return
		}
		if r.URL.Path == "/laterfail.tar.gz" {
			w.WriteHeader(http.StatusInternalServerError)
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

	t.Run("Uncertain ebuild aborts clean and fix", func(t *testing.T) {
		dir := t.TempDir()
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

	t.Run("Response-body processing failure aborted safely", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/trunc.tar.gz -> trunc.tar.gz"
`, ts.URL))
		originalManifest := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, originalManifest)

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err == nil {
			t.Fatalf("Expected cmdVerify to fail")
		}
		if strings.Contains(err.Error(), "bad status") {
			t.Fatalf("Expected body processing failure, but got bad status error: %v", err)
		}

		b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		if string(b) != originalManifest {
			t.Errorf("Manifest was modified on failure! Got: %s", string(b))
		}
	})

	t.Run("Successful first fetch, failing later fetch preserves exact Manifest bytes", func(t *testing.T) {
		for _, clean := range []bool{true, false} {
			dir := t.TempDir()
			writeTestEbuild(t, dir, "foo-1.0.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/firstgood.tar.gz -> firstgood.tar.gz
%s/laterfail.tar.gz -> laterfail.tar.gz"
`, ts.URL, ts.URL))

			originalManifest := "DIST obsolete.tar.gz 123 SHA512 abc\n"
			writeTestManifest(t, dir, originalManifest)

			cfg := &CmdManifestArgConfig{
				MainArgConfig: &MainArgConfig{},
			}
			args := []string{"--fix"}
			if clean {
				args = append(args, "--clean")
			}
			args = append(args, dir)

			err := cfg.cmdVerify(args, hashes)
			if err == nil {
				t.Fatalf("cmdVerify expected to fail due to laterfail.tar.gz with clean=%v", clean)
			}

			b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
			if string(b) != originalManifest {
				t.Errorf("Manifest modified despite failed later download (clean=%v)! Got:\n%s", clean, string(b))
			}
		}
	})
	t.Run("Transitive unknown variables conservatively aborted", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", `EAPI=8
DESCRIPTION="foo"
SLOT="0"
MY_SOURCE="${UNKNOWN_SOURCE}"
SRC_URI="${MY_SOURCE}"
`)
		originalManifest := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, originalManifest)

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err == nil {
			t.Fatalf("cmdVerify expected to abort due to transitive uncertainty")
		}

		b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		if string(b) != originalManifest {
			t.Errorf("Manifest modified despite uncertainty! Got:\n%s", string(b))
		}
	})

	t.Run("Unsupported shell control-flow syntax silently treated as authoritative aborted", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", `EAPI=8
SRC_URI="" && SRC_URI="https://example.invalid/a.tar.gz"
`)
		originalManifest := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, originalManifest)

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err == nil {
			t.Fatalf("cmdVerify expected to abort due to control flow `&&`")
		}
		b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		if string(b) != originalManifest {
			t.Errorf("Manifest modified despite uncertainty! Got:\n%s", string(b))
		}
	})

	t.Run("Unsupported source control flow aborted", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", `EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="https://example.invalid/a.tar.gz"
if false; then
	SRC_URI=""
fi
`)
		originalManifest := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, originalManifest)

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err == nil {
			t.Fatalf("cmdVerify expected to abort due to control flow")
		}

		b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		if string(b) != originalManifest {
			t.Errorf("Manifest modified despite uncertainty! Got:\n%s", string(b))
		}
	})

	t.Run("Valid conditional source lists plus malformed groups/rename targets", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", `EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="foo? ( https://example.invalid/a.tar.gz -> )"
`)
		writeTestManifest(t, dir, "")

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", dir}, hashes)
		if err == nil {
			t.Fatalf("cmdVerify expected to fail due to malformed -> target")
		}
	})

	t.Run("Revisioned P vs PF filenames reconciliation coverage", func(t *testing.T) {
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
		b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		result := string(b)
		if !strings.Contains(result, "foo-1.0-r1.tar.gz") {
			t.Errorf("Expected PF resolution (foo-1.0-r1.tar.gz): %s", result)
		}
		if !strings.Contains(result, "foo-1.0.tar.gz") {
			t.Errorf("Expected P resolution (foo-1.0.tar.gz): %s", result)
		}
	})

	t.Run("AtomicWriteManifest failure test deterministically reaching final commit", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/good.tar.gz -> good.tar.gz"
`, ts.URL))
		originalManifest := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, originalManifest)

		// Hook the atomic write function inside main package
		atomicWriteManifest = func(path string, m *g2.Manifest) error {
			return os.ErrPermission
		}
		t.Cleanup(func() { atomicWriteManifest = g2.AtomicWriteManifest })

		cfg := &CmdManifestArgConfig{}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err == nil {
			t.Fatalf("cmdVerify expected to fail on writing manifest")
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("Expected permission denied error from write path, got: %v", err)
		}

		b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		if string(b) != originalManifest {
			t.Errorf("Manifest was modified on failure! Got: %s", string(b))
		}
	})

	t.Run("Exact final DIST set", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", fmt.Sprintf(`EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="%s/good.tar.gz -> good.tar.gz"
`, ts.URL))

		manifestContent := "DIST obsolete.tar.gz 123 SHA512 abc\n"
		writeTestManifest(t, dir, manifestContent)

		cfg := &CmdManifestArgConfig{
			MainArgConfig: &MainArgConfig{},
		}
		err := cfg.cmdVerify([]string{"--fix", "--clean", dir}, hashes)
		if err != nil {
			t.Fatalf("cmdVerify failed: %v", err)
		}

		b, _ := os.ReadFile(filepath.Join(dir, "Manifest"))
		result := string(b)

		lines := strings.Split(strings.TrimSpace(result), "\n")
		if len(lines) != 1 || !strings.HasPrefix(lines[0], "DIST good.tar.gz ") {
			t.Errorf("Expected exact single DIST entry for good.tar.gz, got:\n%s", result)
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		dir := t.TempDir()
		writeTestEbuild(t, dir, "foo-1.0.ebuild", "EAPI=8\nDESCRIPTION=\"foo\"\nSLOT=\"0\"\nSRC_URI=\""+ts.URL+"/good.tar.gz -> good.tar.gz\"\n")
		writeTestManifest(t, dir, "")
		cfg := &CmdManifestArgConfig{
			MainArgConfig: &MainArgConfig{},
		}
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
			t.Errorf("Idempotency failed: b1: %s b2: %s", string(b1), string(b2))
		}
	})
}

func TestCmdUpsertFromUrl(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("good data"))
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
