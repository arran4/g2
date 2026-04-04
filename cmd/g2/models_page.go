package main

import (
	"html/template"

	"github.com/arran4/g2"
)

type GenericPageContext struct {
	Title       string
	BaseURL     string
	Breadcrumbs []Breadcrumb
	Version     string
	GenInfo     GenerationInfo
	Content     template.HTML

	// Additional data fields used by various templates
	Repos                []*SiteData
	GroupedRepos         []RepoGroup
	GlobalCategories     []*AggCategory
	RepoCategories       []CategoryData
	GlobalPackages       []*AggPackage
	RepoPackages         []PackageData
	Licenses             []*AggLicense
	UseFlags             []*AggUseFlag
	UseExpandDescs       map[string]*g2.UseExpandDesc
	UseExpandDesc        *g2.UseExpandDesc
	Projects             []*AggProject
	Eclass               *AggEclass
	Eclasses             []*AggEclass
	GlobalProfiles       []*g2.AggProfile
	RepoProfiles         []g2.ProfileData
	Arches               []*AggArch
	RecentDurationString string
	RecentGlobalNews     []AggNewsItem
	RecentRepoNews       []g2.NewsItem
	GlobalNews           []AggNewsItem
	News                 []g2.NewsItem
	GlobalNewsItem       *AggNewsItem
	RepoNewsItem         *g2.NewsItem
	GlobalCategory       *AggCategory
	RepoCategory         *CategoryData
	OldName              string
	NewName              string
	NewURL               string
	GlobalPackage        *AggPackage
	RepoPackage          *PackageData
	MovedToName          string
	MovedToURL           string
	ProfilePath          string
	ProfileList          []g2.AggProfileRepo
	GlobalProfile        *g2.AggProfile
	RepoProfile          *g2.ProfileData
	FileName             string
	FileContent          string
	Arch                 *AggArch
	GlobalUseFlag        *AggUseFlag
	RepoUseFlag          *g2.UseFlag
	License              *AggLicense
	Project              *AggProject
	Repo                 *SiteData
	PackageCount         int
	GlobalCategoriesCount int
	GlobalPackagesCount   int
	GlobalLicensesCount   int
	GlobalProfilesCount   int
	Authors              []g2.Author
	ValidLicenses        map[string]bool
	RepoName             string
	Group                *RepoGroup
	VersionData          *VersionData
	FilteredManifest     []ManifestEntryData
	Manifest             *ManifestEntryData

	// Legacy generic interface overrides for TmplPkgs and map
	Category map[string]interface{}
	Package any
	Packages any
	UseFlag any
}
