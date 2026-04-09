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

func (n *PageNode) BaseURL() string {
	if n == nil || n.Parent == nil {
		return ""
	}
	depth := 0
	curr := n
	for curr.Parent != nil {
		if curr.Path != "" {
			parts := strings.Split(curr.Path, "/")
			for _, p := range parts {
				if p != "" {
					depth++
				}
			}
		}
		curr = curr.Parent
	}

	return strings.Repeat("../", depth)
}

func (n *PageNode) Breadcrumbs() []g2.Breadcrumb {
	var crumbs []g2.Breadcrumb
	curr := n

	for curr != nil {
		url := ""
		if curr != n {
			depthDiff := 0
			temp := n
			for temp != curr {
				if temp.Path != "" {
					parts := strings.Split(temp.Path, "/")
					for _, p := range parts {
						if p != "" {
							depthDiff++
						}
					}
				}
				temp = temp.Parent
			}
			url = strings.Repeat("../", depthDiff)
		}

		crumbs = append([]g2.Breadcrumb{{Name: curr.Name, URL: url}}, crumbs...)
		curr = curr.Parent
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
