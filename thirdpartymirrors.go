package g2

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"strings"
)

// ParseThirdPartyMirrors parses a thirdpartymirrors file from a path
func ParseThirdPartyMirrors(path string) (map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseThirdPartyMirrorsFromReader(f)
}

// ParseThirdPartyMirrorsFS parses a thirdpartymirrors file from an fs.FS
func ParseThirdPartyMirrorsFS(sysFS fs.FS, path string) (map[string][]string, error) {
	f, err := sysFS.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseThirdPartyMirrorsFromReader(f)
}

// ParseThirdPartyMirrorsFromReader parses a thirdpartymirrors file from an io.Reader
// It returns a map of mirror name to a list of mirror URLs.
func ParseThirdPartyMirrorsFromReader(r io.Reader) (map[string][]string, error) {
	mirrors := make(map[string][]string)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			urls := parts[1:]
			mirrors[name] = append(mirrors[name], urls...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mirrors, nil
}
