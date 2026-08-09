package g2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestParseInfoVarsReader(t *testing.T) {
	input := `# Copyright 2004-2023 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

##
## These variables are printed with a standard 'emerge info' in
## portage as of 2.0.51-r5. They are sorted regardless of order here.
##
ACCEPT_KEYWORDS
ACCEPT_LICENSE

ADDR2LINE
AR
`

	expected := []string{
		"ACCEPT_KEYWORDS",
		"ACCEPT_LICENSE",
		"ADDR2LINE",
		"AR",
	}

	r := bytes.NewReader([]byte(input))
	parsed, err := parseInfoVarsReader(r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(parsed, expected) {
		t.Errorf("expected %v, got %v", expected, parsed)
	}
}

func TestSerializeInfoVars(t *testing.T) {
	vars := []string{
		"ACCEPT_KEYWORDS",
		"ACCEPT_LICENSE",
		"ADDR2LINE",
		"AR",
	}

	expected := "ACCEPT_KEYWORDS\nACCEPT_LICENSE\nADDR2LINE\nAR\n"

	var buf bytes.Buffer
	err := SerializeInfoVars(&buf, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestParseInfoVarsFS(t *testing.T) {
	input := `# Comment
VAR1
VAR2
`
	sysFS := fstest.MapFS{
		"profiles/info_vars": &fstest.MapFile{Data: []byte(input)},
	}

	expected := []string{"VAR1", "VAR2"}

	t.Run("existing file", func(t *testing.T) {
		parsed, err := ParseInfoVarsFS(sysFS, "profiles/info_vars")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(parsed, expected) {
			t.Errorf("expected %v, got %v", expected, parsed)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		parsed, err := ParseInfoVarsFS(sysFS, "profiles/does_not_exist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed != nil {
			t.Errorf("expected nil, got %v", parsed)
		}
	})
}

func TestParseInfoVars(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "info_vars")

	input := `# Comment
VAR1
VAR2
`
	err := os.WriteFile(path, []byte(input), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	expected := []string{"VAR1", "VAR2"}

	t.Run("existing file", func(t *testing.T) {
		parsed, err := ParseInfoVars(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(parsed, expected) {
			t.Errorf("expected %v, got %v", expected, parsed)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		parsed, err := ParseInfoVars(filepath.Join(tempDir, "does_not_exist"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed != nil {
			t.Errorf("expected nil, got %v", parsed)
		}
	})
}

func TestWriteInfoVarsFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "info_vars")

	vars := []string{"VAR1", "VAR2"}
	expected := "VAR1\nVAR2\n"

	err := WriteInfoVarsFile(path, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if string(content) != expected {
		t.Errorf("expected %q, got %q", expected, string(content))
	}
}
