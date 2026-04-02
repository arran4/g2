package g2

import (
	"encoding/xml"
	"html/template"
	"time"
)

// TODO evaluate the following they should be redundant OR moved to `/`

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

type ProfileDescEntry struct {
	Arch   string
	Path   string
	Status string
}

type SourceURL string

type ProfileData struct {
	Path     string
	IsDesc   bool
	DescArch string
	DescStat string
	Parents  []string
	Children []string
	Files    map[string]string // Maps filename to its content
}

type EclassData struct {
	Name string
}

type SiteData struct {
	Title             string
	RepoName          string
	RemoteURL         string
	Repository        *Repository
	EAPI              string
	Projects          *Projects
	Categories        []CategoryData
	Profiles          []ProfileData
	DefinedEclasses   []EclassData
	AggEclasses       []*AggEclass
	Authors           []Author
	AuthorsURL        string
	Moves             []PackageMove
	SlotMoves         []PackageSlotMove
	News              []NewsItem
	LayoutConf        *LayoutConf
	LicenseMapping    map[string][]string
	QAPolicy          *QAPolicy
	UseDesc           *UseDesc
	UseLocalDesc      *UseLocalDesc
	UseExpandDescs    map[string]*UseExpandDesc
	ValidUseExpands   map[string]bool
	ArchList          *ArchList
	ArchesDesc        *ArchesDesc
	InfoPkgs          []InfoPkg
	Masked            []PackageMasked
	Deprecated        []PackageDeprecated
	ParsedEclasses    []*Ebuild
	Eclasses          []*Ebuild
	PackageCount      int
	AggUseFlags       []*AggUseFlag
	ThirdPartyMirrors map[string][]string
	InfoVars          []string
	GitSize           string
	CheckoutTime      string
	ProcessTime       string
	SourceURL         string
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

type ManifestEntryData struct {
	Entry        *ManifestEntry
	Versions     []string
	URLs         []string
	ResolvedURLs []string
}

type PackageData struct {
	Name                  string
	Category              string
	Versions              []VersionData
	Metadata              *PkgMetadata
	MetadataError         error
	Manifest              *Manifest
	ManifestData          []ManifestEntryData
	Files                 []FileData
	HighestStableVersion  template.HTML
	HighestTestingVersion template.HTML
	EbuildCount           int
	DominantDescription   string
	DominantHomepage      string
	DominantLicense       string

	// Git info
	MetadataRawURL string
	ModTime        time.Time

	// Processed Uses (per package)
	PkgUseFlags []PkgUseFlag

	// Lint Info
	LintWarnings []string

	// Deprecation
	Masked     *PackageMasked
	Deprecated *PackageDeprecated

	// InfoPkg matching
	IsInfoPkg bool

	ReverseVirtuals []string
	Equivalents     []string
	VirtualDeps     []string
}

type PkgUseFlag struct {
	Name     string
	Desc     string
	Source   string
	Versions map[string]string // Version -> Unicode symbol representing state
}

type VersionData struct {
	Version string
	Ebuild  *Ebuild

	// Git info
	EbuildRawURL string
	ModTime      time.Time

	// Deprecation
	Deprecated *PackageDeprecated
	Masked     *PackageMasked

	// Moves
	MovedToSlot string

	ResolvedDepsJSON string
	// Mirrors
	ApplicableMirrors map[string][]string
}

// TODO check model's should be redundant OR migrated to /

type AggCategory struct {
	Name     string
	Packages map[string]*AggPackage
}
type AggPackage struct {
	Name                string
	Category            string
	Repos               map[string]*SiteData
	DominantDescription string
	DominantHomepage    string
	DominantLicense     string
	ReverseVirtuals     []string
	VirtualDeps         []string
}
type AggProject struct {
	Project  *Project
	Packages []*AggPackage
}

type AggLicense struct {
	Name     string
	Count    int
	Packages []*AggPackage
	Text     string
	Aliases  []string
}

type AggUseFlag struct {
	Name          string
	Count         int
	GlobalDesc    string
	LocalDescs    map[string]string
	MetadataDescs map[string]string
	Packages      []*AggPackage
	Warnings      []string
}

// TODO migrate to / if it hasn't been done already check for differences

type AggProfileRepo struct {
	RepoName string
	Profile  ProfileData
}

type AggProfile struct {
	Path     string
	IsDesc   bool
	DescArch string
	DescStat string
	Repos    []AggProfileRepo
	Files    map[string]string // Maps filename to its content
}

type AggEclass struct {
	Name     string
	Repos    map[string]*SiteData
	Packages []*AggPackage
}

type AggPackageMove struct {
	Old string
	New string
}

type AggNewsItem struct {
	NewsItem
	RepoName string
}

type AggArch struct {
	Name   string
	Status string
	Repos  []*SiteData
}

type RepoGroup struct {
	Quality string
	Status  string
	Repos   []*SiteData
}

type AggregatedData struct {
	Categories      []*AggCategory
	Packages        []*AggPackage
	Licenses        []*AggLicense
	Projects        []*AggProject
	Profiles        []*AggProfile
	Arches          []*AggArch
	Moves           map[string]*AggPackageMove
	GlobalNews      []AggNewsItem
	RecentNews      []AggNewsItem
	TotalPackages   int
	UseFlags        []*AggUseFlag
	Eclasses        []*AggEclass
	UseExpandDescs  map[string]*UseExpandDesc
	ValidLicenses   map[string]bool
	ValidUseExpands map[string]bool
	GroupedRepos    []RepoGroup
}
