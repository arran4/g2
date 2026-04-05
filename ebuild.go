package g2

import (
	"io"

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
	Path          string
	Vars          map[string]string
	Functions     map[string]AST
	SrcUri        []URIEntry
	Mode          ParsingMode
	RawText       FileContent
	ParseWarnings []string

	orderOverride []string
	EbuildHeader  string
}

type varEntry struct {
	Key   string
	Value string
}

type funcEntry struct {
	Key   string
	Value AST
}

func (e *Ebuild) String() string {
	// Reconstruct a valid-ish ebuild
	// Since we don't preserve the whole file, we reconstruct what we know.
	// We do NOT output PN/PV/P variables as they are implicit from filename usually,
	// but if we parsed them from filename, we don't need to write them back to file.

	// Map to keep track of added items to prevent duplication
	addedVars := make(map[string]bool)
	addedFuncs := make(map[string]bool)

	var orderedItems []interface{}

	// Helper function to add a variable
	addVar := func(k string) {
		if addedVars[k] {
			return
		}
		if k == "P" || k == "PN" || k == "PV" {
			return
		}
		if k == "SRC_URI" && e.Mode == ParseFull && len(e.SrcUri) > 0 {
			return
		}
		val, ok := e.Vars[k]
		if !ok {
			return
		}

		if strings.Contains(val, "\n") {
			lines := strings.Split(val, "\n")
			for i, line := range lines {
				lines[i] = strings.TrimRight(line, " \t")
			}
			val = strings.Join(lines, "\n")
		}
		orderedItems = append(orderedItems, &varEntry{Key: k, Value: val})
		addedVars[k] = true
	}

	// Helper function to add a function
	addFunc := func(k string) {
		if addedFuncs[k] {
			return
		}
		val, ok := e.Functions[k]
		if !ok {
			return
		}

		if strings.Contains(val.Value, "\n") {
			lines := strings.Split(val.Value, "\n")
			for i, line := range lines {
				lines[i] = strings.TrimRight(line, " \t")
			}
			val.Value = strings.Join(lines, "\n")
		}
		orderedItems = append(orderedItems, &funcEntry{Key: k, Value: val})
		addedFuncs[k] = true
	}

	// 1. Process items in the exact order they appeared in the original source
	for _, name := range e.orderOverride {
		if _, isFunc := e.Functions[name]; isFunc {
			addFunc(name)
		} else if _, isVar := e.Vars[name]; isVar {
			addVar(name)
		}
	}

	// 2. Add remaining variables alphabetically
	var remainingVars []string
	for k := range e.Vars {
		if !addedVars[k] {
			remainingVars = append(remainingVars, k)
		}
	}
	sort.Strings(remainingVars)
	for _, k := range remainingVars {
		addVar(k)
	}

	// 3. Add remaining functions alphabetically
	var remainingFuncs []string
	for k := range e.Functions {
		if !addedFuncs[k] {
			remainingFuncs = append(remainingFuncs, k)
		}
	}
	sort.Strings(remainingFuncs)
	for _, k := range remainingFuncs {
		addFunc(k)
	}

	var buf bytes.Buffer
	if e.EbuildHeader != "" {
		buf.WriteString(e.EbuildHeader)
		buf.WriteString("\n\n")
	}

	for _, item := range orderedItems {
		switch v := item.(type) {
		case *varEntry:
			fmt.Fprintf(&buf, "%s=\"%s\"\n", v.Key, v.Value)
		case *funcEntry:
			fmt.Fprintf(&buf, "%s() %s\n", v.Key, v.Value.Value)
		}
	}

	if e.Mode == ParseFull && len(e.SrcUri) > 0 {
		buf.WriteString("SRC_URI=\"\n")
		for _, u := range e.SrcUri {
			base := filepath.Base(u.URL)
			filename := u.Filename
			if filename == base {
				filename = ""
			}
			fmt.Fprintf(&buf, "\t%s", u.URL)
			if filename != "" {
				fmt.Fprintf(&buf, " -> %s", filename)
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\"\n")
	}

	return buf.String()
}

// ParseEbuild parses an ebuild file with the specified mode.
func ParseEbuild(fsys fs.FS, path string, mode ParsingMode, opts ...any) (*Ebuild, error) {
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

	// Since we need it for parser right now, we can read it directly but save it via FileContent to allow caching or streaming logic if desired.
	// But actually, we want to allow the caller to specify if it should be retained lazily.
	// Ebuild parser receives `fsys` and `path`.
	// For ebuilds we can just create a WeakFileContent and `.Get()` it for the parser.
	isPersistent := true
	for _, opt := range opts {
		if b, ok := opt.(bool); ok {
			isPersistent = b
		}
	}

	loader := func() (io.ReadCloser, error) {
		return fsys.Open(path)
	}

	if isPersistent {
		e.RawText = NewLazyFileContent(loader, UseWeakPointer(true))
	} else {
		rc, err := loader()
		var b []byte
		if err == nil {
			b, _ = io.ReadAll(rc)
			_ = rc.Close()
		}
		e.RawText = &MemoryFileContent{Content: b}
	}

	contentBytes, err := e.RawText.Get()
	if err != nil || contentBytes == nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}
	content := string(*contentBytes)

	if mode >= ParseVariables {
		// Use the recursive descent parser
		parser := NewEbuildParser(context.Background(), strings.NewReader(content))
		parsedEbuild, err := parser.Parse()
		if err != nil {
			return nil, fmt.Errorf("parsing ebuild %s variables: %w", path, err)
		}

		e.ParseWarnings = append(parser.Warnings, parsedEbuild.Warnings...)

		// Since variables might depend on each other, we need to iterate
		// or at least resolve using the whole parsedVars map.
		// Add parsed vars to e.Vars
		for k, v := range parsedEbuild.Variables {
			e.Vars[k] = v
		}

		e.Functions = make(map[string]AST)
		for k, v := range parsedEbuild.Functions {
			e.Functions[k] = v
		}
		e.orderOverride = parsedEbuild.Order
		e.EbuildHeader = parsedEbuild.EbuildHeader
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

type DepNode interface {
	Evaluate(opts ...any) ([]string, error)
}

type DepString string

func (d DepString) Evaluate(opts ...any) ([]string, error) {
	return []string{string(d)}, nil
}

type DepAnyOf struct {
	Children []DepNode
}

func (d DepAnyOf) Evaluate(opts ...any) ([]string, error) {
	var res []string
	for _, c := range d.Children {
		vals, err := c.Evaluate(opts...)
		if err != nil {
			return nil, err
		}
		res = append(res, vals...)
	}
	return res, nil
}

type DepAllOf struct {
	Children []DepNode
}

func (d DepAllOf) Evaluate(opts ...any) ([]string, error) {
	var res []string
	for _, c := range d.Children {
		vals, err := c.Evaluate(opts...)
		if err != nil {
			return nil, err
		}
		res = append(res, vals...)
	}
	return res, nil
}

type UseFlags []string
type UseFlag string
type IgnoreUseFlags bool

type EvaluateConfig struct {
	UseFlags       map[string]bool
	IgnoreUseFlags bool
}

func parseOpts(opts ...any) EvaluateConfig {
	cfg := EvaluateConfig{
		UseFlags: make(map[string]bool),
	}
	for _, opt := range opts {
		switch o := opt.(type) {
		case UseFlags:
			for _, flag := range o {
				cfg.UseFlags[flag] = true
			}
		case UseFlag:
			cfg.UseFlags[string(o)] = true
		case IgnoreUseFlags:
			cfg.IgnoreUseFlags = bool(o)
		}
	}
	return cfg
}

type DepUseConditional struct {
	Flag      string
	IsNegated bool
	Children  []DepNode
}

func (d DepUseConditional) Evaluate(opts ...any) ([]string, error) {
	cfg := parseOpts(opts...)

	include := cfg.IgnoreUseFlags
	if !cfg.IgnoreUseFlags {
		hasFlag := cfg.UseFlags[d.Flag]
		if d.IsNegated {
			include = !hasFlag
		} else {
			include = hasFlag
		}
	}

	if !include {
		return nil, nil
	}

	var res []string
	for _, c := range d.Children {
		vals, err := c.Evaluate(opts...)
		if err != nil {
			return nil, err
		}
		res = append(res, vals...)
	}
	return res, nil
}

func parseDepTokens(tokens []string) ([]DepNode, int) {
	var nodes []DepNode
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t == "" {
			continue
		}
		if t == "||" {
			if i+1 < len(tokens) && tokens[i+1] == "(" {
				children, advance := parseDepTokens(tokens[i+2:])
				nodes = append(nodes, DepAnyOf{Children: children})
				i += advance + 1 // +1 for '('
			} else {
				nodes = append(nodes, DepString(t))
			}
		} else if strings.HasSuffix(t, "?") {
			flag := strings.TrimSuffix(t, "?")
			isNegated := false
			if strings.HasPrefix(flag, "!") {
				isNegated = true
				flag = flag[1:]
			}
			if i+1 < len(tokens) && tokens[i+1] == "(" {
				children, advance := parseDepTokens(tokens[i+2:])
				nodes = append(nodes, DepUseConditional{
					Flag:      flag,
					IsNegated: isNegated,
					Children:  children,
				})
				i += advance + 1
			} else {
				nodes = append(nodes, DepString(t))
			}
		} else if t == "(" {
			children, advance := parseDepTokens(tokens[i+1:])
			nodes = append(nodes, DepAllOf{Children: children})
			i += advance
		} else if t == ")" {
			return nodes, i + 1
		} else {
			nodes = append(nodes, DepString(t))
		}
	}
	return nodes, len(tokens)
}

type DepTree struct {
	Nodes []DepNode
}

func (d DepTree) Evaluate(opts ...any) ([]string, error) {
	var res []string
	for _, n := range d.Nodes {
		vals, err := n.Evaluate(opts...)
		if err != nil {
			return nil, err
		}
		res = append(res, vals...)
	}

	unique := make(map[string]bool)
	var final []string
	for _, r := range res {
		if !unique[r] {
			unique[r] = true
			final = append(final, r)
		}
	}
	return final, nil
}

// ParseDepTree parses a dependency string (like DEPEND, RDEPEND, LICENSE)
// into an AST that can be evaluated with Evaluate().
func ParseDepTree(s string) DepTree {
	tokens := strings.Fields(s)
	nodes, _ := parseDepTokens(tokens)
	return DepTree{Nodes: nodes}
}

// ParseLicense extracts individual license names from a LICENSE string,
// evaluating all conditionals to true to gather all possible licenses.
func ParseLicense(licenseStr string) []string {
	tree := ParseDepTree(licenseStr)
	res, _ := tree.Evaluate(IgnoreUseFlags(true))
	return res
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
			return map[string]string{
				"P":  pn + "-" + pvCandidate,
				"PN": pn,
				"PV": pvCandidate,
			}
		}
	}

	return nil
}

var varExpansionRegex = regexp.MustCompile(`\$\{([a-zA-Z0-9_]+)([^}]*)\}`)

// ResolveVariables replaces ${VAR} and $VAR in the text with values from variables map.
func ResolveVariables(text string, variables map[string]string) string {
	// Simple resolution: multiple passes until no change or limit reached
	// To prevent memory exhaustion from self-referential or heavily nested variables,
	// cap the maximum expanded length.
	maxLen := 1024 * 1024 // 1MB limit for expanded strings

	// 1. Sort keys by length descending to prevent $P from matching before $PN
	var keys []string
	for k := range variables {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	for i := 0; i < 5; i++ { // Limit recursion depth
		original := text

		// 1. Replace all simple $VAR substitutions
		for _, key := range keys {
			value := variables[key]
			varRef := fmt.Sprintf("$%s", key)
			if strings.Contains(text, varRef) {
				if strings.Contains(value, varRef) {
					continue
				}
				text = strings.ReplaceAll(text, varRef, value)
			}
		}

		// 2. Replace all ${VAR...} substitutions
		text = varExpansionRegex.ReplaceAllStringFunc(text, func(match string) string {
			parts := varExpansionRegex.FindStringSubmatch(match)
			if len(parts) != 3 {
				return match
			}
			k := parts[1]
			op := parts[2]
			v, ok := variables[k]

			if op == "" {
				if ok {
					return v
				}
				return ""
			}
			if strings.HasPrefix(op, ":-") || strings.HasPrefix(op, "-") {
				defVal := strings.TrimPrefix(strings.TrimPrefix(op, ":-"), "-")
				if v != "" {
					return v
				}
				return defVal
			}
			if strings.HasPrefix(op, "//") {
				replParts := strings.SplitN(op[2:], "/", 2)
				if len(replParts) == 2 {
					return strings.ReplaceAll(v, replParts[0], replParts[1])
				}
				return strings.ReplaceAll(v, replParts[0], "")
			}
			if strings.HasPrefix(op, "/") {
				replParts := strings.SplitN(op[1:], "/", 2)
				if len(replParts) == 2 {
					return strings.Replace(v, replParts[0], replParts[1], 1)
				}
				return strings.Replace(v, replParts[0], "", 1)
			}
			if strings.HasPrefix(op, "##") {
				prefix := op[2:]
				if strings.HasPrefix(v, prefix) {
					return v[len(prefix):]
				}
				return v
			}
			if strings.HasPrefix(op, "#") {
				prefix := op[1:]
				if strings.HasPrefix(v, prefix) {
					return v[len(prefix):]
				}
				return v
			}
			if strings.HasPrefix(op, "%%") {
				suffix := op[2:]
				if strings.HasSuffix(v, suffix) {
					return v[:len(v)-len(suffix)]
				}
				return v
			}
			if strings.HasPrefix(op, "%") {
				suffix := op[1:]
				if strings.HasSuffix(v, suffix) {
					return v[:len(v)-len(suffix)]
				}
				return v
			}

			// Unknown operation
			return match
		})

		if len(text) > maxLen {
			return text[:maxLen]
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

// PackageAtom represents a parsed Gentoo package dependency specification.
type PackageAtom struct {
	Operator string // e.g. ">=", "~", "!", "!!", ""
	Category string // e.g. "dev-lang"
	Name     string // e.g. "python"
	Version  string // e.g. "3.10.4-r1"
	Slot     string // e.g. "0/3.10"
	UseFlags string // e.g. "sqlite,xml"
}

// ParsePackageAtom parses a raw dependency string into its constituent parts.
func ParsePackageAtom(dep string) PackageAtom {
	var atom PackageAtom

	// 1. Extract Operator
	for len(dep) > 0 && (dep[0] == '>' || dep[0] == '<' || dep[0] == '=' || dep[0] == '~' || dep[0] == '!') {
		atom.Operator += string(dep[0])
		dep = dep[1:]
	}

	// 2. Extract USE Flags
	if idx := strings.Index(dep, "["); idx != -1 {
		if endIdx := strings.LastIndex(dep, "]"); endIdx > idx {
			atom.UseFlags = dep[idx+1 : endIdx]
			dep = dep[:idx] // strip USE flags
		}
	}

	// 3. Extract Slot
	if idx := strings.Index(dep, ":"); idx != -1 {
		atom.Slot = dep[idx+1:]
		dep = dep[:idx] // strip slot
	}

	// 4. Extract Category, Name, and Version
	// dep is now something like "dev-lang/python-3.10.0-r1" or "virtual/pkgconfig"
	parts := strings.Split(dep, "/")
	if len(parts) == 2 {
		atom.Category = parts[0]
		nameAndVer := parts[1]

		// Find where the version starts. A version usually starts after a hyphen followed by a digit.
		verStartIdx := -1
		for i := 0; i < len(nameAndVer)-1; i++ {
			if nameAndVer[i] == '-' && nameAndVer[i+1] >= '0' && nameAndVer[i+1] <= '9' {
				verStartIdx = i
				break
			}
		}

		if verStartIdx != -1 {
			atom.Name = nameAndVer[:verStartIdx]
			atom.Version = nameAndVer[verStartIdx+1:]
		} else {
			atom.Name = nameAndVer
		}
	} else {
		// Fallback for malformed strings missing category
		atom.Name = dep
	}

	return atom
}

// ExtractPackageNameFromDep strips version, slot, and USE flags from a package string
// using the AST parser PackageAtom to satisfy architectural requirements.
func ExtractPackageNameFromDep(dep string) string {
	atom := ParsePackageAtom(dep)
	if atom.Category != "" {
		return atom.Category + "/" + atom.Name
	}
	return atom.Name
}
