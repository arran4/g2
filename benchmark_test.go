package g2

import (
	"bytes"
	"testing"
)

func BenchmarkParsePackageDeprecatedReader(b *testing.B) {
	input := []byte(packageDeprecatedTestInput)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parsePackageDeprecatedReader(bytes.NewReader(input))
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
