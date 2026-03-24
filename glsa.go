package g2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// GLSA represents a Gentoo Linux Security Advisory.
// Source: https://github.com/gentoo-mirror/gentoo/blob/stable/metadata/dtd/glsa.dtd
type GLSA struct {
	XMLName     xml.Name       `xml:"glsa"`
	ID          string         `xml:"id,attr"`
	Title       string         `xml:"title"`
	Synopsis    string         `xml:"synopsis"`
	Product     Product        `xml:"product"`
	Announced   string         `xml:"announced"`
	Revised     Revised        `xml:"revised"`
	Bugs        []string       `xml:"bug"`
	Access      string         `xml:"access,omitempty"`
	Affected    Affected       `xml:"affected"`
	Background  *Background    `xml:"background"`
	Description Description    `xml:"description"`
	Impact      Impact         `xml:"impact"`
	Workaround  Workaround     `xml:"workaround"`
	Resolution  Resolution     `xml:"resolution"`
	References  References     `xml:"references"`
	License     *License       `xml:"license"`
	Metadata    []GLSAMetadata `xml:"metadata"`
}

type Product struct {
	Text string `xml:",innerxml"`
	Type string `xml:"type,attr"` // ebuild|infrastructure|informational
}

type Revised struct {
	Text  string `xml:",innerxml"`
	Count string `xml:"count,attr"`
}

type Affected struct {
	Packages []Package `xml:"package"`
	Services []Service `xml:"service"`
}

type Package struct {
	Name       string       `xml:"name,attr"`
	Auto       string       `xml:"auto,attr"` // yes|no
	Arch       string       `xml:"arch,attr"`
	Vulnerable []Vulnerable `xml:"vulnerable"`
	Unaffected []Unaffected `xml:"unaffected"`
}

type Vulnerable struct {
	Text  string `xml:",innerxml"`
	Range string `xml:"range,attr"`
	Slot  string `xml:"slot,attr"`
}

type Unaffected struct {
	Text  string `xml:",innerxml"`
	Range string `xml:"range,attr"`
	Slot  string `xml:"slot,attr"`
	Name  string `xml:"name,attr,omitempty"`
}

type Service struct {
	Text  string `xml:",innerxml"`
	Type  string `xml:"type,attr"`            // rsync|web|mirror
	Fixed string `xml:"fixed,attr,omitempty"` // yes|no
}

type Background struct {
	Text string `xml:",innerxml"`
}

type GLSADescription struct {
	Text string `xml:",innerxml"`
}

type Impact struct {
	Text string `xml:",innerxml"`
	Type string `xml:"type,attr"`
}

type Workaround struct {
	Text string `xml:",innerxml"`
}

type Resolution struct {
	Text string `xml:",innerxml"`
}

type References struct {
	URIs []URI `xml:"uri"`
}

type URI struct {
	Text string `xml:",innerxml"`
	Link string `xml:"link,attr,omitempty"`
}

type License struct {
	Text string `xml:",innerxml"`
}

type GLSAMetadata struct {
	Text      string `xml:",innerxml"`
	Tag       string `xml:"tag,attr"`
	Revision  string `xml:"revision,attr,omitempty"`
	Author    string `xml:"author,attr,omitempty"`
	Timestamp string `xml:"timestamp,attr,omitempty"`
}

// ParseGLSA parses a glsa.xml file and returns a GLSA pointer, or an error.
func ParseGLSA(path string) (*GLSA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return ParseGLSAFromReader(f)
}

// ParseGLSABytes parses GLSA from a byte slice.
func ParseGLSABytes(data []byte) (*GLSA, error) {
	return ParseGLSAFromReader(bytes.NewReader(data))
}

// ParseGLSAFromReader parses GLSA from an io.Reader.
func ParseGLSAFromReader(r io.Reader) (*GLSA, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var glsa GLSA
	if err := xml.Unmarshal(data, &glsa); err != nil {
		return nil, err
	}
	if glsa.XMLName.Local != "glsa" {
		return nil, fmt.Errorf("invalid XML: expected glsa, got %s", glsa.XMLName.Local)
	}

	return &glsa, nil
}
