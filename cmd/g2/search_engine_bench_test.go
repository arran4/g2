package main

import (
	"testing"
)

func BenchmarkSearchEngine_MatchVersion(b *testing.B) {
	engine := NewSearchEngine()
	doc := SearchDocument{
		Version: "1.2.3-r1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.matchVersion(doc, ">1.0.0")
	}
}
