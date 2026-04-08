package g2

import "time"

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
	AggEclasses       any // To break cycle, keep CLI specific type as any or redefine
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
	AggUseFlags       any // To break cycle
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

type ProfileDescEntry struct {
	Arch   string
	Path   string
	Status string
}

type ProfileData struct {
	Path     string
	IsDesc   bool
	DescArch string
	DescStat string
	Parents  []string
	Children []string
	Files    map[string]string // Maps filename to its content
}

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
	Name                  string
	Category              string
	Versions              []VersionData
	Metadata              *PkgMetadata
	MetadataError         error
	Manifest              *Manifest
	ManifestData          []ManifestEntryData
	Files                 []FileData
	HighestStableVersion  any
	HighestTestingVersion any
	EbuildCount           int
	DominantDescription   string
	DominantHomepage      string
	DominantLicense       string

	// Git info
	MetadataRawURL string
	ModTime        time.Time // Changed from time.Time to break cycle or keep as int64 if needed, wait time is standard

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

type EclassData struct {
	Name string
}

type PkgUseFlag struct {
	Name     string
	Desc     string
	Source   string
	Versions map[string]string // Version -> Unicode symbol representing state
}

type ManifestEntryData struct {
	Entry        *ManifestEntry
	Versions     []string
	URLs         []string
	ResolvedURLs []string
}
