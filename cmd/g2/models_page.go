package main

import (
	"html/template"

	"github.com/arran4/g2"
)

type GenericPageContext struct {
	Title       string
	BaseURL     string
	Breadcrumbs []g2.Breadcrumb
	Version     string
	GenInfo     GenerationInfo
	Content     template.HTML

	// Additional data fields used by various templates
	Repos                []*g2.SiteData
	GroupedRepos         []g2.RepoGroup
	Categories           interface{} // Can be []*g2.AggCategory or []g2.CategoryData
	Packages             interface{} // Can be []*g2.AggPackage or []g2.PackageData
	Licenses             []*g2.AggLicense
	UseFlags             []*g2.AggUseFlag
	UseExpandDescs        map[string]*g2.UseExpandDesc
	UseExpandDesc         *g2.UseExpandDesc
	Projects             []*g2.AggProject
	Eclass               *g2.AggEclass
	Eclasses             []*g2.AggEclass
	Profiles             interface{} // Can be []*g2.AggProfile or []g2.ProfileData
	Arches               []*g2.AggArch
	RecentDurationString string
	RecentNews           interface{} // Can be []g2.AggNewsItem or []g2.NewsItem
	GlobalNews           []g2.AggNewsItem
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
	ProfileList          interface{} // Can be []g2.AggProfileRepo
	Profile              interface{}
	FileName             string
	FileContent          string
	Arch                 *g2.AggArch
	UseFlag              interface{}
	License              map[string]interface{}
	Project              *g2.AggProject
	Repo                 *g2.SiteData
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
	FilteredManifest     []g2.ManifestEntryData
	Manifest             interface{}
}
