package g2

import (
	"embed"
	"reflect"
	"testing"
	"testing/fstest"
)

//go:embed testdata
var testData embed.FS

func TestParseEbuildVariables(t *testing.T) {
	tests := []struct {
		filename string
		want     map[string]string
	}{
		{
			filename: "ollama-bin-0.10.1.ebuild",
			want: map[string]string{
				"P":  "ollama-bin-0.10.1",
				"PN": "ollama-bin",
				"PV": "0.10.1",
			},
		},
		{
			filename: "g2-bin-0.0.2.ebuild",
			want: map[string]string{
				"P":  "g2-bin-0.0.2",
				"PN": "g2-bin",
				"PV": "0.0.2",
			},
		},
		{
			filename: "app-1.2.3_rc4-r1.ebuild",
			want: map[string]string{
				"P":  "app-1.2.3_rc4-r1",
				"PN": "app",
				"PV": "1.2.3_rc4-r1",
			},
		},
		{
			filename: "invalid.txt",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := ParseEbuildVariables(tt.filename); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseEbuildVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractURIs(t *testing.T) {
	content := `
# Copyright 2023
EAPI=8

SRC_URI="
    https://example.com/files/${P}.tar.gz
    https://example.com/other/file.bin -> renamed.bin
"
`
	variables := map[string]string{
		"P": "mypackage-1.0",
	}

	want := []URIEntry{
		{URL: "https://example.com/files/mypackage-1.0.tar.gz", Filename: "mypackage-1.0.tar.gz"},
		{URL: "https://example.com/other/file.bin", Filename: "renamed.bin"},
	}

	got, err := ExtractURIs(content, variables)
	if err != nil {
		t.Fatalf("ExtractURIs error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractURIs() = %v, want %v", got, want)
	}
}

func TestExtractURIsSingleQuote(t *testing.T) {
	content := `SRC_URI='https://example.com/file.tar.gz'`
	variables := map[string]string{}

	want := []URIEntry{
		{URL: "https://example.com/file.tar.gz", Filename: "file.tar.gz"},
	}

	got, err := ExtractURIs(content, variables)
	if err != nil {
		t.Fatalf("ExtractURIs error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractURIs() = %v, want %v", got, want)
	}
}

func TestParseEbuild_MetadataOnly(t *testing.T) {
	path := "testdata/basic-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseMetadataOnly)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	if ebuild.Vars["PN"] != "basic" {
		t.Errorf("Expected PN=basic, got %s", ebuild.Vars["PN"])
	}
	if ebuild.Vars["PV"] != "1.0" {
		t.Errorf("Expected PV=1.0, got %s", ebuild.Vars["PV"])
	}
	// Content variables shouldn't be parsed
	if _, ok := ebuild.Vars["DESCRIPTION"]; ok {
		t.Error("Did not expect DESCRIPTION to be parsed in MetadataOnly mode")
	}
}

func TestParseEbuild_Variables(t *testing.T) {
	path := "testdata/vars-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseVariables)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	if ebuild.Vars["MY_PN"] != "mypackage" {
		t.Errorf("Expected MY_PN=mypackage, got %s", ebuild.Vars["MY_PN"])
	}
	// Check resolution
	if ebuild.Vars["S"] != "${WORKDIR}/mypackage-1.0" && ebuild.Vars["S"] != "/mypackage-1.0" {
		// Note: WORKDIR is not defined in the ebuild, so it remains as ${WORKDIR} or empty depending on resolver logic
		// My simple resolver leaves ${WORKDIR} if not found in map? No, wait.
		// ResolveVariables implementation:
		// text = strings.ReplaceAll(text, fmt.Sprintf("${%s}", key), value)
		// It iterates over keys in map. If WORKDIR is not in map, it won't be replaced.
		// So it should be "${WORKDIR}/mypackage-1.0"
		if ebuild.Vars["S"] != "${WORKDIR}/mypackage-1.0" {
			t.Errorf("Expected S=${WORKDIR}/mypackage-1.0, got %s", ebuild.Vars["S"])
		}
	}
}

func TestParseEbuild_Full(t *testing.T) {
	path := "testdata/vars-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseFull)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	if len(ebuild.SrcUri) != 1 {
		t.Fatalf("Expected 1 URI, got %d", len(ebuild.SrcUri))
	}

	// SRC_URI="https://example.com/${MY_PN}-${MY_PV}.tar.gz -> ${P}.tar.gz"
	// P=vars-1.0
	// MY_PN=mypackage
	// MY_PV=1.0
	expectedUrl := "https://example.com/mypackage-1.0.tar.gz"
	expectedFile := "vars-1.0.tar.gz"

	if ebuild.SrcUri[0].URL != expectedUrl {
		t.Errorf("Expected URL=%s, got %s", expectedUrl, ebuild.SrcUri[0].URL)
	}
	if ebuild.SrcUri[0].Filename != expectedFile {
		t.Errorf("Expected Filename=%s, got %s", expectedFile, ebuild.SrcUri[0].Filename)
	}
}

func TestParseEbuild_Circular(t *testing.T) {
	// 1. Parse an ebuild
	path := "testdata/vars-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseFull)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	// 2. Generate string
	generated := ebuild.String()

	// 3. Parse generated string as a new ebuild
	// We need to fake the file presence using fstest.MapFS or similar,
	// because ParseEbuild expects to read from FS.
	// We'll reuse the filename "vars-1.0.ebuild" so we get same P/PN/PV.

	memFS := fstest.MapFS{
		"vars-1.0.ebuild": &fstest.MapFile{
			Data: []byte(generated),
		},
	}

	ebuild2, err := ParseEbuild(memFS, "vars-1.0.ebuild", ParseFull)
	if err != nil {
		t.Fatalf("ParseEbuild (round 2) failed: %v", err)
	}

	// 4. Compare key attributes
	if ebuild2.Vars["MY_PN"] != ebuild.Vars["MY_PN"] {
		t.Errorf("Circular mismatch MY_PN: %s vs %s", ebuild2.Vars["MY_PN"], ebuild.Vars["MY_PN"])
	}

	// SRC_URI parsing might fail if generated output doesn't match the regex exactly.
	// My String() implementation generates:
	// SRC_URI="
	//     url -> filename
	// "
	// My ExtractURIs implementation expects:
	// SRC_URI="..."
	// multiline with tokens.
	// It should work.

	if len(ebuild2.SrcUri) != len(ebuild.SrcUri) {
		t.Errorf("Circular mismatch URI count: %d vs %d", len(ebuild2.SrcUri), len(ebuild.SrcUri))
	} else if len(ebuild.SrcUri) > 0 {
		if ebuild2.SrcUri[0].URL != ebuild.SrcUri[0].URL {
			t.Errorf("Circular mismatch URL: %s vs %s", ebuild2.SrcUri[0].URL, ebuild.SrcUri[0].URL)
		}
	}
}

// TestBlackbox verifies that we can interact with Ebuild struct publicly
func TestBlackbox(t *testing.T) {
	// This test just ensures public fields are accessible
	e := &Ebuild{
		Path: "test-1.0.ebuild",
		Vars: make(map[string]string),
	}
	e.Vars["KEY"] = "VAL"
	if e.Path != "test-1.0.ebuild" {
		t.Error("Public Path field not accessible/settable")
	}
}
