package main

import (
	"strings"

	"github.com/arran4/g2"
)

// PageNode models a page in the generated site hierarchy.
//
// Invariant: Path is the node's path relative to its Parent (e.g. "packages" or
// "media-gfx"), with empty Path representing the same route as Parent. For
// backward compatibility, if a non-root node's Path already starts with
// Parent.CanonicalPath()+"/", CanonicalPath treats it as a site-relative path
// rather than double-counting ancestor segments.
type PageNode struct {
	Parent *PageNode
	Name   string
	Path   string
}

// CanonicalPath returns the normalized slash-separated path from the site root
// without leading or trailing slashes.
func (n *PageNode) CanonicalPath() string {
	if n == nil {
		return ""
	}
	p := strings.Trim(n.Path, "/")
	if n.Parent == nil {
		return p
	}
	parentCanon := n.Parent.CanonicalPath()
	if p == "" {
		return parentCanon
	}
	if parentCanon == "" {
		return p
	}
	if strings.HasPrefix(p, parentCanon+"/") {
		return p
	}
	return parentCanon + "/" + p
}

func pathSegmentDepth(canonPath string) int {
	if canonPath == "" {
		return 0
	}
	depth := 0
	for p := range strings.SplitSeq(canonPath, "/") {
		if p != "" {
			depth++
		}
	}
	return depth
}

func (n *PageNode) BaseURL() string {
	if n == nil || n.Parent == nil {
		return ""
	}
	return strings.Repeat("../", pathSegmentDepth(n.CanonicalPath()))
}

func (n *PageNode) Breadcrumbs() []g2.Breadcrumb {
	if n == nil {
		return nil
	}
	var nodes []*PageNode
	for curr := n; curr != nil; curr = curr.Parent {
		nodes = append(nodes, curr)
	}
	targetCanon := n.CanonicalPath()
	crumbs := make([]g2.Breadcrumb, 0, len(nodes))
	for i := len(nodes) - 1; i >= 0; i-- {
		curr := nodes[i]
		url := ""
		if curr != n {
			rel := pathRel(targetCanon, curr.CanonicalPath())
			if rel != "." {
				url = rel + "/"
			}
		}
		crumbs = append(crumbs, g2.Breadcrumb{Name: curr.Name, URL: url})
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
