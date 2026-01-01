package g2

import (
	"reflect"
	"testing"
)

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
