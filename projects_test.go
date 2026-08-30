package g2

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseProjectsBytes(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *Projects
		wantErr bool
	}{
		{
			name: "valid XML",
			data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<projects>
	<project>
		<email>test@example.com</email>
		<name>Test Project</name>
		<url>https://example.com/test</url>
		<description>A test project description.</description>
		<subproject inherit-members="1" ref="sub1">Subproject Text</subproject>
		<member is-lead="1">
			<email>lead@example.com</email>
			<name>Lead Dev</name>
			<role>Developer</role>
		</member>
	</project>
</projects>`),
			want: &Projects{
				XMLName: xml.Name{Local: "projects"},
				Projects: []Project{
					{
						Email:       "test@example.com",
						Name:        "Test Project",
						URL:         "https://example.com/test",
						Description: "A test project description.",
						Subprojects: []Subproject{
							{
								Text:           "Subproject Text",
								InheritMembers: "1",
								Ref:            "sub1",
							},
						},
						Members: []Member{
							{
								IsLead: "1",
								Email:  "lead@example.com",
								Name:   "Lead Dev",
								Role:   "Developer",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid XML",
			data:    []byte(`<projects><project>broken xml`),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid root tag",
			data:    []byte(`<wrongroot></wrongroot>`),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProjectsBytes(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProjectsBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				got.XMLName.Space = "" // ignore space comparison
			}
			if tt.want != nil {
				tt.want.XMLName.Space = ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseProjectsBytes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseProjects(t *testing.T) {
	tmpDir := t.TempDir()

	validXML := []byte(`<?xml version="1.0" encoding="UTF-8"?><projects></projects>`)
	validPath := filepath.Join(tmpDir, "valid.xml")
	if err := os.WriteFile(validPath, validXML, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("valid file", func(t *testing.T) {
		got, err := ParseProjects(validPath)
		if err != nil {
			t.Errorf("ParseProjects() unexpected error = %v", err)
		}
		if got == nil || got.XMLName.Local != "projects" {
			t.Errorf("ParseProjects() got = %v, want valid projects", got)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := ParseProjects(filepath.Join(tmpDir, "nonexistent.xml"))
		if err == nil {
			t.Error("ParseProjects() expected error for non-existent file, got nil")
		}
	})
}

var errTestRead = errors.New("test read error")

type projectErrorReader struct {
	err error
}

func (r projectErrorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

func TestParseProjectsFromReader(t *testing.T) {
	got, err := ParseProjectsFromReader(projectErrorReader{err: errTestRead})
	if !errors.Is(err, errTestRead) {
		t.Errorf("ParseProjectsFromReader() error = %v, wantErr %v", err, errTestRead)
	}
	if got != nil {
		t.Errorf("ParseProjectsFromReader() got = %v, want nil", got)
	}
}
