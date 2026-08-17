package g2

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

var ebuildContent = `EAPI=8
# Generated via: some tool
DESCRIPTION="A test ebuild"
SLOT="0"
KEYWORDS="~amd64"
DEPEND="dev-libs/foo"
RDEPEND="${DEPEND}"
src_compile() {
	emake
}
`

func BenchmarkHashOld(b *testing.B) {
	contentBytes := []byte(strings.Repeat(ebuildContent, 100))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lines := strings.Split(string(contentBytes), "\n")
		var hashContent []string
		slot := "0"
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# Generated via:") {
				continue
			}
			hashContent = append(hashContent, line)

			if strings.HasPrefix(trimmed, "SLOT=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					slot = strings.Trim(parts[1], `"'`)
				}
			}
		}

		hasher := md5.New()
		hasher.Write([]byte(strings.Join(hashContent, "\n")))
		_ = hex.EncodeToString(hasher.Sum(nil))
		_ = slot
	}
}

func BenchmarkHashNew(b *testing.B) {
	contentBytes := []byte(strings.Repeat(ebuildContent, 100))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lines := strings.Split(string(contentBytes), "\n")
		hasher := md5.New()
		slot := "0"
		firstLine := true
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# Generated via:") {
				continue
			}
			if !firstLine {
				if _, err := io.WriteString(hasher, "\n"); err != nil {
					// Handle error
				}
			}
			if _, err := io.WriteString(hasher, line); err != nil {
				// Handle error
			}
			firstLine = false

			if strings.HasPrefix(trimmed, "SLOT=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					slot = strings.Trim(parts[1], `"'`)
				}
			}
		}
		_ = hex.EncodeToString(hasher.Sum(nil))
		_ = slot
	}
}
