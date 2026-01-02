package g2

import (
	"bufio"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"sort"
)

type URIEntry struct {
	URL      string
	Filename string
}

type ParsingMode uint

const (
	ParseMetadataOnly ParsingMode = iota + 1 // Parse only filename-based metadata (PN, PV, etc.)
	ParseVariables                           // Parse variable definitions in the file
	ParseFull                                // Parse everything (e.g. SRC_URI)
)

func (m ParsingMode) String() string {
	switch m {
	case ParseMetadataOnly:
		return "ParseMetadataOnly"
	case ParseVariables:
		return "ParseVariables"
	case ParseFull:
		return "ParseFull"
	default:
		return "Unknown"
	}
}

type Ebuild struct {
	Path      string
	Vars      map[string]string
	SrcUri    []URIEntry
	Mode      ParsingMode
}

func (e *Ebuild) String() string {
	var sb strings.Builder

	// Reconstruct a valid-ish ebuild
	// Since we don't preserve the whole file, we reconstruct what we know.
	// We do NOT output PN/PV/P variables as they are implicit from filename usually,
	// but if we parsed them from filename, we don't need to write them back to file.

	// Write variables
	// Sort keys for deterministic output
	keys := make([]string, 0, len(e.Vars))
	for k := range e.Vars {
		// Skip P, PN, PV if they match what we parsed from metadata
		// But wait, if we are generating "valid" from parsing, maybe we should output them?
		// No, real ebuilds don't define PN/PV usually.
		if k == "P" || k == "PN" || k == "PV" {
			continue
		}
		// If we are printing full ebuild with SRC_URI, don't print SRC_URI variable
		if k == "SRC_URI" && e.Mode == ParseFull && len(e.SrcUri) > 0 {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=\"%s\"\n", k, e.Vars[k]))
	}

	if e.Mode == ParseFull && len(e.SrcUri) > 0 {
		sb.WriteString("SRC_URI=\"\n")
		for _, u := range e.SrcUri {
			line := u.URL
			base := filepath.Base(u.URL)
			if u.Filename != base && u.Filename != "" {
				line = fmt.Sprintf("%s -> %s", u.URL, u.Filename)
			}
			sb.WriteString(fmt.Sprintf("\t%s\n", line))
		}
		sb.WriteString("\"\n")
	}

	return sb.String()
}

// ParseEbuild parses an ebuild file with the specified mode.
func ParseEbuild(fsys fs.FS, path string, mode ParsingMode) (*Ebuild, error) {
	e := &Ebuild{
		Path: path,
		Vars: make(map[string]string),
		Mode: mode,
	}

	// Always parse metadata from filename
	vars := ParseEbuildVariables(path)
	for k, v := range vars {
		e.Vars[k] = v
	}

	if mode == ParseMetadataOnly {
		return e, nil
	}

	contentBytes, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}
	content := string(contentBytes)

	if mode >= ParseVariables {
		// Simple variable parsing
		// This extends ParseEbuildVariables logic
		sc := bufio.NewScanner(strings.NewReader(content))
		for sc.Scan() {
			line := sc.Text()
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			// Look for KEY="VAL" or KEY=VAL
			// Very basic parser
			if idx := strings.Index(line, "="); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])

				// Remove quotes if present
				if len(val) >= 2 && strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
					val = val[1 : len(val)-1]
				} else if len(val) >= 2 && strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
					val = val[1 : len(val)-1]
				} else if strings.Count(val, "\"")%2 != 0 || strings.Count(val, "'")%2 != 0 {
					// Ignore lines with unbalanced quotes (likely multi-line strings)
					continue
				}

				// Resolve variables in value if possible
				val = ResolveVariables(val, e.Vars)

				// Only add if key looks like a variable (uppercase, underscores)
				// Ebuild vars are typically UPPER_CASE.
				// But some local vars might be lower.
				// Let's accept things that look like identifiers.
				// Ebuild variable names are essentially shell variable names.
				// They must start with a letter or underscore, followed by letters, numbers, or underscores.
				isIdentifier := true
				if len(key) == 0 {
					isIdentifier = false
				} else {
					for i, r := range key {
						if i == 0 {
							if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
								isIdentifier = false
								break
							}
						} else {
							if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
								isIdentifier = false
								break
							}
						}
					}
				}

				if isIdentifier {
					e.Vars[key] = val
				}
			}
		}
	}

	if mode >= ParseFull {
		uris, _ := ExtractURIs(content, e.Vars)
		// Don't fail hard on URI extraction?
		// The user said "partial implementation".
		e.SrcUri = uris
	}

	return e, nil
}


// ParseEbuildVariables extracts PN, PV, P from the ebuild filename.
func ParseEbuildVariables(filename string) map[string]string {
	basename := filepath.Base(filename)
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
	// Simple resolution: multiple passes until no change or limit reached
	for i := 0; i < 5; i++ { // Limit recursion depth
		original := text
		for key, value := range variables {
			text = strings.ReplaceAll(text, fmt.Sprintf("${%s}", key), value)
			text = strings.ReplaceAll(text, fmt.Sprintf("$%s", key), value)
		}
		if text == original {
			break
		}
	}
	return text
}

var (
	reDouble = regexp.MustCompile(`SRC_URI\s*=\s*"([^"]*)"`)
	reSingle = regexp.MustCompile(`SRC_URI\s*=\s*'([^']*)'`)
)

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

	var srcUriBody string

	match := reDouble.FindStringSubmatch(cleanContent)
	if match == nil {
		match = reSingle.FindStringSubmatch(cleanContent)
	}

	if match == nil {
		return nil, nil
	}

	srcUriBody = match[1]

	tokens := strings.Fields(srcUriBody)

	var uris []URIEntry
	i := 0
	for i < len(tokens) {
		token := tokens[i]

		if strings.Contains(token, "://") {
			url := token
			filename := filepath.Base(url)

			if i+2 < len(tokens) && tokens[i+1] == "->" {
				filename = tokens[i+2]
				i += 3
			} else {
				i += 1
			}

			url = ResolveVariables(url, variables)
			filename = ResolveVariables(filename, variables)

			uris = append(uris, URIEntry{URL: url, Filename: filename})
		} else {
			i += 1
		}
	}

	return uris, nil
}
