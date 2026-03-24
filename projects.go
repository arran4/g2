package g2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// Projects represents a list of projects.
// Source: https://github.com/gentoo-mirror/gentoo/blob/stable/metadata/dtd/projects.dtd
type Projects struct {
	XMLName  xml.Name  `xml:"projects"`
	Projects []Project `xml:"project"`
}

type Project struct {
	Email       string       `xml:"email"`
	Name        string       `xml:"name"`
	URL         string       `xml:"url"`
	Description string       `xml:"description"`
	Subprojects []Subproject `xml:"subproject"`
	Members     []Member     `xml:"member"`
}

type Subproject struct {
	Text           string `xml:",innerxml"`
	InheritMembers string `xml:"inherit-members,attr,omitempty"` // 0|1
	Ref            string `xml:"ref,attr"`
}

type Member struct {
	IsLead string `xml:"is-lead,attr,omitempty"` // 0|1
	Email  string `xml:"email"`
	Name   string `xml:"name"`
	Role   string `xml:"role"`
}

// ParseProjects parses a projects.xml file and returns a Projects pointer, or an error.
func ParseProjects(path string) (*Projects, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return ParseProjectsFromReader(f)
}

// ParseProjectsBytes parses Projects from a byte slice.
func ParseProjectsBytes(data []byte) (*Projects, error) {
	return ParseProjectsFromReader(bytes.NewReader(data))
}

// ParseProjectsFromReader parses Projects from an io.Reader.
func ParseProjectsFromReader(r io.Reader) (*Projects, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var projects Projects
	if err := xml.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	if projects.XMLName.Local != "projects" {
		return nil, fmt.Errorf("invalid XML: expected projects, got %s", projects.XMLName.Local)
	}

	return &projects, nil
}
