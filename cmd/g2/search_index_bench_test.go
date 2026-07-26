package main

import (
	"fmt"
	"testing"
)

func BenchmarkDeduplicateStrings(b *testing.B) {
	var input []string
	for i := 0; i < 1000; i++ {
		input = append(input, fmt.Sprintf("str-%d", i%100))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		deduplicateStrings(input)
	}
}
