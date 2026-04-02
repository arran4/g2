package main

import (
	"strings"
	"github.com/arran4/g2"
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

	res := ""
	for i := 0; i < depth; i++ {
		res += "../"
	}
	return res
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
			for i := 0; i < depthDiff; i++ {
				url += "../"
			}
		}

		crumbs = append([]g2.Breadcrumb{{Name: curr.Name, URL: url}}, crumbs...)
		curr = curr.Parent
	}

	return crumbs
}
