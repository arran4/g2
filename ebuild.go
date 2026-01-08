package g2

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
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
	Path   string
	Vars   map[string]string
	SrcUri []URIEntry
	Mode   ParsingMode
}

const ebuildTemplate = `{{- range .Vars -}}
{{ .Key }}="{{ .Value }}"
{{ end -}}
{{- if .SrcUri -}}
SRC_URI="
{{- range .SrcUri }}
	{{ .URL }}{{ if .Filename }} -> {{ .Filename }}{{ end }}
{{- end }}
"
{{ end -}}
`

type varEntry struct {
	Key   string
	Value string
}

type ebuildData struct {
	Vars   []varEntry
	SrcUri []URIEntry
}

func (e *Ebuild) String() string {
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

	var entries []varEntry
	for _, k := range keys {
		entries = append(entries, varEntry{Key: k, Value: e.Vars[k]})
	}

	data := ebuildData{
		Vars: entries,
	}

	if e.Mode == ParseFull && len(e.SrcUri) > 0 {
		// Populate SrcUri entries with filename logic
		for i, u := range e.SrcUri {
			base := filepath.Base(u.URL)
			if u.Filename == base || u.Filename == "" {
				// Don't duplicate if filename matches base
				// But wait, template expects filename to be empty if we want to skip "->".
				// Copy struct to avoid modifying original?
				// Actually, if filename matches, we should set it to empty in the copy for template.
				newU := u
				newU.Filename = ""
				if len(data.SrcUri) == 0 {
					data.SrcUri = make([]URIEntry, len(e.SrcUri))
				}
				data.SrcUri[i] = newU
			} else {
				if len(data.SrcUri) == 0 {
					data.SrcUri = make([]URIEntry, len(e.SrcUri))
				}
				data.SrcUri[i] = u
			}
		}
		// Wait, loop above initializes slice only once? No.
		// Let's rewrite cleaner.
		data.SrcUri = make([]URIEntry, len(e.SrcUri))
		for i, u := range e.SrcUri {
			base := filepath.Base(u.URL)
			filename := u.Filename
			if filename == base {
				filename = ""
			}
			data.SrcUri[i] = URIEntry{URL: u.URL, Filename: filename}
		}
	}

	tmpl := template.Must(template.New("ebuild").Parse(ebuildTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// Should not happen with valid data
		return fmt.Sprintf("Error generating ebuild: %v", err)
	}
	return buf.String()
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
					// Likely multi-line strings or unbalanced quotes.
					// Handle simple multi-line double-quoted string
					if strings.HasPrefix(val, "\"") {
						// Keep reading lines until we find the closing quote
						var sb strings.Builder
						sb.WriteString(val[1:])
						foundEnd := false
						for sc.Scan() {
							nextLine := sc.Text()
							// Don't trim space inside the string? Ebuild strings usually ignore newlines or treat them as space.
							// But for variable assignments, newlines are preserved if quoted.
							// However, usually people indent subsequent lines.

							// Check if line ends with quote
							trimmedNext := strings.TrimSpace(nextLine)
							if strings.HasSuffix(trimmedNext, "\"") {
								sb.WriteString("\n")
								// Extract content before the last quote
								// We need to be careful not to strip too much if there are spaces before the quote
								// But usually closing quote is on its own line or at end of content.

								// Find the last quote index in the ORIGINAL line (not trimmed)
								lastQuoteIdx := strings.LastIndex(nextLine, "\"")
								if lastQuoteIdx >= 0 {
									sb.WriteString(nextLine[:lastQuoteIdx])
								}
								foundEnd = true
								break
							} else {
								sb.WriteString("\n")
								sb.WriteString(nextLine)
							}
						}
						if foundEnd {
							val = sb.String()
						} else {
							continue // Unbalanced
						}
					} else {
						// Ignore other unbalanced cases
						continue
					}
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
