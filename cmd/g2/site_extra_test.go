package main

import (
	"github.com/arran4/g2"
	"testing"
)

func TestResolveDependencies(t *testing.T) {
	site := &SiteData{
		Categories: []CategoryData{
			{
				Name: "app-misc",
				Packages: []PackageData{
					{
						Category: "app-misc",
						Name:     "bar",
					},
				},
			},
		},
	}

	depTree := g2.ParseDepTree("test? ( app-misc/bar app-misc/baz )")

	nodes := make([]ResolvedDepNode, 0)
	for _, n := range depTree.Nodes {
		nodes = append(nodes, resolveDependencies(n, site))
	}

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	condNode := nodes[0]
	if condNode.Type != "use_conditional" || condNode.Flag != "test" {
		t.Fatalf("expected use_conditional test, got %+v", condNode)
	}

	if len(condNode.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(condNode.Children))
	}

	c1 := condNode.Children[0]
	if c1.Name != "app-misc/bar" || c1.Link == "" {
		t.Errorf("expected app-misc/bar with link, got %+v", c1)
	}

	c2 := condNode.Children[1]
	if c2.Name != "app-misc/baz" || c2.Link != "" {
		t.Errorf("expected app-misc/baz without link, got %+v", c2)
	}
}
