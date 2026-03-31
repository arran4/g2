package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestEbuildExplainCommand(t *testing.T) {
	fixturePath := "testdata/ebuild-explain-basic.txtar"
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", fixturePath, err)
	}

	ar := txtar.Parse(raw)

	tmpDir, err := os.MkdirTemp("", "ebuild-explain-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var ebuildFile string
	var expectedOutput string
	for _, f := range ar.Files {
		if strings.HasSuffix(f.Name, ".ebuild") {
			ebuildFile = filepath.Join(tmpDir, f.Name)
			err := os.WriteFile(ebuildFile, f.Data, 0644)
			if err != nil {
				t.Fatalf("failed to write %s: %v", f.Name, err)
			}
		} else if f.Name == "expected.txt" {
			expectedOutput = string(f.Data)
		}
	}

	if ebuildFile == "" {
		t.Fatalf("no ebuild file found in fixture")
	}

	cfg := &MainArgConfig{
		Args: []string{"g2"},
	}

	cmdCfg := &CmdEbuildArgConfig{
		MainArgConfig: cfg,
	}

	var buf bytes.Buffer
	err = cmdCfg.cmdEbuildExplain([]string{ebuildFile}, &buf)

	if err != nil {
		t.Fatalf("cmdEbuildExplain failed: %v", err)
	}

	gotOutput := buf.String()

	if strings.TrimSpace(gotOutput) != strings.TrimSpace(expectedOutput) {
		t.Errorf("output mismatch.\nwant:\n%s\ngot:\n%s", expectedOutput, gotOutput)
	}
}
