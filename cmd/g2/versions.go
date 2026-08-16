package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) CmdVersions(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: versions <subcommand>")
	}

	subcmd := args[0]
	switch subcmd {

	case "bump":
		if len(args) < 3 {
			return fmt.Errorf("usage: versions bump <ebuild filepath|version|- (stdin)|category/package directory> <major|minor|patch|revision|rev> [alpha, beta, pre, rc or p] [number]")
		}
		return bumpVersion(args[1:])
	case "compare":
		if len(args) < 4 {
			return fmt.Errorf("usage: versions compare <ebuild|version-string> <operator> <ebuild|version-string>")
		}
		return compareVersions(args[1:])
	case "convert":
		if len(args) < 3 {
			return fmt.Errorf("usage: versions convert <semantic-to-gentoo|gentoo-to-semantic|flutter-to-gentoo|gentoo-to-flutter> <version string | ->")
		}
		mode := args[1]
		input := args[2]

		var processFunc func(string) string
		switch mode {
		case "semantic-to-gentoo":
			processFunc = SemanticToGentoo
		case "gentoo-to-semantic":
			processFunc = GentooToSemantic
		case "flutter-to-gentoo":
			processFunc = FlutterToGentoo
		case "gentoo-to-flutter":
			processFunc = GentooToFlutter
		default:
			return fmt.Errorf("unknown convert mode: %s, must be semantic-to-gentoo, gentoo-to-semantic, flutter-to-gentoo, or gentoo-to-flutter", mode)
		}

		if input == "-" {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				fmt.Println(processFunc(scanner.Text()))
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error reading stdin: %w", err)
			}
		} else {
			fmt.Println(processFunc(input))
		}
	default:
		return fmt.Errorf("unknown versions subcommand: %s", subcmd)
	}

	return nil
}

// SemanticVersion represents a structured semantic version.
type SemanticVersion struct {
	Nums        []int
	NumStrs     []string
	Letter      string // optional trailing letter like 'b'
	PreRelease  string // e.g., alpha, beta, pre, rc, p
	PreReleaseN string // numeric part of pre-release
	Revision    int    // gentoo revision usually added like -r1
	IsValid     bool
}

// ParseSemanticVersion parses a version string loosely following semantic versioning,
// with awareness of Gentoo-specific suffixes like -r1 or -alpha1 that might appear
// out of standard semantic order.
func ParseSemanticVersion(v string) SemanticVersion {
	sv := SemanticVersion{IsValid: true}

	if v == "" {
		sv.IsValid = false
		return sv
	}

	// 1. Strip 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	// 2. Extract base numbers and letter
	i := 0
	for ; i < len(v); i++ {
		c := v[i]
		if unicode.IsDigit(rune(c)) || c == '.' {
			continue
		}
		if unicode.IsLetter(rune(c)) && (i+1 == len(v) || v[i+1] == '-' || v[i+1] == '_') {
			// Trailing letter in base like 1.2.3b
			sv.Letter = string(c)
			i++
			break
		}
		break
	}

	baseStr := v[:i]
	if sv.Letter != "" {
		baseStr = v[:i-1]
	}

	parts := strings.Split(baseStr, ".")
	for _, p := range parts {
		if p == "" {
			continue
		}
		num, err := strconv.Atoi(p)
		if err == nil {
			sv.Nums = append(sv.Nums, num)
			sv.NumStrs = append(sv.NumStrs, p)
		}
	}

	rem := v[i:]

	// 3. Extract Revision and PreRelease components
	// We parse by tokens separated by '-' or '_'
	tokens := strings.FieldsFunc(rem, func(r rune) bool {
		return r == '-' || r == '_'
	})

	for _, token := range tokens {
		if strings.HasPrefix(token, "r") && len(token) > 1 {
			// Check if it's a revision -rX
			isRev := true
			for j := 1; j < len(token); j++ {
				if !unicode.IsDigit(rune(token[j])) {
					isRev = false
					break
				}
			}
			if isRev {
				revNum, _ := strconv.Atoi(token[1:])
				sv.Revision = revNum
				continue
			}
		}

		// Check for pre-release
		preTypes := []string{"alpha", "beta", "pre", "rc", "p"}
		foundPre := false
		for _, pt := range preTypes {
			if strings.HasPrefix(token, pt) {
				sv.PreRelease = pt
				sv.PreReleaseN = token[len(pt):]
				foundPre = true
				break
			}
		}
		if !foundPre {
			// Unknown token
			sv.IsValid = false
		}
	}

	return sv
}

// SemanticToGentoo converts a semantic version string to a Gentoo version string.
func SemanticToGentoo(v string) string {
	sv := ParseSemanticVersion(v)
	if !sv.IsValid {
		return v // Fallback
	}

	res := strings.Join(sv.NumStrs, ".")
	if sv.Letter != "" {
		res += sv.Letter
	}
	if sv.PreRelease != "" {
		res += "_" + sv.PreRelease + sv.PreReleaseN
	}
	if sv.Revision > 0 {
		res += fmt.Sprintf("-r%d", sv.Revision)
	}

	return res
}

// ParseFlutterVersion parses a flutter version string into its components.
func ParseFlutterVersion(v string) (base string, pre string, build string) {
	v = strings.TrimPrefix(v, "v")

	if idx := strings.Index(v, "+"); idx != -1 {
		build = v[idx+1:]
		v = v[:idx]
	}

	if idx := strings.Index(v, "-"); idx != -1 {
		pre = v[idx+1:]
		v = v[:idx]
	}

	base = v
	return
}

// FlutterToGentoo converts a Flutter version string to a Gentoo version string.
func FlutterToGentoo(v string) string {
	base, pre, build := ParseFlutterVersion(v)
	var sb strings.Builder
	sb.Grow(len(base) + len(pre) + len(build) + 16)
	sb.WriteString(base)

	if pre != "" {
		pre = strings.TrimSuffix(pre, ".pre")
		parts := strings.Split(pre, ".")
		if len(parts) >= 2 {
			sb.WriteString("_pre")
			sb.WriteString(parts[0])
			sb.WriteString("_p")
			sb.WriteString(parts[1])
		} else if len(parts) == 1 {
			sb.WriteString("_pre")
			sb.WriteString(parts[0])
		} else {
			sb.WriteString("_pre")
			sb.WriteString(pre)
		}
	}

	if build != "" {
		build = strings.TrimPrefix(build, "hotfix.")
		sb.WriteString("_p")
		sb.WriteString(build)
	}

	return sb.String()
}

// GentooToFlutter converts a Gentoo version string to a Flutter version string.
func GentooToFlutter(v string) string {
	gv := g2.ParseGentooVersion(v)
	if !gv.IsValid {
		return v
	}

	res := strings.Join(gv.NumStrs, ".")
	if gv.Letter != "" {
		res += gv.Letter
	}

	var preParts []string
	var buildParts []string

	for _, suf := range gv.Suffixes {
		switch suf.Name {
		case "pre":
			preParts = append(preParts, suf.ValueStr)
		case "p":
			// If we already have a 'pre' but no patch for it, this '_p' acts as the pre-release's minor patch
			if len(preParts) > 0 && len(preParts) < 2 {
				preParts = append(preParts, suf.ValueStr)
			} else {
				buildParts = append(buildParts, suf.ValueStr)
			}
		default:
			preParts = append(preParts, suf.Name+suf.ValueStr)
		}
	}

	if len(preParts) > 0 {
		if len(preParts) == 1 {
			res += fmt.Sprintf("-%s.0.pre", preParts[0])
		} else {
			res += fmt.Sprintf("-%s.%s.pre", preParts[0], preParts[1])
		}
	}

	if len(buildParts) > 0 {
		res += "+" + buildParts[0]
	}

	if gv.Revision > 0 {
		if len(buildParts) == 0 {
			res += fmt.Sprintf("+%d", gv.Revision)
		} else {
			res += fmt.Sprintf(".%d", gv.Revision)
		}
	}

	return res
}

// GentooToSemantic converts a Gentoo version string to a semantic version string.
func GentooToSemantic(v string) string {
	gv := g2.ParseGentooVersion(v)
	if !gv.IsValid {
		return strings.ReplaceAll(v, "_", "-")
	}

	res := strings.Join(gv.NumStrs, ".")
	if gv.Letter != "" {
		res += gv.Letter
	}
	for _, s := range gv.Suffixes {
		res += "-" + s.Name
		res += s.ValueStr
	}
	if gv.Revision > 0 {
		res += fmt.Sprintf("-r%d", gv.Revision)
	}

	return res
}

func compareVersions(args []string) error {
	v1Str := args[0]
	op := args[1]
	v2Str := args[2]

	// handle if args are filepaths
	if strings.HasSuffix(v1Str, ".ebuild") {
		vars := g2.ParseEbuildVariables(v1Str)
		if vars != nil {
			if pv, ok := vars["PV"]; ok {
				v1Str = pv
				if pr, ok := vars["PR"]; ok && pr != "r0" {
					v1Str += "-" + pr
				}
			}
		}
	}
	if strings.HasSuffix(v2Str, ".ebuild") {
		vars := g2.ParseEbuildVariables(v2Str)
		if vars != nil {
			if pv, ok := vars["PV"]; ok {
				v2Str = pv
				if pr, ok := vars["PR"]; ok && pr != "r0" {
					v2Str += "-" + pr
				}
			}
		}
	}

	cmp := g2.CompareVersions(v1Str, v2Str)

	isTrue := false
	switch op {
	case "<", "lt", "older", "less-than":
		isTrue = cmp < 0
	case "<=", "le":
		isTrue = cmp <= 0
	case "==", "=", "eq", "equal":
		isTrue = cmp == 0
	case ">=", "ge":
		isTrue = cmp >= 0
	case ">", "gt", "newer", "greater-than":
		isTrue = cmp > 0
	default:
		return fmt.Errorf("unknown operator: %s", op)
	}

	fmt.Println(isTrue)
	if !isTrue {
		return &ExitError{Code: 1}
	}
	return nil
}

func parseBumpTarget(target string) string {
	if target == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text()
		}
		return ""
	}

	stat, err := os.Stat(target)
	if err == nil {
		if stat.IsDir() {
			// Find the highest ebuild in the directory
			entries, err := os.ReadDir(target)
			if err == nil {
				var highestVersion string
				for _, entry := range entries {
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
						continue
					}
					vars := g2.ParseEbuildVariables(entry.Name())
					if vars != nil {
						if pv, ok := vars["PV"]; ok {
							vStr := pv
							if pr, ok := vars["PR"]; ok && pr != "r0" {
								vStr += "-" + pr
							}
							if highestVersion == "" || g2.CompareVersions(vStr, highestVersion) > 0 {
								highestVersion = vStr
							}
						}
					}
				}
				if highestVersion != "" {
					return highestVersion
				}
			}
		} else if strings.HasSuffix(target, ".ebuild") {
			vars := g2.ParseEbuildVariables(target)
			if vars != nil {
				if pv, ok := vars["PV"]; ok {
					res := pv
					if pr, ok := vars["PR"]; ok && pr != "r0" {
						res += "-" + pr
					}
					return res
				}
			}
		}
	}

	return target // assume it's a raw version string
}

func bumpVersionString(target string, bumpType string, suffix string, forceNum int) (string, error) {
	v := g2.ParseGentooVersion(target)
	if !v.IsValid {
		return "", fmt.Errorf("invalid version string: %s", target)
	}

	switch bumpType {
	case "major":
		if len(v.Nums) > 0 {
			v.Nums[0]++
			v.NumStrs[0] = strconv.Itoa(v.Nums[0])
		}
		for i := 1; i < len(v.Nums); i++ {
			v.Nums[i] = 0
			v.NumStrs[i] = "0"
		}
		v.Revision = 0
		v.Letter = ""
		v.Suffixes = nil
	case "minor":
		if len(v.Nums) > 1 {
			v.Nums[1]++
			v.NumStrs[1] = strconv.Itoa(v.Nums[1])
		}
		for i := 2; i < len(v.Nums); i++ {
			v.Nums[i] = 0
			v.NumStrs[i] = "0"
		}
		v.Revision = 0
		v.Letter = ""
		v.Suffixes = nil
	case "patch":
		if len(v.Nums) > 2 {
			v.Nums[2]++
			v.NumStrs[2] = strconv.Itoa(v.Nums[2])
		}
		for i := 3; i < len(v.Nums); i++ {
			v.Nums[i] = 0
			v.NumStrs[i] = "0"
		}
		v.Revision = 0
		v.Letter = ""
		v.Suffixes = nil
	case "revision", "rev":
		v.Revision++
	}

	if suffix != "" {
		v.Suffixes = []g2.GentooSuffix{
			{Name: suffix},
		}
		if forceNum != -1 {
			v.Suffixes[0].Value = forceNum
			v.Suffixes[0].ValueStr = strconv.Itoa(forceNum)
		} else {
			v.Suffixes[0].Value = 1 // default
			v.Suffixes[0].ValueStr = "1"
		}
	}

	return v.String(), nil
}

func bumpVersion(args []string) error {
	target := parseBumpTarget(args[0])
	bumpType := args[1]

	suffix := ""
	forceNum := -1

	if len(args) > 2 {
		suffix = args[2]
	}
	if len(args) > 3 {
		forceNum, _ = strconv.Atoi(args[3])
	}

	res, err := bumpVersionString(target, bumpType, suffix, forceNum)
	if err != nil {
		return err
	}

	fmt.Println(res)
	return nil
}
