package g2

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}

func TestParseArchesDesc(t *testing.T) {
	tests := []struct {
		name        string
		input       io.Reader
		wantHeaders []string
		wantArches  map[string]string
		wantErr     bool
	}{
		{
			name: "basic",
			input: strings.NewReader(`# Copyright 1999-2024 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

# This file contains descriptions of architectures.

amd64           stable
x86             stable
~mips           testing
`),
			wantHeaders: []string{
				"# Copyright 1999-2024 Gentoo Authors",
				"# Distributed under the terms of the GNU General Public License v2",
				"",
				"# This file contains descriptions of architectures.",
				"",
			},
			wantArches: map[string]string{
				"amd64": "stable",
				"x86":   "stable",
				"~mips": "testing",
			},
			wantErr: false,
		},
		{
			name: "no header",
			input: strings.NewReader(`amd64 stable
x86 stable`),
			wantHeaders: []string{},
			wantArches: map[string]string{
				"amd64": "stable",
				"x86":   "stable",
			},
			wantErr: false,
		},
		{
			name: "empty",
			input: strings.NewReader(``),
			wantHeaders: []string{},
			wantArches:  map[string]string{},
			wantErr: false,
		},
		{
			name: "malformed arch line",
			input: strings.NewReader(`amd64
x86 stable
`),
			wantHeaders: []string{},
			wantArches: map[string]string{
				"x86": "stable",
			},
			wantErr: false,
		},
		{
			name: "trailing comments and blank lines",
			input: strings.NewReader(`# Header
amd64 stable

# This is a trailing comment
x86 stable
`),
			wantHeaders: []string{"# Header"},
			wantArches: map[string]string{
				"amd64": "stable",
				"x86":   "stable",
			},
			wantErr: false,
		},
		{
			name:    "read error",
			input:   &errorReader{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ad, err := ParseArchesDesc(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseArchesDesc() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if tt.wantHeaders == nil {
				tt.wantHeaders = []string{}
			}
			if len(ad.HeaderLines) == 0 && len(tt.wantHeaders) == 0 {
				// both empty, DeepEqual handles this but sometimes []string{} and nil are different
			} else if !reflect.DeepEqual(ad.HeaderLines, tt.wantHeaders) {
				t.Errorf("HeaderLines = %v, want %v", ad.HeaderLines, tt.wantHeaders)
			}

			if tt.wantArches == nil {
				tt.wantArches = map[string]string{}
			}
			if len(ad.Arches) == 0 && len(tt.wantArches) == 0 {
				// both empty
			} else if !reflect.DeepEqual(ad.Arches, tt.wantArches) {
				t.Errorf("Arches = %v, want %v", ad.Arches, tt.wantArches)
			}
		})
	}
}
