package templates

import "embed"

//go:embed ebuild/*
var EbuildFiles embed.FS

//go:embed site/*
var SiteFiles embed.FS
