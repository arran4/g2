package g2

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var (
	reRevisionMatchB = regexp.MustCompile(`-r(\d+)$`)
	reDigitMatchB    = regexp.MustCompile(`(\d+)`)
)

func padVersionTokensRegex(v string) string {
	v = reRevisionMatchB.ReplaceAllString(v, "+r$1")
	return reDigitMatchB.ReplaceAllStringFunc(v, func(s string) string {
		return fmt.Sprintf("%010s", s)
	})
}

func padVersionTokensBuilder(v string) string {
	replaceRIndex := -1
	for i := len(v) - 1; i >= 0; i-- {
		if v[i] >= '0' && v[i] <= '9' {
			continue
		}
		if v[i] == 'r' && i > 0 && v[i-1] == '-' {
			if i < len(v)-1 {
				replaceRIndex = i - 1
			}
		}
		break
	}

	var sb strings.Builder
	// Over-allocate slightly to avoid resizing during padding
	sb.Grow(len(v) + 30)

	for i := 0; i < len(v); {
		if i == replaceRIndex {
			sb.WriteString("+r")
			i += 2
			continue
		}

		if v[i] >= '0' && v[i] <= '9' {
			start := i
			for i < len(v) && v[i] >= '0' && v[i] <= '9' {
				i++
			}
			pad := 10 - (i - start)
			if pad > 0 {
				for j := 0; j < pad; j++ {
					sb.WriteByte('0')
				}
			}
			sb.WriteString(v[start:i])
		} else {
			sb.WriteByte(v[i])
			i++
		}
	}
	return sb.String()
}

func BenchmarkPadVersionTokensRegex(b *testing.B) {
	benchmarks := []struct {
		name string
		v    string
	}{
		{"WithRevision", "1.2.3-r4"},
		{"WithAlpha", "2.0_alpha1"},
		{"Complex", "3.0_beta2-r10"},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				padVersionTokensRegex(bm.v)
			}
		})
	}
}

func BenchmarkPadVersionTokensBuilder(b *testing.B) {
	benchmarks := []struct {
		name string
		v    string
	}{
		{"WithRevision", "1.2.3-r4"},
		{"WithAlpha", "2.0_alpha1"},
		{"Complex", "3.0_beta2-r10"},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				padVersionTokensBuilder(bm.v)
			}
		})
	}
}
