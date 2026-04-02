package g2

import (
	"io"
	"weak"
)

type SiteData struct {
	Title          string
	RepoName       string
	RemoteURL      string
	Categories     []CategoryData
	LicenseMapping map[string][]string
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

type FileContent struct {
	data     weak.Pointer[[]byte]
	generate func() (io.ReadCloser, error)
}

func (fc *FileContent) Data() ([]byte, error) {
	if ptr := fc.data.Value(); ptr != nil {
		return *ptr, nil
	}

	rc, err := fc.generate()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	fc.data = weak.Make(&b)
	return b, nil
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

	Masked *PackageMasked
	Deprecated *PackageDeprecated
}

type VersionData struct {
	Version string
	Ebuild  *Ebuild

	// Git info
	EbuildRawURL string

	// Deprecation
	Deprecated *PackageDeprecated

	// Masked
	Masked *PackageMasked
}
