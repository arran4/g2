package g2

import (
	"bufio"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type URIEntry struct {
	URL      string
	Filename string
}

// ParseEbuildVariables extracts PN, PV, P from the ebuild filename.
func ParseEbuildVariables(filename string) map[string]string {
	basename := filepath.Base(filename)
	// Regex to capture PN and PV.
	// Matches things like:
	// ollama-bin-0.10.1.ebuild -> PN=ollama-bin, PV=0.10.1
	// g2-bin-0.0.2.ebuild -> PN=g2-bin, PV=0.0.2
	// complex-app-1.2.3_rc4-r1.ebuild

	// Go regexp doesn't support named groups in the same way as Python for easy extraction into map,
	// but we can use submatches.
	// Python regex: r'^(?P<pn>.+)-(?P<pv>\d+(\.\d+)*([a-z]|_p\d+|_rc\d+|_beta\d+|_alpha\d+)?(-r\d+)?)\.ebuild$'

	re := regexp.MustCompile(`^(.+)-(\d+(\.\d+)*([a-z]|_p\d+|_rc\d+|_beta\d+|_alpha\d+)?(-r\d+)?)\.ebuild$`)
	matches := re.FindStringSubmatch(basename)

	if matches == nil {
		return nil
	}

	pn := matches[1]
	pv := matches[2]
	p := fmt.Sprintf("%s-%s", pn, pv)

	return map[string]string{
		"P":  p,
		"PN": pn,
		"PV": pv,
	}
}

// ResolveVariables replaces ${VAR} and $VAR in the text with values from variables map.
func ResolveVariables(text string, variables map[string]string) string {
	for key, value := range variables {
		text = strings.ReplaceAll(text, fmt.Sprintf("${%s}", key), value)
		text = strings.ReplaceAll(text, fmt.Sprintf("$%s", key), value)
	}
	return text
}

// ExtractURIs parses the ebuild content and extracts SRC_URI entries.
func ExtractURIs(content string, variables map[string]string) ([]URIEntry, error) {
	// Remove comments
	var cleanLines []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}
		cleanLines = append(cleanLines, line)
	}
	cleanContent := strings.Join(cleanLines, "\n")

	// Find SRC_URI block
	// It might use " or '
	// Multiline handling in Go regex needs (?s) flag for . to match newline if we were using .
	// But here we are looking for quoted string.
	// The python regex was: r'SRC_URI\s*=\s*"([^"]*)"'

	// We need to be careful about multiline strings.

	var srcUriBody string

	reDouble := regexp.MustCompile(`SRC_URI\s*=\s*"([^"]*)"`)
	reSingle := regexp.MustCompile(`SRC_URI\s*=\s*'([^']*)'`)

	// Note: these regexes are greedy and might match too much if there are multiple quotes or escapes,
	// but standard ebuilds usually have one SRC_URI block.
	// However, `[^"]*` will stop at the next quote, so it handles multiline if the string is quoted across lines
	// AND the regex engine is running in dot-matches-newline mode (which is not relevant for [^"]).
	// Go's regexp supports `\s` matching newlines.

	// Issue: `.` in Go doesn't match newline by default, but `[^"]` does match newline.
	// So `[^"]*` matches newlines.

	match := reDouble.FindStringSubmatch(cleanContent)
	if match == nil {
		match = reSingle.FindStringSubmatch(cleanContent)
	}

	if match == nil {
		return nil, nil
	}

	srcUriBody = match[1]

	// Simple tokenizer
	tokens := strings.Fields(srcUriBody)

	var uris []URIEntry
	i := 0
	for i < len(tokens) {
		token := tokens[i]

		// Check if it looks like a URL
		if strings.Contains(token, "://") {
			url := token
			filename := filepath.Base(url)

			// Check for -> rename
			if i+2 < len(tokens) && tokens[i+1] == "->" {
				filename = tokens[i+2]
				i += 3 // skip url, ->, filename
			} else {
				i += 1 // skip url
			}

			// Resolve variables in both URL and filename
			url = ResolveVariables(url, variables)
			filename = ResolveVariables(filename, variables)

			uris = append(uris, URIEntry{URL: url, Filename: filename})
		} else {
			i += 1
		}
	}

	return uris, nil
}
