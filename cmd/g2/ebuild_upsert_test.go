package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEbuildUpsert_NoFiles(t *testing.T) {
	dir := t.TempDir()

	cfg := &CmdEbuildArgConfig{}

	// Mock stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("some ebuild content"))
		_ = w.Close()
	}()

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	err = cfg.cmdEbuildUpsert([]string{
		"--dir", dir,
		"--package", "dummy",
		"--version", "1.2",
	})

	_ = outW.Close()

	if err != nil {
		t.Fatalf("cmdEbuildUpsert failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(outR)

	expectedFile := filepath.Join(dir, "dummy-1.2.ebuild")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("Expected file %s was not created", expectedFile)
	}

	content, _ := os.ReadFile(expectedFile)
	if string(content) != "some ebuild content" {
		t.Fatalf("Unexpected content: %s", string(content))
	}
}

func TestEbuildUpsert_ContentMatches(t *testing.T) {
	dir := t.TempDir()

	expectedFile := filepath.Join(dir, "dummy-1.2.ebuild")
	_ = os.WriteFile(expectedFile, []byte("same content"), 0644)

	cfg := &CmdEbuildArgConfig{}

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("same content"))
		_ = w.Close()
	}()

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	_, outW, _ := os.Pipe()
	os.Stdout = outW

	err = cfg.cmdEbuildUpsert([]string{
		"--dir", dir,
		"--package", "dummy",
		"--version", "1.2",
	})

	_ = outW.Close()

	if err != nil {
		t.Fatalf("cmdEbuildUpsert failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "dummy-1.2-r1.ebuild")); !os.IsNotExist(err) {
		t.Fatalf("Expected dummy-1.2-r1.ebuild to not be created")
	}
}

func TestEbuildUpsert_ContentDiffers(t *testing.T) {
	dir := t.TempDir()

	expectedFile := filepath.Join(dir, "dummy-1.2.ebuild")
	_ = os.WriteFile(expectedFile, []byte("old content"), 0644)

	cfg := &CmdEbuildArgConfig{}

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("new content"))
		_ = w.Close()
	}()

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	_, outW, _ := os.Pipe()
	os.Stdout = outW

	err = cfg.cmdEbuildUpsert([]string{
		"--dir", dir,
		"--package", "dummy",
		"--version", "1.2",
	})

	_ = outW.Close()

	if err != nil {
		t.Fatalf("cmdEbuildUpsert failed: %v", err)
	}

	r1File := filepath.Join(dir, "dummy-1.2-r1.ebuild")
	if _, err := os.Stat(r1File); os.IsNotExist(err) {
		t.Fatalf("Expected %s to be created", r1File)
	}
}


func TestEbuildUpsert_FromFile(t *testing.T) {
	dir := t.TempDir()

	inputFile := filepath.Join(dir, "input.ebuild")
	_ = os.WriteFile(inputFile, []byte("some file content"), 0644)

	cfg := &CmdEbuildArgConfig{}

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	err := cfg.cmdEbuildUpsert([]string{
		"--dir", dir,
		"--package", "dummy",
		"--version", "1.2",
		inputFile,
	})

	_ = outW.Close()

	if err != nil {
		t.Fatalf("cmdEbuildUpsert failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(outR)

	expectedFile := filepath.Join(dir, "dummy-1.2.ebuild")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("Expected file %s was not created", expectedFile)
	}

	content, _ := os.ReadFile(expectedFile)
	if string(content) != "some file content" {
		t.Fatalf("Unexpected content: %s", string(content))
	}
}

func TestEbuildUpsert_WithoutVersionFlag(t *testing.T) {
	dir := t.TempDir()

	inputFile := filepath.Join(dir, "input.ebuild")
	_ = os.WriteFile(inputFile, []byte("PV=2.0\nsome content"), 0644)

	cfg := &CmdEbuildArgConfig{}

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	err := cfg.cmdEbuildUpsert([]string{
		"--dir", dir,
		"--package", "dummy",
		inputFile,
	})

	_ = outW.Close()

	if err != nil {
		t.Fatalf("cmdEbuildUpsert failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(outR)

	expectedFile := filepath.Join(dir, "dummy-2.0.ebuild")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("Expected file %s was not created", expectedFile)
	}
}

func TestEbuildUpsert_WithExplicitVersionBumpType(t *testing.T) {
	dir := t.TempDir()

	expectedFile := filepath.Join(dir, "dummy-1.2.ebuild")
	_ = os.WriteFile(expectedFile, []byte("old content"), 0644)

	cfg := &CmdEbuildArgConfig{}

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("PV=1.2\nnew content"))
		_ = w.Close()
	}()

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	_, outW, _ := os.Pipe()
	os.Stdout = outW

	err = cfg.cmdEbuildUpsert([]string{
		"--dir", dir,
		"--package", "dummy",
		"1.3",
		"-",
	})

	_ = outW.Close()

	if err != nil {
		t.Fatalf("cmdEbuildUpsert failed: %v", err)
	}

	newFile := filepath.Join(dir, "dummy-1.3.ebuild")
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Fatalf("Expected %s to be created", newFile)
	}
}
