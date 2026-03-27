package g2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// Repositories represents a list of Gentoo repositories.
// Source: https://github.com/gentoo-mirror/gentoo/blob/stable/metadata/dtd/repositories.dtd
type Repositories struct {
	XMLName      xml.Name     `xml:"repositories"`
	Version      string       `xml:"version,attr"`
	Repositories []Repository `xml:"repo"`
}

type Repository struct {
	Priority         string                  `xml:"priority,attr,omitempty"`
	Quality          string                  `xml:"quality,attr"` // core|stable|testing|experimental|graveyard
	Status           string                  `xml:"status,attr"`  // official|unofficial
	Name             string                  `xml:"name"`
	Descriptions     []RepositoryDescription `xml:"description"`
	LongDescriptions []RepositoryDescription `xml:"longdescription"`
	Homepage         string                  `xml:"homepage,omitempty"`
	Owners           []RepositoryOwner       `xml:"owner"`
	Sources          []RepositorySource      `xml:"source"`
	Feeds            []string                `xml:"feed"`
}

type RepositoryDescription struct {
	Text string `xml:",innerxml"`
	Lang string `xml:"lang,attr,omitempty"`
}

type RepositoryOwner struct {
	Type  string `xml:"type,attr,omitempty"` // project|person
	Email string `xml:"email"`
	Name  string `xml:"name,omitempty"`
}

type RepositorySource struct {
	Text string `xml:",innerxml"`
	Type string `xml:"type,attr"` // bzr|cvs|darcs|git|mercurial|rsync|svn|tar
}

// ParseRepositories parses a repositories.xml file and returns a Repositories pointer, or an error.
func ParseRepositories(path string) (*Repositories, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return ParseRepositoriesFromReader(f)
}

// ParseRepositoriesBytes parses Repositories from a byte slice.
func ParseRepositoriesBytes(data []byte) (*Repositories, error) {
	return ParseRepositoriesFromReader(bytes.NewReader(data))
}

// ParseRepositoriesFromReader parses Repositories from an io.Reader.
func ParseRepositoriesFromReader(r io.Reader) (*Repositories, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var repositories Repositories
	if err := xml.Unmarshal(data, &repositories); err != nil {
		return nil, err
	}
	if repositories.XMLName.Local != "repositories" {
		return nil, fmt.Errorf("invalid XML: expected repositories, got %s", repositories.XMLName.Local)
	}

	return &repositories, nil
}

// RemoteRepositories models a list of overlay repositories.
