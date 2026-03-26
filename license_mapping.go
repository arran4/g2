package g2

import (
	"bufio"
	"io"
	"strings"
)

// ParseLicenseMapping parses a metadata/license-mapping.conf file.
// It returns a map where the key is the target Gentoo license, and the value
// is a list of SPDX license aliases that map to it.
func ParseLicenseMapping(r io.Reader) (map[string][]string, error) {
	scanner := bufio.NewScanner(r)
	mapping := make(map[string][]string)

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, "[") {
			continue
		}

		parts := strings.SplitN(trimmedLine, "=", 2)
		if len(parts) == 2 {
			alias := strings.TrimSpace(parts[0])
			target := strings.TrimSpace(parts[1])
			if alias != "" && target != "" {
				mapping[target] = append(mapping[target], alias)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mapping, nil
}
