package g2

import "encoding/xml"

type RemoteRepositories struct {
	XMLName xml.Name     `xml:"repositories"`
	Repos   []RemoteRepo `xml:"repo"`
}

type RemoteRepo struct {
	Name    string       `xml:"name"`
	Sources []RepoSource `xml:"source"`
}

type RepoSource struct {
	Type string `xml:"type,attr"`
	URL  string `xml:",chardata"`
}

type SiteData struct {
	Title      string
	RepoName   string
	RemoteURL  string
	Categories []CategoryData
}

type LicenseData struct {
	Name     string
	Count    int
	Packages []PackageData
}

type Breadcrumb struct {
	Name string
	URL  string
}

type CategoryData struct {
	Name     string
	Packages []PackageData
}

type FileData struct {
	Name   string
	Path   string
	RawURL string
}

type PackageData struct {
	Name          string
	Category      string
	Versions      []VersionData
	Metadata      *PkgMetadata
	MetadataError error
	Manifest      *Manifest
	Files         []FileData

	// Git info
	MetadataRawURL string

	// Lint Info
	LintWarnings []string
}

type VersionData struct {
	Version string
	Ebuild  *Ebuild

	// Git info
	EbuildRawURL string
}

// FeedItem is needed from site generation too
type FeedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
	Updated     string
}
