package main

import (
	"testing"
)

func BenchmarkPageNodeBaseURL(b *testing.B) {
	node := &PageNode{
		Name: "test",
		Path: "a/b/c/d/e",
		Parent: &PageNode{
			Name: "parent",
			Path: "1/2/3/4/5",
			Parent: &PageNode{
				Name: "root",
				Path: "",
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = node.BaseURL()
	}
}

func BenchmarkPageNodeBreadcrumbs(b *testing.B) {
	node := &PageNode{
		Name: "test",
		Path: "a/b/c/d/e",
		Parent: &PageNode{
			Name: "parent",
			Path: "1/2/3/4/5",
			Parent: &PageNode{
				Name: "root",
				Path: "",
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = node.Breadcrumbs()
	}
}
