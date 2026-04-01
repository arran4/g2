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
	Categories           interface{} // Can be []*AggCategory or []CategoryData
	Packages             interface{} // Can be []*AggPackage or []PackageData
	Licenses             []*AggLicense
	UseFlags             []*AggUseFlag
	UseExpandDescs        map[string]*g2.UseExpandDesc
	UseExpandDesc         *g2.UseExpandDesc
	Projects             []*AggProject
	Profiles             interface{} // Can be []*AggProfile or []ProfileData
	Arches               []*AggArch
	RecentDurationString string
	RecentNews           interface{} // Can be []AggNewsItem or []g2.NewsItem
	GlobalNews           []AggNewsItem
	News                 []g2.NewsItem
	NewsItem             interface{}
	Category             map[string]interface{}
	OldName              string
	NewName              string
	NewURL               string
	Package              interface{}
	MovedToName          string
	MovedToURL           string
	ProfilePath          string
	ProfileList          interface{} // Can be []AggProfileRepo
	Profile              interface{}
	Arch                 *AggArch
	UseFlag              interface{}
	License              map[string]interface{}
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
	Group                interface{}
	VersionData          interface{}
	FilteredManifest     []ManifestEntryData
	Manifest             interface{}
}
