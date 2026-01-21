package g2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// CatMetadata represents metadata for a category.
type CatMetadata struct {
	XMLName         xml.Name          `xml:"catmetadata"`
	LongDescription []LongDescription `xml:"longdescription"`
}

func (c *CatMetadata) String() string {
	b, err := xml.MarshalIndent(c, "", "\t")
	if err != nil {
		return ""
	}
	return string(b)
}

// PkgMetadata represents metadata for a package.
type PkgMetadata struct {
	XMLName            xml.Name          `xml:"pkgmetadata"`
	Maintainers        []Maintainer      `xml:"maintainer"`
	LongDescription    []LongDescription `xml:"longdescription"`
	Slots              *Slots            `xml:"slots"`
	StabilizeAllArches bool              `xml:"stabilize-allarches,omitempty"`
	Use                *Use              `xml:"use"`
	Upstream           *Upstream         `xml:"upstream"`
}

func (p *PkgMetadata) String() string {
	b, err := xml.MarshalIndent(p, "", "\t")
	if err != nil {
		return ""
	}
	return string(b)
}

// Maintainer represents a maintainer of a package.
type Maintainer struct {
	// Email of the maintainer.
	Email string `xml:"email"`
	// Name of the maintainer.
	Name string `xml:"name,omitempty"`
	// Description of the maintainer.
	Description string `xml:"description,omitempty"`
	// Type of the maintainer: person, project, or unknown.
	Type string `xml:"type,attr,omitempty"`
	// Proxied indicates whether the maintainer is proxied: yes, no, or proxy.
	Proxied string `xml:"proxied,attr,omitempty"`
	// Status of the upstream maintainer: active, inactive, or unknown.
	Status string `xml:"status,attr,omitempty"`
	// Restrict specifies restrictions on the applicability of the maintainer.
	Restrict string `xml:"restrict,attr,omitempty"`
}

// LongDescription represents a long description of a package or category.
type LongDescription struct {
	// Body contains the inner XML of the long description.
	// We use InnerXML to preserve mixed content (text + <pkg>/<cat> tags) and ensure circularity.
	Body    string `xml:",innerxml"`
	// Lang specifies the language of the description.
	Lang string `xml:"lang,attr,omitempty"`
	// Restrict specifies restrictions on the applicability of the description.
	Restrict string `xml:"restrict,attr,omitempty"`
}

// Slots represents the description of a package's SLOTs.
type Slots struct {
	// Slot elements.
	Slot []Slot `xml:"slot"`
	// Subslots of the package.
	Subslots string `xml:"subslots,omitempty"`
	// Lang specifies the language of the slots.
	Lang string `xml:"lang,attr,omitempty"`
}

// Slot represents a particular SLOT.
type Slot struct {
	// Name of the SLOT.
	Name string `xml:"name,attr"`
	// Text of the slot.
	Text string `xml:",chardata"`
}

// Use represents the description of USE flags for a package.
type Use struct {
	// Flags representing the USE flags.
	Flags []Flag `xml:"flag"`
	// Lang specifies the language of the USE description.
	Lang string `xml:"lang,attr,omitempty"`
}

// Flag represents a USE flag.
type Flag struct {
	// Name of the USE flag.
	Name string `xml:"name,attr"`
	// Text of the flag.
	Text string `xml:",chardata"`
	// Restrict specifies restrictions on the applicability of the flag.
	Restrict string `xml:"restrict,attr,omitempty"`
}

// Upstream represents upstream metadata information.
type Upstream struct {
	// Maintainers of the upstream.
	Maintainers []Maintainer `xml:"maintainer"`
	// URL of the upstream changelog.
	Changelog string `xml:"changelog,omitempty"`
	// Documentation URL of the upstream.
	Doc []Doc `xml:"doc"`
	// URL or email address to report bugs.
	BugsTo string `xml:"bugs-to,omitempty"`
	// Remote identifiers for the package.
	RemoteID []RemoteID `xml:"remote-id"`
}

// Doc represents a documentation URL.
type Doc struct {
	// URL of the documentation.
	URL string `xml:",chardata"`
	// Lang specifies the language of the documentation.
	Lang string `xml:"lang,attr,omitempty"`
}

// RemoteID represents a remote identifier for a package.
type RemoteID struct {
	// Text of the remote identifier.
	Text string `xml:",chardata"`
	// Type of the remote identifier.
	Type string `xml:"type,attr"`
}

// Pkg represents a cross-linking package.
// Note: This struct is not directly used in LongDescription to preserve mixed content via InnerXML,
// but is provided as part of the model definition.
type Pkg struct {
	// Text of the package.
	Text string `xml:",chardata"`
}

// Cat represents a cross-linking category.
// Note: This struct is not directly used in LongDescription to preserve mixed content via InnerXML,
// but is provided as part of the model definition.
type Cat struct {
	// Text of the category.
	Text string `xml:",chardata"`
}

// Email represents an email address.
type Email struct {
	// Text of the email address.
	Text string `xml:",chardata"`
}

// Name represents a name of a person or maintainer.
type Name struct {
	// Text of the name.
	Text string `xml:",chardata"`
}

// Description represents a description of a maintainer or change.
type Description struct {
	// Text of the description.
	Text string `xml:",chardata"`
}

// ParseMetadata parses a metadata.xml file and returns either a PkgMetadata or CatMetadata pointer (as interface{}), or an error.
func ParseMetadata(path string) (interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ParseMetadataFromReader(f)
}

// ParseMetadataBytes parses metadata from a byte slice.
func ParseMetadataBytes(data []byte) (interface{}, error) {
	return ParseMetadataFromReader(bytes.NewReader(data))
}

// ParseMetadataFromReader parses metadata from an io.Reader.
func ParseMetadataFromReader(r io.Reader) (interface{}, error) {
	// We need to read the whole thing to peek or just try to unmarshal.
	// Since we don't know if it is pkgmetadata or catmetadata, we can decode into a struct that captures the root name.

	// Or we can just read it all into memory (it's small) and unmarshal.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Try PkgMetadata
	var pkgMd PkgMetadata
	if err := xml.Unmarshal(data, &pkgMd); err == nil && pkgMd.XMLName.Local == "pkgmetadata" {
		return &pkgMd, nil
	}

	// Try CatMetadata
	var catMd CatMetadata
	if err := xml.Unmarshal(data, &catMd); err == nil && catMd.XMLName.Local == "catmetadata" {
		return &catMd, nil
	}

	return nil, fmt.Errorf("unknown metadata type or invalid XML")
}

// WriteMetadata writes the metadata to a file with proper headers.
func WriteMetadata(path string, data interface{}) error {
	var output []byte
	output = append(output, []byte(xml.Header)...)

	var docType string
	switch data.(type) {
	case *PkgMetadata:
		docType = "<!DOCTYPE pkgmetadata SYSTEM \"http://www.gentoo.org/dtd/metadata.dtd\">\n"
	case *CatMetadata:
		docType = "<!DOCTYPE catmetadata SYSTEM \"http://www.gentoo.org/dtd/metadata.dtd\">\n"
	default:
		return fmt.Errorf("unknown metadata type")
	}
	output = append(output, []byte(docType)...)

	b, err := xml.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	output = append(output, b...)
	output = append(output, '\n')

	return os.WriteFile(path, output, 0644)
}
