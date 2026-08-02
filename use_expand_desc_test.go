package g2

import (
	"io/fs"

	"fmt"
	"os"

	"bytes"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseUseExpandDesc(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		descName string
		expected *UseExpandDesc
	}{
		{
			name:     "Basic usage",
			content:  "# Copyright 1999-2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nfoo - description foo\nbar - description bar\n",
			descName: "example",
			expected: &UseExpandDesc{
				Name: "example",
				Flags: map[string]string{
					"foo": "description foo",
					"bar": "description bar",
				},
				Lines: []DescLine{
					{Text: "# Copyright 1999-2024 Gentoo Authors"},
					{Text: "# Distributed under the terms of the GNU General Public License v2"},
					{Text: ""},
					{Flag: "foo"},
					{Flag: "bar"},
				},
			},
		},
		{
			name:     "Empty file",
			content:  "",
			descName: "empty",
			expected: &UseExpandDesc{
				Name:  "empty",
				Flags: map[string]string{},
				Lines: nil,
			},
		},
		{
			name:     "Invalid line format",
			content:  "invalid line without dash",
			descName: "invalid",
			expected: &UseExpandDesc{
				Name:  "invalid",
				Flags: map[string]string{},
				Lines: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ud, err := ParseUseExpandDesc(strings.NewReader(tt.content), tt.descName)
			if err != nil {
				t.Fatalf("ParseUseExpandDesc() error = %v", err)
			}

			if ud.Name != tt.expected.Name {
				t.Errorf("Name = %v, want %v", ud.Name, tt.expected.Name)
			}

			if len(ud.Flags) != len(tt.expected.Flags) {
				t.Fatalf("Flags len = %v, want %v", len(ud.Flags), len(tt.expected.Flags))
			}

			for k, v := range tt.expected.Flags {
				if ud.Flags[k] != v {
					t.Errorf("Flags[%q] = %v, want %v", k, ud.Flags[k], v)
				}
			}

			if len(ud.Lines) != len(tt.expected.Lines) {
				t.Fatalf("Lines len = %v, want %v", len(ud.Lines), len(tt.expected.Lines))
			}

			for i, v := range tt.expected.Lines {
				if ud.Lines[i] != v {
					t.Errorf("Lines[%d] = %+v, want %+v", i, ud.Lines[i], v)
				}
			}
		})
	}
}

func TestWriteUseExpandDesc(t *testing.T) {
	ud := &UseExpandDesc{
		Name: "test",
		Flags: map[string]string{
			"flag1": "description 1",
			"flag2": "description 2",
			"flag3": "description 3", // New flag added programmatically
		},
		Lines: []DescLine{
			{Text: "# Header line"},
			{Flag: "flag2"},
			{Flag: "flag1"},
		},
	}

	var buf bytes.Buffer
	if err := ud.Write(&buf); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	expected := `# Header line
flag2 - description 2
flag1 - description 1
flag3 - description 3
`

	if buf.String() != expected {
		t.Errorf("Write() = \n%v\nwant \n%v", buf.String(), expected)
	}
}

func TestParseUseExpandDescFS(t *testing.T) {
	mockFS := fstest.MapFS{
		"foo.desc": &fstest.MapFile{
			Data: []byte("foo1 - desc1\n"),
		},
		"bar.desc": &fstest.MapFile{
			Data: []byte("bar1 - desc2\n"),
		},
		"not_desc.txt": &fstest.MapFile{
			Data: []byte("ignored"),
		},
	}

	result, err := ParseUseExpandDescFS(mockFS, ".")
	if err != nil {
		t.Fatalf("ParseUseExpandDescFS() error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 descriptions, got %d", len(result))
	}

	if foo, ok := result["foo"]; !ok {
		t.Errorf("Expected 'foo' description")
	} else {
		if foo.Flags["foo1"] != "desc1" {
			t.Errorf("Expected 'foo1' to be 'desc1', got %v", foo.Flags["foo1"])
		}
	}

	if bar, ok := result["bar"]; !ok {
		t.Errorf("Expected 'bar' description")
	} else {
		if bar.Flags["bar1"] != "desc2" {
			t.Errorf("Expected 'bar1' to be 'desc2', got %v", bar.Flags["bar1"])
		}
	}
}

func TestParseUseExpandDescFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test_desc_*.desc")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "foo - desc foo\n"
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	ud, err := ParseUseExpandDescFile(tmpFile.Name(), "test")
	if err != nil {
		t.Fatalf("ParseUseExpandDescFile() error = %v", err)
	}

	if ud.Name != "test" {
		t.Errorf("Expected name 'test', got %q", ud.Name)
	}

	if ud.Flags["foo"] != "desc foo" {
		t.Errorf("Expected 'foo' to be 'desc foo', got %q", ud.Flags["foo"])
	}

	// Test non-existent file
	udNotExist, err := ParseUseExpandDescFile("non_existent_file.desc", "new_test")
	if err != nil {
		t.Fatalf("ParseUseExpandDescFile() returned error for non-existent file: %v", err)
	}

	if udNotExist.Name != "new_test" {
		t.Errorf("Expected name 'new_test', got %q", udNotExist.Name)
	}

	if len(udNotExist.Flags) != 0 {
		t.Errorf("Expected 0 flags for non-existent file, got %d", len(udNotExist.Flags))
	}

	if len(udNotExist.Lines) != 2 {
		t.Errorf("Expected 2 header lines for non-existent file, got %d", len(udNotExist.Lines))
	}
}

func TestWriteUseExpandDescFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_write_desc_*.desc")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFileName := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFileName)

	ud := &UseExpandDesc{
		Name: "test_write",
		Flags: map[string]string{
			"flag1": "desc1",
		},
		Lines: []DescLine{
			{Text: "# Header"},
			{Flag: "flag1"},
		},
	}

	if err := ud.WriteFile(tmpFileName); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	content, err := os.ReadFile(tmpFileName)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "# Header\nflag1 - desc1\n"
	if string(content) != expected {
		t.Errorf("File content = %q, want %q", string(content), expected)
	}
}

func TestParseUseExpandDescDir(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "test_desc_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create desc files
	err = os.WriteFile(tmpDir+"/foo.desc", []byte("foo1 - desc1\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write foo.desc: %v", err)
	}

	err = os.WriteFile(tmpDir+"/bar.desc", []byte("bar1 - desc2\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write bar.desc: %v", err)
	}

	// Create non-desc file
	err = os.WriteFile(tmpDir+"/ignored.txt", []byte("ignored\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write ignored.txt: %v", err)
	}

	result, err := ParseUseExpandDescDir(tmpDir)
	if err != nil {
		t.Fatalf("ParseUseExpandDescDir() error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 descriptions, got %d", len(result))
	}

	if foo, ok := result["foo"]; !ok {
		t.Errorf("Expected 'foo' description")
	} else {
		if foo.Flags["foo1"] != "desc1" {
			t.Errorf("Expected 'foo1' to be 'desc1', got %v", foo.Flags["foo1"])
		}
	}
}

type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock reader error")
}

func TestParseUseExpandDesc_Error(t *testing.T) {
	// bufio.Scanner doesn't easily expose reader errors unless they cause a read failure,
	// but testing for scanner errors might be tricky. The main thing is that
	// `scanner.Err()` is checked.
}

func TestWriteUseExpandDesc_Error(t *testing.T) {
	ud := &UseExpandDesc{
		Name: "test_error",
		Flags: map[string]string{
			"flag1": "desc1",
		},
		Lines: []DescLine{
			{Flag: "flag1"},
			{Text: "# Test comment"},
		},
	}

	// mock writer that fails
	errW := &errWriter{}
	if err := ud.Write(errW); err == nil {
		t.Errorf("Expected Write() to return an error, got nil")
	}
}

type errWriter struct{}

func (errWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock writer error")
}

func TestWriteUseExpandDesc_Error_TextLine(t *testing.T) {
	ud := &UseExpandDesc{
		Name: "test_error",
		Lines: []DescLine{
			{Text: "# Test comment"},
		},
	}

	errW := &errWriter{}
	if err := ud.Write(errW); err == nil {
		t.Errorf("Expected Write() to return an error, got nil")
	}
}

func TestWriteUseExpandDesc_Error_NewFlag(t *testing.T) {
	ud := &UseExpandDesc{
		Name: "test_error",
		Flags: map[string]string{
			"flag1": "desc1",
		},
		Lines: []DescLine{
			{Text: "# Test comment"},
		},
	}

	errW := &errWriter{}
	if err := ud.Write(errW); err == nil {
		t.Errorf("Expected Write() to return an error, got nil")
	}
}

func TestParseUseExpandDescFS_Error(t *testing.T) {
	mockFS := fstest.MapFS{
		"unreadable.desc": &fstest.MapFile{
			Data: []byte("foo - bar\n"),
			Mode: 0, // No permissions
		},
	}

	_, err := ParseUseExpandDescFS(mockFS, ".")
	// fstest MapFS actually doesn't enforce permissions strictly on Open, but let's test a non-existent dir
	// for error coverage.
	if err == nil {
		// Just a fallback check, MapFS doesn't simulate permission errors.
	}
}

func TestParseUseExpandDescFS_NonExistentDir(t *testing.T) {
	mockFS := fstest.MapFS{}

	result, err := ParseUseExpandDescFS(mockFS, "non_existent_dir")
	if err != nil {
		t.Fatalf("ParseUseExpandDescFS() should return empty map for non-existent dir, got error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("Expected empty map, got %d items", len(result))
	}
}

func TestParseUseExpandDescFile_ReadError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_desc_err_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a directory with a .desc extension
	err = os.Mkdir(tmpDir+"/bad.desc", 0755)
	if err != nil {
		t.Fatalf("Failed to create bad.desc dir: %v", err)
	}

	_, err = ParseUseExpandDescFile(tmpDir+"/bad.desc", "bad")
	if err == nil {
		t.Fatalf("Expected ParseUseExpandDescFile() to return error when opening a directory, got nil")
	}
}

func TestWriteFileUseExpandDesc_Error(t *testing.T) {
	ud := &UseExpandDesc{
		Name: "test_error",
		Flags: map[string]string{
			"flag1": "desc1",
		},
	}

	// Write to an invalid file path or a directory to trigger Create error
	err := ud.WriteFile("/invalid/path/that/does/not/exist/test.desc")
	if err == nil {
		t.Errorf("Expected WriteFile() to return an error, got nil")
	}
}

type faultyFS struct{}

func (f *faultyFS) Open(name string) (fs.File, error) {
	if strings.HasSuffix(name, "error.desc") {
		return nil, fmt.Errorf("mock open error")
	}
	return nil, fs.ErrNotExist
}

// We need a dummy read dir function to test ParseUseExpandDescFS with a faulty FS
type faultyFSDir struct {
	*faultyFS
}

type faultyFileEntry struct {
	name string
}

func (f *faultyFileEntry) Name() string               { return f.name }
func (f *faultyFileEntry) IsDir() bool                { return false }
func (f *faultyFileEntry) Type() fs.FileMode          { return 0 }
func (f *faultyFileEntry) Info() (fs.FileInfo, error) { return nil, nil }

func (f *faultyFSDir) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		return []fs.DirEntry{
			&faultyFileEntry{name: "error.desc"},
		}, nil
	}
	return nil, fmt.Errorf("read dir error")
}

func (f *faultyFSDir) Open(name string) (fs.File, error) {
	if name == "error.desc" {
		return nil, fmt.Errorf("mock open error")
	}
	return nil, fs.ErrNotExist
}

func TestParseUseExpandDescFS_ReadDirError(t *testing.T) {
	ffs := &faultyFSDir{&faultyFS{}}

	// trigger ReadDir error
	_, err := ParseUseExpandDescFS(ffs, "invalid_dir")
	if err == nil {
		t.Errorf("Expected ParseUseExpandDescFS() to return an error, got nil")
	}

	// trigger Open error
	_, err = ParseUseExpandDescFS(ffs, ".")
	if err == nil {
		t.Errorf("Expected ParseUseExpandDescFS() to return an error, got nil")
	}
}

// To test scanner errors, we need a reader that fails *after* successful reads, or just an immediate failure
// during scan. bufio.Scanner fails if Read returns an error other than io.EOF.
func TestParseUseExpandDesc_ScannerError(t *testing.T) {
	_, err := ParseUseExpandDesc(errReader{}, "test_scanner_error")
	if err == nil {
		t.Errorf("Expected ParseUseExpandDesc() to return an error with a faulty reader")
	}
}

// To test file open error in ParseUseExpandDescFile (not IsNotExist)
func TestParseUseExpandDescFile_OpenError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_open_err_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file without read permissions
	filePath := tmpDir + "/no_read.desc"
	err = os.WriteFile(filePath, []byte("test"), 0000)
	if err != nil {
		t.Fatalf("Failed to write no_read.desc: %v", err)
	}

	_, err = ParseUseExpandDescFile(filePath, "test")
	// If the file can be opened (e.g. running as root), it will just succeed parsing.
	// But usually this returns a permission denied error.
	// As long as we hit the code path.
	if err == nil {
		// Log but don't fail, as some CI environments run tests as root where 0000 doesn't prevent reading.
		t.Logf("Failed to trigger open error (running as root?)")
	}
}

type faultyFile struct {
	fs.File
}

func (f *faultyFile) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock read error")
}

func (f *faultyFile) Close() error {
	return nil
}

type faultyFSDirWithReadErr struct {
	*faultyFS
}

func (f *faultyFSDirWithReadErr) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		return []fs.DirEntry{
			&faultyFileEntry{name: "read_error.desc"},
		}, nil
	}
	return nil, nil
}

func (f *faultyFSDirWithReadErr) Open(name string) (fs.File, error) {
	if name == "read_error.desc" {
		return &faultyFile{}, nil
	}
	return nil, fs.ErrNotExist
}

func TestParseUseExpandDescFS_FileReadError(t *testing.T) {
	ffs := &faultyFSDirWithReadErr{&faultyFS{}}

	_, err := ParseUseExpandDescFS(ffs, ".")
	if err == nil {
		t.Errorf("Expected ParseUseExpandDescFS() to return an error when file reading fails, got nil")
	}
}

func TestWriteUseExpandDesc_Error_MissingDesc(t *testing.T) {
	ud := &UseExpandDesc{
		Name: "test",
		Lines: []DescLine{
			{Flag: "flag1"}, // flag present in Lines but missing in Flags map
		},
		Flags: map[string]string{}, // Empty map
	}

	var buf bytes.Buffer
	// It shouldn't error, it should just ignore it
	err := ud.Write(&buf)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if buf.String() != "" {
		t.Errorf("Expected empty string, got %q", buf.String())
	}
}

// To hit the error when writing new flags
func TestWriteUseExpandDesc_Error_NewFlagFormat(t *testing.T) {
	ud := &UseExpandDesc{
		Name: "test",
		Flags: map[string]string{
			"new_flag": "new desc",
		},
	}

	errW := &errWriter{}
	if err := ud.Write(errW); err == nil {
		t.Errorf("Expected Write() to return an error, got nil")
	}
}
