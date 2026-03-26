package g2

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"github.com/hashicorp/go-version"
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
		val := e.Vars[k]
		// Clean up value: strip trailing whitespace from each line
		// This avoids issues where editors or tools strip trailing whitespace from golden files
		// causing mismatch with generated output.
		if strings.Contains(val, "\n") {
			lines := strings.Split(val, "\n")
			for i, line := range lines {
				lines[i] = strings.TrimRight(line, " \t")
			}
			val = strings.Join(lines, "\n")
		}
		entries = append(entries, varEntry{Key: k, Value: val})
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
		// Use the recursive descent parser
		parser := NewEbuildParser(context.Background(), strings.NewReader(content))
		parsedVars, err := parser.Parse()
		if err != nil {
			return nil, fmt.Errorf("parsing ebuild variables: %w", err)
		}

		// Since variables might depend on each other, we need to iterate
		// or at least resolve using the whole parsedVars map.
		// Add parsed vars to e.Vars
		for k, v := range parsedVars {
			e.Vars[k] = v
		}
		// Resolve all values now that all vars are added
		// Using a multi-pass approach to resolve nested variables
		for pass := 0; pass < 5; pass++ {
			changed := false
			for k, v := range e.Vars {
				resolved := ResolveVariables(v, e.Vars)
				if resolved != v {
					e.Vars[k] = resolved
					changed = true
				}
			}
			if !changed {
				break
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

// ParseIUSE extracts the actual USE flag names from an IUSE string,
// stripping prefixes like + and -.
func ParseIUSE(iuseStr string) []string {
	flags := strings.Fields(iuseStr)
	var parsed []string
	for _, flagName := range flags {
		flagName = strings.TrimPrefix(flagName, "+")
		flagName = strings.TrimPrefix(flagName, "-")
		if flagName != "" {
			parsed = append(parsed, flagName)
		}
	}
	return parsed
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


// CompareVersions compares two gentoo versions. Returns > 0 if v1 > v2, < 0 if v1 < v2, and 0 if equal.
func CompareVersions(v1, v2 string) int {
	// Attempt parsing via go-version, fallback to strings.Compare
	parseGentooVersion := func(v string) string {
		v = regexp.MustCompile(`-r(\d+)$`).ReplaceAllString(v, "+r$1")
		return v
	}

	ver1, err1 := version.NewVersion(parseGentooVersion(v1))
	ver2, err2 := version.NewVersion(parseGentooVersion(v2))

	if err1 == nil && err2 == nil {
		if ver1.LessThan(ver2) {
			return -1
		} else if ver2.LessThan(ver1) {
			return 1
		}
		return 0
	}

	return strings.Compare(v1, v2)
}
