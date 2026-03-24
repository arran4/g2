package g2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// Mirrors represents a set of mirror groups.
// Source: https://github.com/gentoo-mirror/gentoo/blob/stable/metadata/dtd/mirrors.dtd
type Mirrors struct {
	XMLName      xml.Name      `xml:"mirrors"`
	MirrorGroups []MirrorGroup `xml:"mirrorgroup"`
}

type MirrorGroup struct {
	Region      string   `xml:"region,attr"`
	Country     string   `xml:"country,attr"`
	CountryName string   `xml:"countryname,attr,omitempty"`
	Mirrors     []Mirror `xml:"mirror"`
}

type Mirror struct {
	City        string      `xml:"city,attr,omitempty"`
	Coordinates string      `xml:"coordinates,attr,omitempty"`
	GentooBug   string      `xml:"gentoo-bug,attr,omitempty"`
	Name        string      `xml:"name"`
	URIs        []MirrorURI `xml:"uri"`
}

type MirrorURI struct {
	Text     string `xml:",innerxml"`
	IPv4     string `xml:"ipv4,attr,omitempty"`     // Y|y|N|n
	IPv6     string `xml:"ipv6,attr,omitempty"`     // Y|y|N|n
	Partial  string `xml:"partial,attr,omitempty"`  // Y|y|N|n
	Protocol string `xml:"protocol,attr,omitempty"` // http|ftp|rsync
}

// ParseMirrors parses a mirrors.xml file and returns a Mirrors pointer, or an error.
func ParseMirrors(path string) (*Mirrors, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return ParseMirrorsFromReader(f)
}

// ParseMirrorsBytes parses Mirrors from a byte slice.
func ParseMirrorsBytes(data []byte) (*Mirrors, error) {
	return ParseMirrorsFromReader(bytes.NewReader(data))
}

// ParseMirrorsFromReader parses Mirrors from an io.Reader.
func ParseMirrorsFromReader(r io.Reader) (*Mirrors, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var mirrors Mirrors
	if err := xml.Unmarshal(data, &mirrors); err != nil {
		return nil, err
	}
	if mirrors.XMLName.Local != "mirrors" {
		return nil, fmt.Errorf("invalid XML: expected mirrors, got %s", mirrors.XMLName.Local)
	}

	return &mirrors, nil
}
