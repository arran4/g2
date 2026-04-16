package g2

import "unique"

// InternString deduplicates string allocations to save memory by using Go's unique package.
func InternString(s string) string {
	return unique.Make(s).Value()
}
