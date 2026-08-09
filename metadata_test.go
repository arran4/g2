package g2

import (
	"encoding/xml"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType interface{}
		wantErr  bool
	}{
		{
			name:     "PkgMetadata",
			path:     "testdata/pkg_metadata.xml",
			wantType: &PkgMetadata{},
		},
		{
			name:     "CatMetadata",
			path:     "testdata/cat_metadata.xml",
			wantType: &CatMetadata{},
		},
		{
			name:    "Invalid XML",
			path:    "testdata/invalid_metadata.xml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMetadata(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMetadata() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}

			if reflect.TypeOf(got) != reflect.TypeOf(tt.wantType) {
				t.Errorf("ParseMetadata() type = %T, want %T", got, tt.wantType)
			}

			// Circularity check
			// Marshal the result and parse it back.

			var data []byte
			switch v := got.(type) {
			case *PkgMetadata:
				data, err = xml.Marshal(v)
			case *CatMetadata:
				data, err = xml.Marshal(v)
			}
			if err != nil {
				t.Fatalf("Marshal error = %v", err)
			}

			// Parse again
			got2, err := ParseMetadataBytes(data)
			if err != nil {
				t.Fatalf("ParseMetadataBytes() error = %v", err)
			}

			// Compare types
			if reflect.TypeOf(got2) != reflect.TypeOf(got) {
				t.Errorf("Type mismatch after round trip: %T vs %T", got2, got)
			}
		})
	}
}

func TestCircularity(t *testing.T) {
	// Read original file
	path := "testdata/pkg_metadata.xml"
	_, err := os.ReadFile(path) // Read file just to verify it exists
	if err != nil {
		t.Fatal(err)
	}

	// Parse
	got, err := ParseMetadata(path)
	if err != nil {
		t.Fatalf("ParseMetadata() error = %v", err)
	}

	// Marshal
	var newBytes []byte
	switch v := got.(type) {
	case *PkgMetadata:
		newBytes, err = xml.MarshalIndent(v, "", "\t")
	case *CatMetadata:
		newBytes, err = xml.MarshalIndent(v, "", "\t")
	}
	if err != nil {
		t.Fatal(err)
	}

	// Parse again
	got2, err := ParseMetadataBytes(newBytes)
	if err != nil {
		t.Fatalf("ParseMetadataBytes() error = %v", err)
	}

	// Deep compare got and got2?
	// reflect.DeepEqual might fail if field ordering or unexported fields differ (none here).
	if !reflect.DeepEqual(got, got2) {
		t.Errorf("Structs changed after round trip.\nGot: %+v\nWant: %+v", got2, got)
	}
}

func TestParseMetadataFromReader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType interface{}
		wantErr  bool
	}{
		{
			name:     "PkgMetadata",
			input:    `<pkgmetadata></pkgmetadata>`,
			wantType: &PkgMetadata{},
		},
		{
			name:     "CatMetadata",
			input:    `<catmetadata></catmetadata>`,
			wantType: &CatMetadata{},
		},
		{
			name:    "Invalid XML",
			input:   `<pkgmetadata><invalid></pkgmetadata>`,
			wantErr: true,
		},
		{
			name:    "Unknown Metadata",
			input:   `<unknownmetadata></unknownmetadata>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMetadataFromReader(strings.NewReader(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMetadataFromReader() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseMetadataFromReader() error = %v, wantErr %v", err, tt.wantErr)
			}

			if reflect.TypeOf(got) != reflect.TypeOf(tt.wantType) {
				t.Errorf("ParseMetadataFromReader() type = %T, want %T", got, tt.wantType)
			}
		})
	}
}

type mdErrorReader struct{}

func (e *mdErrorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func TestParseMetadataFromReader_ReadError(t *testing.T) {
	_, err := ParseMetadataFromReader(&mdErrorReader{})
	if err == nil || err.Error() != "read error" {
		t.Errorf("Expected 'read error', got %v", err)
	}
}
