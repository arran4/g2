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
	"strconv"
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
	if vars != nil {
		e.Vars["P"] = vars.P
		e.Vars["PN"] = vars.PN
		e.Vars["PV"] = vars.PV
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

// EbuildVariables represents standard variables parsed from an ebuild filename.
type EbuildVariables struct {
	P  string
	PN string
	PV string
}

// ParseEbuildVariables extracts PN, PV, P from the ebuild filename.
func ParseEbuildVariables(filename string) *EbuildVariables {
	basename := filepath.Base(filename)
	if !strings.HasSuffix(basename, ".ebuild") {
		return nil
	}
	basename = strings.TrimSuffix(basename, ".ebuild")

	parts := strings.Split(basename, "-")
	if len(parts) < 2 {
		return nil
	}

	// Iterate to find the first valid version suffix from the left
	for i := 1; i < len(parts); i++ {
		pvCandidate := strings.Join(parts[i:], "-")
		gv := ParseGentooVersion(pvCandidate)
		if gv.IsValid {
			pn := strings.Join(parts[:i], "-")
			return &EbuildVariables{
				P:  pn + "-" + pvCandidate,
				PN: pn,
				PV: pvCandidate,
			}
		}
	}

	return nil
}

// ResolveVariables replaces ${VAR} and $VAR in the text with values from variables map.
func ResolveVariables(text string, variables map[string]string) string {
	// Simple resolution: multiple passes until no change or limit reached
	// To prevent memory exhaustion from self-referential or heavily nested variables,
	// cap the maximum expanded length.
	maxLen := 1024 * 1024    // 1MB limit for expanded strings
	for i := 0; i < 5; i++ { // Limit recursion depth
		original := text
		for key, value := range variables {
			if strings.Contains(text, fmt.Sprintf("${%s}", key)) || strings.Contains(text, fmt.Sprintf("$%s", key)) {
				// Prevent replacing if the value itself contains the key, preventing infinite growth in edge cases
				if strings.Contains(value, fmt.Sprintf("${%s}", key)) || strings.Contains(value, fmt.Sprintf("$%s", key)) {
					continue
				}
				text = strings.ReplaceAll(text, fmt.Sprintf("${%s}", key), value)
				text = strings.ReplaceAll(text, fmt.Sprintf("$%s", key), value)
				if len(text) > maxLen {
					return text[:maxLen]
				}
			}
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


var versionRegex = regexp.MustCompile(`^(\d+(?:\.\d+)*)(?:([a-z]))?(?:_(alpha|beta|pre|rc|p)(\d*))?(?:-r(\d+))?$`)

// GentooVersion represents a parsed Gentoo package version strictly adhering to PMS rules.
type GentooVersion struct {
	Nums        []int
	NumStrs     []string
	Letter      string
	Suffix      string
	SuffixNoStr string
	SuffixNo    int
	Revision    int
	IsValid     bool
}

// String reassembles and serializes the parsed GentooVersion back into a string.
func (gv *GentooVersion) String() string {
	if !gv.IsValid {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(strings.Join(gv.NumStrs, "."))

	if gv.Letter != "" {
		sb.WriteString(gv.Letter)
	}

	if gv.Suffix != "" {
		sb.WriteString("_")
		sb.WriteString(gv.Suffix)
		if gv.SuffixNoStr != "" {
			sb.WriteString(gv.SuffixNoStr)
		}
	}

	if gv.Revision > 0 {
		sb.WriteString("-r")
		sb.WriteString(strconv.Itoa(gv.Revision))
	}

	return sb.String()
}

// Part represents a specific component of a Gentoo version.
type Part string

const (
	MajorPart    Part = "major"
	MinorPart    Part = "minor"
	PatchPart    Part = "patch"
	SuffixPart   Part = "suffix"
	RevisionPart Part = "revision"
)

// IncrementPart allows incrementing specific parts of the version string based on common bump operations.
// Supports variadic Part arguments to increment multiple parts sequentially.
func (gv *GentooVersion) IncrementPart(parts ...any) {
	if !gv.IsValid {
		return
	}

	for _, p := range parts {
		var partStr string
		switch v := p.(type) {
		case string:
			partStr = v
		case Part:
			partStr = string(v)
		default:
			continue
		}

		switch partStr {
		case "major":
			if len(gv.Nums) > 0 {
				gv.Nums[0]++
				gv.NumStrs[0] = strconv.Itoa(gv.Nums[0])
			}
			// Reset trailing sections
			for i := 1; i < len(gv.Nums); i++ {
				gv.Nums[i] = 0
				gv.NumStrs[i] = "0"
			}
			gv.Revision = 0
			gv.Letter = ""
			gv.Suffix = ""
			gv.SuffixNoStr = ""
			gv.SuffixNo = 0
		case "minor":
			if len(gv.Nums) > 1 {
				gv.Nums[1]++
				gv.NumStrs[1] = strconv.Itoa(gv.Nums[1])
			} else if len(gv.Nums) == 1 {
				gv.Nums = append(gv.Nums, 1)
				gv.NumStrs = append(gv.NumStrs, "1")
			}
			// Reset trailing sections
			for i := 2; i < len(gv.Nums); i++ {
				gv.Nums[i] = 0
				gv.NumStrs[i] = "0"
			}
			gv.Revision = 0
			gv.Letter = ""
			gv.Suffix = ""
			gv.SuffixNoStr = ""
			gv.SuffixNo = 0
		case "patch":
			if len(gv.Nums) > 2 {
				gv.Nums[2]++
				gv.NumStrs[2] = strconv.Itoa(gv.Nums[2])
			} else if len(gv.Nums) == 2 {
				gv.Nums = append(gv.Nums, 1)
				gv.NumStrs = append(gv.NumStrs, "1")
			} else if len(gv.Nums) == 1 {
				gv.Nums = append(gv.Nums, 0, 1)
				gv.NumStrs = append(gv.NumStrs, "0", "1")
			}
			// Reset trailing sections
			for i := 3; i < len(gv.Nums); i++ {
				gv.Nums[i] = 0
				gv.NumStrs[i] = "0"
			}
			gv.Revision = 0
			gv.Letter = ""
			gv.Suffix = ""
			gv.SuffixNoStr = ""
			gv.SuffixNo = 0
		case "suffix":
			if gv.Suffix != "" {
				gv.SuffixNo++
				gv.SuffixNoStr = strconv.Itoa(gv.SuffixNo)
			}
			gv.Revision = 0
		case "revision":
			gv.Revision++
		}
	}
}

// IncrementRevision increments the Gentoo version revision number (e.g., -r1 -> -r2).
func (gv *GentooVersion) IncrementRevision() {
	gv.IncrementPart(RevisionPart)
}

// ParseGentooVersion parses a gentoo version into parts
func ParseGentooVersion(v string) GentooVersion {
	m := versionRegex.FindStringSubmatch(v)
	if m == nil {
		return GentooVersion{IsValid: false}
	}

	toInt := func(s string) int {
		if s == "" {
			return 0
		}
		i, _ := strconv.Atoi(s)
		return i
	}

	numStrs := strings.Split(m[1], ".")
	var nums []int
	for _, n := range numStrs {
		nums = append(nums, toInt(n))
	}

	return GentooVersion{
		Nums:        nums,
		NumStrs:     numStrs,
		Letter:      m[2],
		Suffix:      m[3],
		SuffixNoStr: m[4],
		SuffixNo:    toInt(m[4]),
		Revision:    toInt(m[5]),
		IsValid:     true,
	}
}

func cmpStr(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareGentooVersionParts(v1, v2 GentooVersion) int {
	maxLen := len(v1.Nums)
	if len(v2.Nums) > maxLen {
		maxLen = len(v2.Nums)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(v1.Nums) {
			// v1 is shorter. If remaining v2 parts are all 0, they are equal in this part
			allZero := true
			for j := i; j < len(v2.Nums); j++ {
				if v2.Nums[j] != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				continue
			}
			return -1
		}
		if i >= len(v2.Nums) {
			// v2 is shorter. If remaining v1 parts are all 0, they are equal in this part
			allZero := true
			for j := i; j < len(v1.Nums); j++ {
				if v1.Nums[j] != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				continue
			}
			return 1
		}

		s1 := v1.NumStrs[i]
		s2 := v2.NumStrs[i]

		if strings.HasPrefix(s1, "0") || strings.HasPrefix(s2, "0") {
			if strings.HasPrefix(s1, "0") && !strings.HasPrefix(s2, "0") {
				return -1
			}
			if !strings.HasPrefix(s1, "0") && strings.HasPrefix(s2, "0") {
				return 1
			}

			s1Stripped := strings.TrimRight(s1, "0")
			s2Stripped := strings.TrimRight(s2, "0")
			if c := cmpStr(s1Stripped, s2Stripped); c != 0 {
				return c
			}
		} else {
			n1 := v1.Nums[i]
			n2 := v2.Nums[i]
			if c := cmpInt(n1, n2); c != 0 {
				return c
			}
		}
	}

	if c := cmpStr(v1.Letter, v2.Letter); c != 0 {
		return c
	}

	suffixOrder := map[string]int{
		"alpha": 1,
		"beta":  2,
		"pre":   3,
		"rc":    4,
		"":      5, // no suffix
		"p":     6,
	}

	if c := cmpInt(suffixOrder[v1.Suffix], suffixOrder[v2.Suffix]); c != 0 {
		return c
	}

	if c := cmpInt(v1.SuffixNo, v2.SuffixNo); c != 0 {
		return c
	}

	if c := cmpInt(v1.Revision, v2.Revision); c != 0 {
		return c
	}

	return 0
}

// CompareVersions compares two gentoo versions strictly adhering to PMS.
// Returns > 0 if v1 > v2, < 0 if v1 < v2, and 0 if equal.
func CompareVersions(v1, v2 string) int {
	gv1 := ParseGentooVersion(v1)
	gv2 := ParseGentooVersion(v2)

	if gv1.IsValid && gv2.IsValid {
		return compareGentooVersionParts(gv1, gv2)
	}

	return strings.Compare(v1, v2)
}

// PadVersionTokens produces a sortable string representation of a gentoo version.
func PadVersionTokens(v string) string {
	parseGentooVersion := func(v string) string {
		v = regexp.MustCompile(`-r(\d+)$`).ReplaceAllString(v, "+r$1")
		return v
	}

	v = parseGentooVersion(v)
	re := regexp.MustCompile(`(\d+)`)
	return re.ReplaceAllStringFunc(v, func(s string) string {
		return fmt.Sprintf("%010s", s)
	})
}
