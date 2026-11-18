package main

import (
	"github.com/arran4/g2"
	"strings"
)

type PageNode struct {
	Parent *PageNode
	Name   string
	Path   string
}

func pageNodeSegmentPath(parentCanonical, rawPath string) string {
	cleaned := strings.Trim(rawPath, "/")
	if cleaned == "" {
		return parentCanonical
	}
	if parentCanonical == "" {
		return cleaned
	}
	if cleaned == parentCanonical || strings.HasPrefix(cleaned, parentCanonical+"/") {
		return cleaned
	}
	return parentCanonical + "/" + cleaned
}

// CanonicalPath returns the normalized site-relative slash-separated path for n
// (for example "packages/app-misc/foo"), regardless of whether intermediate nodes
// stored single path segments or cumulative paths from root.
func (n *PageNode) CanonicalPath() string {
	if n == nil {
		return ""
	}
	var chain []*PageNode
	for curr := n; curr != nil; curr = curr.Parent {
		chain = append(chain, curr)
	}
	canonical := ""
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i].Parent == nil && strings.Trim(chain[i].Path, "/") == "" {
			continue
		}
		canonical = pageNodeSegmentPath(canonical, chain[i].Path)
	}
	return canonical
}

func (n *PageNode) BaseURL() string {
	if n == nil || n.Parent == nil {
		return ""
	}
	canonical := n.CanonicalPath()
	if canonical == "" {
		return ""
	}
	return strings.Repeat("../", strings.Count(canonical, "/")+1)
}

func (n *PageNode) Breadcrumbs() []g2.Breadcrumb {
	var chain []*PageNode
	for curr := n; curr != nil; curr = curr.Parent {
		chain = append(chain, curr)
	}
	if len(chain) == 0 {
		return nil
	}

	currentCanonical := n.CanonicalPath()
	crumbs := make([]g2.Breadcrumb, 0, len(chain))
	canonical := ""

	for i := len(chain) - 1; i >= 0; i-- {
		curr := chain[i]
		if curr.Parent != nil || strings.Trim(curr.Path, "/") != "" {
			canonical = pageNodeSegmentPath(canonical, curr.Path)
		}

		crumbURL := ""
		if curr != n {
			rel := pathRel(currentCanonical, canonical)
			if rel != "." {
				crumbURL = rel + "/"
			}
		}

		crumbs = append(crumbs, g2.Breadcrumb{
			Name: curr.Name,
			URL:  crumbURL,
		})
	}

	return crumbs
}

func (n *PageNode) Context(title string, version string, genInfo GenerationInfo) GenericPageContext {
	return GenericPageContext{
		Title:       title,
		BaseURL:     n.BaseURL(),
		Breadcrumbs: n.Breadcrumbs(),
		Version:     version,
		GenInfo:     genInfo,
	}
}
