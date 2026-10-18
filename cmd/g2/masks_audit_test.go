package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arran4/g2"
	"golang.org/x/tools/txtar"
)

func TestMasksAuditCommand(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile("../../testdata/mask_audit/overlay.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range txtar.Parse(data).Files {
		if f.Name == "expected.json" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(f.Data), "@ROOT@", filepath.ToSlash(root))), 0644); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() map[string]string {
		t.Helper()
		files := map[string]string{}
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			value := info.ModTime().String() + info.Mode().String()
			if !entry.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				value += string(data)
			}
			files[path] = value
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	cfg := &MainArgConfig{Args: []string{"g2", "masks"}}
	args := []string{"audit", "--config-root", filepath.Join(root, "config"), "--profile", filepath.Join(root, "repo/profiles/selected")}
	output, err := captureStdout(t, func() error { return cfg.cmdMasks(args) })
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"10-main:3  cat/obsolete",
		"required=6 non-matching=1 no-change=1 duplicate/shadowed=3 indeterminate=1",
		"would become masked", "covered by", "Do not remove duplicate/shadowed entries together",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing %q in output:\n%s", expected, output)
		}
	}
	jsonOutput, err := captureStdout(t, func() error { return cfg.cmdMasks(append(args, "--format", "json")) })
	if err != nil {
		t.Fatal(err)
	}
	var report g2.UnmaskAuditReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
		t.Fatalf("audit stdout must be valid JSON: %v\n%s", err, jsonOutput)
	}
	if len(report.Entries) != 12 || report.Summary.Required != 6 {
		t.Fatalf("unexpected JSON report: %+v", report)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("audit command modified its inputs")
	}
	// Explicit repository overrides avoid depending on host repos.conf.
	overrideArgs := []string{"--config-root", filepath.Join(root, "config"), "--profile", filepath.Join(root, "repo/profiles/selected"),
		"--repos-conf", filepath.Join(root, "does-not-exist"), "--repository", "test=" + filepath.Join(root, "repo"), "--format", "json"}
	_, err = captureStdout(t, func() error { return cfg.cmdMasksAudit(overrideArgs) })
	if err != nil {
		t.Fatalf("repository override: %v", err)
	}
}

func TestMasksAuditInvalidArguments(t *testing.T) {
	cfg := &MainArgConfig{}
	for _, args := range [][]string{
		{"--format", "invalid"}, {"--apply"}, {"unexpected"}, {"--repository", "invalid"},
	} {
		if err := cfg.cmdMasksAudit(args); err == nil {
			t.Fatalf("expected invalid options %v to fail", args)
		}
	}
	if err := cfg.cmdMasksAudit([]string{"--help"}); err != nil {
		t.Fatalf("help must succeed: %v", err)
	}
}
