package g2

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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
	RawText       string
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

func cleanValue(v string) string {
	if !strings.Contains(v, "\n") {
		return v
	}
	lines := strings.Split(v, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

func (e *Ebuild) addVar(k string, addedVars map[string]bool, orderedItems *[]interface{}) {
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

	val = cleanValue(val)
	*orderedItems = append(*orderedItems, &varEntry{Key: k, Value: val})
	addedVars[k] = true
}

func (e *Ebuild) addFunc(k string, addedFuncs map[string]bool, orderedItems *[]interface{}) {
	if addedFuncs[k] {
		return
	}
	val, ok := e.Functions[k]
	if !ok {
		return
	}

	val.Value = cleanValue(val.Value)
	*orderedItems = append(*orderedItems, &funcEntry{Key: k, Value: val})
	addedFuncs[k] = true
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

	// 1. Process items in the exact order they appeared in the original source
	for _, name := range e.orderOverride {
		if _, isFunc := e.Functions[name]; isFunc {
			e.addFunc(name, addedFuncs, &orderedItems)
		} else if _, isVar := e.Vars[name]; isVar {
			e.addVar(name, addedVars, &orderedItems)
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
		e.addVar(k, addedVars, &orderedItems)
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
		e.addFunc(k, addedFuncs, &orderedItems)
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
	e.RawText = content

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

			origRev := gv.Revision
			gv.Revision = 0
			pvBase := gv.String()
			gv.Revision = origRev

			return map[string]string{
				"PN":  pn,
				"PV":  pvBase,
				"P":   pn + "-" + pvBase,
				"PR":  fmt.Sprintf("r%d", origRev),
				"PVR": pvCandidate,
				"PF":  pn + "-" + pvCandidate,
			}
		}
	}

	return nil
}

// ResolveVariables replaces ${VAR} and $VAR in the text with values from variables map.
func ResolveVariables(text string, variables map[string]string) string {
	maxLen := 100 * 1024

	for i := 0; i < 5; i++ { // Limit recursion depth
		original := text
		text = resolveBash(context.Background(), text, variables, WithFastPath())

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

// GentooVersion represents a parsed Gentoo package version strictly adhering to PMS rules.
type GentooSuffix struct {
	Name     string
	Value    int
	ValueStr string
}

type GentooVersion struct {
	Nums     []int
	NumStrs  []string
	Letter   string
	Suffixes []GentooSuffix
	Revision int
	IsValid  bool
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

	for _, s := range gv.Suffixes {
		sb.WriteString("_")
		sb.WriteString(s.Name)
		sb.WriteString(s.ValueStr)
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
			gv.Suffixes = nil
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
			gv.Suffixes = nil
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
			gv.Suffixes = nil
		case "suffix":
			if len(gv.Suffixes) > 0 {
				lastIdx := len(gv.Suffixes) - 1
				gv.Suffixes[lastIdx].Value++
				gv.Suffixes[lastIdx].ValueStr = strconv.Itoa(gv.Suffixes[lastIdx].Value)
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
	if v == "" {
		return GentooVersion{IsValid: false}
	}

	toInt := func(s string) int {
		if s == "" {
			return 0
		}
		i, _ := strconv.Atoi(s)
		return i
	}

	var nums []int
	var numStrs []string

	// Start parsing Nums
	i := 0
	for ; i < len(v); i++ {
		start := i
		for i < len(v) && v[i] >= '0' && v[i] <= '9' {
			i++
		}
		if i == start {
			return GentooVersion{IsValid: false}
		}
		numStrs = append(numStrs, v[start:i])
		nums = append(nums, toInt(v[start:i]))

		if i < len(v) && v[i] == '.' {
			// Expect another number part
			// But if it's the last character, it's invalid
			if i+1 == len(v) {
				return GentooVersion{IsValid: false}
			}
			continue
		} else {
			break
		}
	}

	var letter string
	if i < len(v) && v[i] >= 'a' && v[i] <= 'z' {
		letter = string(v[i])
		i++
	}

	var suffixes []GentooSuffix
	for i < len(v) && v[i] == '_' {
		i++
		start := i
		var suffixName string
		// valid suffixes: alpha, beta, pre, rc, p
		validSuffixes := []string{"alpha", "beta", "pre", "rc", "p"}
		for _, s := range validSuffixes {
			if strings.HasPrefix(v[start:], s) {
				suffixName = s
				i += len(s)
				break
			}
		}
		if suffixName == "" {
			return GentooVersion{IsValid: false}
		}
		startNum := i
		for i < len(v) && v[i] >= '0' && v[i] <= '9' {
			i++
		}
		valStr := v[startNum:i]
		suffixes = append(suffixes, GentooSuffix{
			Name:     suffixName,
			Value:    toInt(valStr),
			ValueStr: valStr,
		})
	}

	var revision int
	if i < len(v) && strings.HasPrefix(v[i:], "-r") {
		i += 2
		start := i
		for i < len(v) && v[i] >= '0' && v[i] <= '9' {
			i++
		}
		if i == start {
			return GentooVersion{IsValid: false}
		}
		revision = toInt(v[start:i])
	}

	if i < len(v) {
		return GentooVersion{IsValid: false} // Trailing characters
	}

	return GentooVersion{
		Nums:     nums,
		NumStrs:  numStrs,
		Letter:   letter,
		Suffixes: suffixes,
		Revision: revision,
		IsValid:  true,
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
		"p":     6,
	}

	maxSuffixLen := len(v1.Suffixes)
	if len(v2.Suffixes) > maxSuffixLen {
		maxSuffixLen = len(v2.Suffixes)
	}

	for i := 0; i < maxSuffixLen; i++ {
		var s1, s2 GentooSuffix

		if i >= len(v1.Suffixes) {
			s2 = v2.Suffixes[i]
			if s2.Name == "p" {
				return -1
			}
			return 1
		} else if i >= len(v2.Suffixes) {
			s1 = v1.Suffixes[i]
			if s1.Name == "p" {
				return 1
			}
			return -1
		}

		s1 = v1.Suffixes[i]
		s2 = v2.Suffixes[i]

		if c := cmpInt(suffixOrder[s1.Name], suffixOrder[s2.Name]); c != 0 {
			return c
		}
		if c := cmpInt(s1.Value, s2.Value); c != 0 {
			return c
		}
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
	replaceRIndex := -1
	for i := len(v) - 1; i >= 0; i-- {
		if v[i] >= '0' && v[i] <= '9' {
			continue
		}
		if v[i] == 'r' && i > 0 && v[i-1] == '-' {
			if i < len(v)-1 {
				replaceRIndex = i - 1
			}
		}
		break
	}

	var sb strings.Builder
	// Over-allocate slightly to avoid resizing during padding
	sb.Grow(len(v) + 30)

	for i := 0; i < len(v); {
		if i == replaceRIndex {
			sb.WriteString("+r")
			i += 2
			continue
		}

		if v[i] >= '0' && v[i] <= '9' {
			start := i
			for i < len(v) && v[i] >= '0' && v[i] <= '9' {
				i++
			}
			pad := 10 - (i - start)
			if pad > 0 {
				for j := 0; j < pad; j++ {
					sb.WriteByte('0')
				}
			}
			sb.WriteString(v[start:i])
		} else {
			sb.WriteByte(v[i])
			i++
		}
	}
	return sb.String()
}

// PackageAtom represents a parsed Gentoo package dependency specification.
type PackageAtom struct {
	Operator string // e.g. ">=", "~", "!", "!!", ""
	Category string // e.g. "dev-lang"
	Name     string // e.g. "python"
	Version  string // e.g. "3.10.4-r1"
	Slot     string // e.g. "0/3.10"
	Repo     string // e.g. "arrans-overlay"
	UseFlags string // e.g. "sqlite,xml"
}

// String returns the Gentoo package atom specification as a string.
func (a PackageAtom) String() string {
	var sb strings.Builder
	sb.WriteString(a.Operator)
	if a.Category != "" {
		sb.WriteString(a.Category)
		sb.WriteString("/")
	}
	sb.WriteString(a.Name)
	if a.Version != "" {
		sb.WriteString("-")
		sb.WriteString(a.Version)
	}
	if a.Slot != "" {
		sb.WriteString(":")
		sb.WriteString(a.Slot)
	}
	if a.Repo != "" {
		sb.WriteString("::")
		sb.WriteString(a.Repo)
	}
	if a.UseFlags != "" {
		sb.WriteString("[")
		sb.WriteString(a.UseFlags)
		sb.WriteString("]")
	}
	return sb.String()
}

// QualifyAtomForRepo validates that the given package atom specification is valid and
// belongs to targetRepo. If the atom has no repository qualifier, ::targetRepo is appended.
// If the atom already specifies ::targetRepo, it is accepted.
// If the atom specifies a conflicting repository qualifier, an error is returned.
// Repo-wide wildcard atoms (e.g. */*) are rejected because repo-wide policy requires explicit commands.
func validateCategoryName(cat string) error {
	if cat == "" {
		return fmt.Errorf("category cannot be empty")
	}
	if cat[0] == '-' || cat[0] == '+' || cat[0] == '.' {
		return fmt.Errorf("category %q must not begin with %q", cat, string(cat[0]))
	}
	for i := 0; i < len(cat); i++ {
		c := cat[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '_' || c == '.' || c == '-' {
			continue
		}
		return fmt.Errorf("category %q contains invalid character %q", cat, string(c))
	}
	return nil
}

func validatePackageName(pkg string) error {
	if pkg == "" {
		return fmt.Errorf("package name cannot be empty")
	}
	if pkg[0] == '-' || pkg[0] == '+' {
		return fmt.Errorf("package name %q must not begin with %q", pkg, string(pkg[0]))
	}
	for i := 0; i < len(pkg); i++ {
		c := pkg[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '_' || c == '-' {
			continue
		}
		return fmt.Errorf("package name %q contains invalid character %q", pkg, string(c))
	}
	return nil
}

func validateSlotName(name string) error {
	if name == "" {
		return fmt.Errorf("slot cannot be empty")
	}
	if name[0] == '-' || name[0] == '+' || name[0] == '.' {
		return fmt.Errorf("slot %q must not begin with %q", name, string(name[0]))
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '_' || c == '.' || c == '-' {
			continue
		}
		return fmt.Errorf("slot %q contains invalid character %q", name, string(c))
	}
	return nil
}

// QualifyAtomForRepo validates that the given package atom specification is valid according to
// Gentoo PMS rules and belongs to targetRepo. If the atom has no repository qualifier, ::targetRepo is appended.
// If the atom already specifies ::targetRepo, it is accepted.
// If the atom specifies a conflicting repository qualifier, an error is returned.
// Repo-wide wildcard atoms (e.g. */*) are rejected because repo-wide policy requires explicit commands.
func QualifyAtomForRepo(dep string, targetRepo string) (string, error) {
	if dep == "" {
		return "", fmt.Errorf("package atom cannot be empty")
	}
	if strings.ContainsAny(dep, " \t\r\n\x00") {
		return "", fmt.Errorf("invalid package atom %q: whitespace is not permitted", dep)
	}
	if err := ValidateRepoName(targetRepo); err != nil {
		return "", fmt.Errorf("invalid target repository %q: %w", targetRepo, err)
	}

	// Reject repo-wide wildcard atoms (*/* or */*::...)
	if dep == "*/*" || strings.HasPrefix(dep, "*/*::") {
		return "", fmt.Errorf("repo-wide wildcard atom %q is not permitted in package-specific commands; repo-wide policy requires an explicit repo-wide command", dep)
	}

	s := dep

	// 1. Extract USE flags ([...])
	var useFlags string
	if strings.Contains(s, "[") {
		if !strings.HasSuffix(s, "]") {
			return "", fmt.Errorf("invalid package atom %q: malformed USE flags specification", dep)
		}
		idx := strings.LastIndex(s, "[")
		useFlags = s[idx+1 : len(s)-1]
		if useFlags == "" {
			return "", fmt.Errorf("invalid package atom %q: empty USE flags []", dep)
		}
		for i := 0; i < len(useFlags); i++ {
			c := useFlags[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
				c == '+' || c == '_' || c == '-' || c == '@' || c == ',' || c == '!' || c == '?' || c == '=' || c == '(' || c == ')' || c == '*' {
				continue
			}
			return "", fmt.Errorf("invalid package atom %q: invalid character in USE flags %q", dep, string(c))
		}
		s = s[:idx]
	}

	// 2. Extract Repository qualifier (::repo)
	var specifiedRepo string
	if strings.Contains(s, "::") {
		if strings.Count(s, "::") > 1 {
			return "", fmt.Errorf("invalid package atom %q: multiple repository qualifiers", dep)
		}
		if strings.HasSuffix(s, "::") {
			return "", fmt.Errorf("invalid package atom %q: empty repository qualifier", dep)
		}
		idx := strings.Index(s, "::")
		repoPart := s[idx+2:]
		if err := ValidateRepoName(repoPart); err != nil {
			return "", fmt.Errorf("invalid package atom %q: %w", dep, err)
		}
		specifiedRepo = repoPart
		s = s[:idx]
	}

	// 3. Extract Slot / Subslot (:slot[/subslot])
	var slotPart string
	if strings.Contains(s, ":") {
		idx := strings.LastIndex(s, ":")
		slotPart = s[idx+1:]
		if slotPart == "" {
			return "", fmt.Errorf("invalid package atom %q: empty slot specification", dep)
		}
		s = s[:idx]

		slotBase := strings.TrimSuffix(slotPart, "=")
		if slotBase != "*" && slotBase != "" {
			if strings.Contains(slotBase, "/") {
				parts := strings.Split(slotBase, "/")
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					return "", fmt.Errorf("invalid package atom %q: malformed slot/subslot %q", dep, slotPart)
				}
				if err := validateSlotName(parts[0]); err != nil {
					return "", fmt.Errorf("invalid package atom %q: invalid slot %q: %w", dep, parts[0], err)
				}
				if parts[1] != "*" && parts[1] != "=" {
					if err := validateSlotName(parts[1]); err != nil {
						return "", fmt.Errorf("invalid package atom %q: invalid subslot %q: %w", dep, parts[1], err)
					}
				}
			} else {
				if err := validateSlotName(slotBase); err != nil {
					return "", fmt.Errorf("invalid package atom %q: invalid slot %q: %w", dep, slotBase, err)
				}
			}
		}
	}

	// 4. Extract Blocker and Operator
	var blocker string
	if strings.HasPrefix(s, "!!") {
		blocker = "!!"
		s = s[2:]
	} else if strings.HasPrefix(s, "!") {
		blocker = "!"
		s = s[1:]
	}

	var op string
	if strings.HasPrefix(s, ">=") || strings.HasPrefix(s, "<=") {
		op = s[:2]
		s = s[2:]
	} else if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "~") || strings.HasPrefix(s, ">") || strings.HasPrefix(s, "<") {
		op = s[:1]
		s = s[1:]
	}

	// 5. Category / Package[-version] split
	if strings.Count(s, "/") != 1 {
		return "", fmt.Errorf("invalid package atom %q: expected category/package format", dep)
	}
	slashIdx := strings.Index(s, "/")
	catStr := s[:slashIdx]
	pkgVerStr := s[slashIdx+1:]

	if err := validateCategoryName(catStr); err != nil {
		return "", fmt.Errorf("invalid package atom %q: %w", dep, err)
	}

	// 6. Split pkgVerStr into pkgName and versionStr
	var pkgName, versionStr string
	parts := strings.Split(pkgVerStr, "-")
	for i := 1; i < len(parts); i++ {
		candidateVer := strings.Join(parts[i:], "-")
		if len(candidateVer) > 0 && candidateVer[0] >= '0' && candidateVer[0] <= '9' {
			gv := ParseGentooVersion(candidateVer)
			if gv.IsValid {
				pkgName = strings.Join(parts[:i], "-")
				versionStr = candidateVer
				break
			}
		}
	}
	if pkgName == "" {
		pkgName = pkgVerStr
	}

	if err := validatePackageName(pkgName); err != nil {
		return "", fmt.Errorf("invalid package atom %q: %w", dep, err)
	}

	// 7. Validate Operator vs Version consistency
	if op != "" {
		if versionStr == "" {
			return "", fmt.Errorf("invalid package atom %q: operator %q requires a version", dep, op)
		}
		gv := ParseGentooVersion(versionStr)
		if !gv.IsValid {
			return "", fmt.Errorf("invalid package atom %q: invalid version %q", dep, versionStr)
		}
	} else {
		if versionStr != "" {
			return "", fmt.Errorf("invalid package atom %q: versioned package dependency requires an operator", dep)
		}
	}

	// 8. Validate Repository matching
	if specifiedRepo != "" && specifiedRepo != targetRepo {
		return "", fmt.Errorf("package qualifier %q does not match selected repository %q", specifiedRepo, targetRepo)
	}

	// 9. Reassemble qualified atom string
	var sb strings.Builder
	sb.WriteString(blocker)
	sb.WriteString(op)
	sb.WriteString(catStr)
	sb.WriteString("/")
	sb.WriteString(pkgName)
	if versionStr != "" {
		sb.WriteString("-")
		sb.WriteString(versionStr)
	}
	if slotPart != "" {
		sb.WriteString(":")
		sb.WriteString(slotPart)
	}
	sb.WriteString("::")
	sb.WriteString(targetRepo)
	if useFlags != "" {
		sb.WriteString("[")
		sb.WriteString(useFlags)
		sb.WriteString("]")
	}

	return sb.String(), nil
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

	// 3. Extract Repo (::repo)
	if idx := strings.Index(dep, "::"); idx != -1 {
		atom.Repo = dep[idx+2:]
		dep = dep[:idx] // strip repo
	}

	// 4. Extract Slot (:slot)
	if idx := strings.Index(dep, ":"); idx != -1 {
		atom.Slot = dep[idx+1:]
		dep = dep[:idx] // strip slot
	}

	// 5. Extract Category, Name, and Version
	// dep is now something like "dev-lang/python-3.10.0-r1" or "virtual/pkgconfig"
	parts := strings.Split(dep, "/")
	var nameAndVer string
	if len(parts) == 2 {
		atom.Category = parts[0]
		nameAndVer = parts[1]
	} else {
		// Fallback for malformed strings missing category
		nameAndVer = dep
	}

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

	return atom
}

// ExtractPackageNameFromDep strips version, slot, and USE flags from a package string
// using the AST parser PackageAtom to satisfy architectural requirements.
func ExtractPackageNameFromDep(dep string) string {
	if !strings.ContainsAny(dep, "><=~![:") && !strings.Contains(dep, "/*") && !strings.ContainsAny(dep, "0123456789") {
		return dep
	}
	atom := ParsePackageAtom(dep)
	if atom.Category != "" {
		return atom.Category + "/" + atom.Name
	}
	return atom.Name
}

// GetLatestMatchingPackageRevision finds the highest revision for a given package and base version.
// It looks for ebuilds in `category/pkgName` directory within the provided `fs.FS`.
// The version argument is the base version to match against (e.g. "2.17.0").
// Returns the full version string (including the highest revision, e.g. "2.17.0-r2").
func GetLatestMatchingPackageRevision(overlayFS fs.FS, category, pkgName, version string) (string, error) {
	dir := filepath.Join(category, pkgName)
	entries, err := fs.ReadDir(overlayFS, dir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var highestRevGV GentooVersion
	found := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".ebuild") {
			continue
		}

		vars := ParseEbuildVariables(name)
		if vars == nil || vars["PV"] == "" {
			continue
		}

		gv := ParseGentooVersion(vars["PVR"])
		origRev := gv.Revision
		gv.Revision = 0
		base := gv.String()
		gv.Revision = origRev

		if base == version {
			if !found || gv.Revision > highestRevGV.Revision {
				highestRevGV = gv
				found = true
			}
		}
	}

	if !found {
		return "", nil
	}

	return highestRevGV.String(), nil
}

// ParseEbuildVariablesFromReader parses an ebuild file from a reader and extracts simple variables defined in the top level.
func ParseEbuildVariablesFromReader(r io.Reader) map[string]string {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				vars[key] = val
			}
		}
	}
	return vars
}
