package g2

type CacheMode string

const (
	CacheModeCI     CacheMode = "ci"
	CacheModeStrict CacheMode = "strict"
)

type CacheResultStatus int

const (
	CacheVerified CacheResultStatus = iota
	CacheDrift
	CacheSkipped
	CacheError
)

func (s CacheResultStatus) String() string {
	switch s {
	case CacheVerified:
		return "Verified"
	case CacheDrift:
		return "Drift"
	case CacheSkipped:
		return "Skipped"
	case CacheError:
		return "Error"
	default:
		return "Unknown"
	}
}

// CacheResult represents the outcome of a cache operation for a specific package.
type CacheResult struct {
	Status  CacheResultStatus
	Message string
	Error   error
	Path    string // The expected cache path
}

// CachePolicy defines the execution policy for cache operations.
type CachePolicy struct {
	Mode          CacheMode
	ExplicitRepos map[string]string // map from repo name to path
	ReposConfPath string            // Path to repos.conf for resolving masters
}

// NewCachePolicy creates a new CachePolicy with the specified mode.
func NewCachePolicy(mode CacheMode) *CachePolicy {
	if mode == "" {
		mode = CacheModeCI // Default to CI mode
	}
	return &CachePolicy{
		Mode:          mode,
		ExplicitRepos: make(map[string]string),
	}
}
