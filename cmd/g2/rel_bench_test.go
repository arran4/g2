package main

import (
	"strings"
	"testing"
)

func BenchmarkRelToRootLoop(b *testing.B) {
	path := "a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p"
	count := strings.Count(path, "/")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		relToRoot := "../../"
		for j := 0; j < count; j++ {
			relToRoot += "../"
		}
	}
}

func BenchmarkRelToRootRepeat(b *testing.B) {
	path := "a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p"
	count := strings.Count(path, "/")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		relToRoot := "../../" + strings.Repeat("../", count)
		_ = relToRoot
	}
}
