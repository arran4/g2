package g2

import (
	"fmt"
	"regexp"
	"strings"
)

// MaskCandidate identifies an available repository version. An empty Slot means
// that slot metadata is unknown, not that the package belongs to slot zero.
type MaskCandidate struct {
	Category   string `json:"category"`
	Package    string `json:"package"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Slot       string `json:"slot,omitempty"`
}

func (c MaskCandidate) String() string {
	return c.Category + "/" + c.Package + "-" + c.Version + "::" + c.Repository
}

// MaskRule retains the physical identity of an individual mask/unmask line.
// Repository scopes repository-wide rules; profile and user rules leave it empty.
type MaskRule struct {
	Atom       string `json:"atom"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Repository string `json:"repository,omitempty"`
	User       bool   `json:"user,omitempty"`
	Problem    string `json:"problem,omitempty"`
}

type UnmaskClassification string

const (
	UnmaskRequired      UnmaskClassification = "required"
	UnmaskNonMatching   UnmaskClassification = "non-matching"
	UnmaskNoChange      UnmaskClassification = "no-change"
	UnmaskShadowed      UnmaskClassification = "duplicate/shadowed"
	UnmaskIndeterminate UnmaskClassification = "indeterminate"
)

// MaskAuditInput is a snapshot of available versions and stacked active rules.
// Problems prevent claims of redundancy when discovery was incomplete.
type MaskAuditInput struct {
	Candidates []MaskCandidate
	Masks      []MaskRule
	Unmasks    []MaskRule
	Problems   []string
}

type MaskState struct {
	Masked        bool
	Masks         []MaskRule
	Unmasks       []MaskRule
	Indeterminate []string
}

type UnmaskAffectedVersion struct {
	Candidate MaskCandidate `json:"candidate"`
	Masks     []MaskRule    `json:"masks"`
}

type UnmaskAuditEntry struct {
	Source         MaskRule                `json:"source"`
	Classification UnmaskClassification    `json:"classification"`
	Explanation    string                  `json:"explanation"`
	Matching       int                     `json:"matching_versions"`
	Affected       []UnmaskAffectedVersion `json:"affected,omitempty"`
	CoveredBy      []MaskRule              `json:"covered_by,omitempty"`
}

type UnmaskAuditSummary struct {
	Required      int `json:"required"`
	NonMatching   int `json:"non-matching"`
	NoChange      int `json:"no-change"`
	Shadowed      int `json:"duplicate/shadowed"`
	Indeterminate int `json:"indeterminate"`
}

type UnmaskAuditReport struct {
	Entries  []UnmaskAuditEntry `json:"entries"`
	Summary  UnmaskAuditSummary `json:"summary"`
	Warnings []string           `json:"warnings,omitempty"`
}

var maskAtomComponent = regexp.MustCompile(`^[A-Za-z0-9_+][A-Za-z0-9_+.-]*$`)

// MatchMaskAtom distinguishes unsupported syntax/metadata from a non-match.
// Only package.mask semantics are considered; keyword and license acceptance
// and installed package state are outside this operation.
func MatchMaskAtom(raw string, c MaskCandidate) (bool, error) {
	wild := strings.HasSuffix(raw, "*") && strings.HasPrefix(raw, "=")
	parse := raw
	if wild {
		parse = strings.TrimSuffix(raw, "*")
	}
	a := ParsePackageAtom(parse)
	if a.String() != parse || !maskAtomComponent.MatchString(a.Category) ||
		!maskAtomComponent.MatchString(a.Name) || a.UseFlags != "" ||
		strings.ContainsAny(raw, " \t\r\n[]") {
		return false, fmt.Errorf("unsupported or malformed mask atom %q", raw)
	}
	if a.Repo != "" && !maskAtomComponent.MatchString(a.Repo) {
		return false, fmt.Errorf("unsupported repository qualifier in %q", raw)
	}
	switch a.Operator {
	case "":
		if a.Version != "" || wild {
			return false, fmt.Errorf("version without an operator in %q", raw)
		}
	case "=", "~", "<", "<=", ">", ">=":
		if !ParseGentooVersion(a.Version).IsValid || (wild && a.Operator != "=") {
			return false, fmt.Errorf("unsupported version constraint in %q", raw)
		}
	default:
		return false, fmt.Errorf("unsupported operator in %q", raw)
	}
	if a.Slot != "" {
		for part := range strings.SplitSeq(a.Slot, "/") {
			if !maskAtomComponent.MatchString(part) {
				return false, fmt.Errorf("unsupported slot constraint in %q", raw)
			}
		}
		if strings.Count(a.Slot, "/") > 1 {
			return false, fmt.Errorf("unsupported subslot constraint in %q", raw)
		}
	}
	if c.Category != a.Category || c.Package != a.Name || (a.Repo != "" && a.Repo != c.Repository) {
		return false, nil
	}
	if a.Version != "" {
		if !ParseGentooVersion(c.Version).IsValid {
			return false, fmt.Errorf("invalid candidate version %q", c.Version)
		}
		cmp := CompareVersions(c.Version, a.Version)
		match := false
		switch a.Operator {
		case "=":
			match = cmp == 0
			if wild {
				// Portage's =* matches version-part boundaries: 1* matches
				// 1.2 and 1a, but not 10. Normalize initial zeroes first.
				normalize := func(version string) string {
					version = strings.TrimLeft(version, "0")
					if version == "" || version[0] < '0' || version[0] > '9' {
						version = "0" + version
					}
					return version
				}
				version, prefix := normalize(c.Version), normalize(a.Version)
				match = strings.HasPrefix(version, prefix)
				if match && len(version) > len(prefix) {
					next, last := version[len(prefix)], prefix[len(prefix)-1]
					digit := func(c byte) bool { return c >= '0' && c <= '9' }
					match = strings.ContainsRune("._-", rune(next)) || digit(last) != digit(next)
				}
			}
		case "~":
			v1, v2 := ParseGentooVersion(c.Version), ParseGentooVersion(a.Version)
			v1.Revision, v2.Revision = 0, 0
			match = v1.String() == v2.String()
		case "<":
			match = cmp < 0
		case "<=":
			match = cmp <= 0
		case ">":
			match = cmp > 0
		case ">=":
			match = cmp >= 0
		}
		if !match {
			return false, nil
		}
	}
	if a.Slot != "" {
		if c.Slot == "" {
			return false, fmt.Errorf("slot metadata unavailable for %s", c.String())
		}
		slot := strings.SplitN(c.Slot, "/", 2)
		if strings.Contains(a.Slot, "/") {
			candidateSlot := c.Slot
			if len(slot) == 1 {
				candidateSlot += "/" + slot[0]
			}
			return a.Slot == candidateSlot, nil
		}
		return a.Slot == slot[0], nil
	}
	return true, nil
}

func matchMaskRule(r MaskRule, c MaskCandidate) (bool, error) {
	if r.Repository != "" && r.Repository != c.Repository {
		return false, nil
	}
	if r.Problem != "" {
		return false, fmt.Errorf("%s:%d: %s", r.File, r.Line, r.Problem)
	}
	return MatchMaskAtom(r.Atom, c)
}

// EvaluateMaskState evaluates a snapshot with one unmask rule excluded by its
// index, or -1 to exclude none. It never edits the input or its sources.
func EvaluateMaskState(in MaskAuditInput, candidate MaskCandidate, excluded int) MaskState {
	state := MaskState{Indeterminate: append([]string(nil), in.Problems...)}
	for _, r := range in.Masks {
		match, err := matchMaskRule(r, candidate)
		if err != nil {
			state.Indeterminate = append(state.Indeterminate, err.Error())
		} else if match {
			state.Masks = append(state.Masks, r)
		}
	}
	for i, r := range in.Unmasks {
		if i == excluded {
			continue
		}
		match, err := matchMaskRule(r, candidate)
		if err != nil {
			state.Indeterminate = append(state.Indeterminate, err.Error())
		} else if match {
			state.Unmasks = append(state.Unmasks, r)
		}
	}
	state.Masked = len(state.Masks) > 0 && len(state.Unmasks) == 0
	return state
}

// AuditUnmasks classifies each active user entry independently. Shadowed entries
// are not a simultaneous removal plan: two duplicates can each cover the other.
func AuditUnmasks(in MaskAuditInput) UnmaskAuditReport {
	report := UnmaskAuditReport{Entries: []UnmaskAuditEntry{}, Warnings: append([]string(nil), in.Problems...)}
	byPackage := make(map[string][]MaskCandidate)
	for _, c := range in.Candidates {
		key := c.Category + "/" + c.Package
		byPackage[key] = append(byPackage[key], c)
	}
	for i, r := range in.Unmasks {
		if !r.User {
			continue
		}
		entry := UnmaskAuditEntry{Source: r}
		problems := append([]string(nil), in.Problems...)
		// Validate even when the inventory is empty.
		if r.Problem != "" {
			problems = append(problems, r.Problem)
		}
		if _, err := MatchMaskAtom(r.Atom, MaskCandidate{}); err != nil {
			problems = append(problems, err.Error())
		}
		atom := ParsePackageAtom(r.Atom)
		for _, c := range byPackage[atom.Category+"/"+atom.Name] {
			match, err := matchMaskRule(r, c)
			if err != nil {
				problems = append(problems, err.Error())
				continue
			}
			if !match {
				continue
			}
			entry.Matching++
			state := EvaluateMaskState(in, c, i)
			problems = append(problems, state.Indeterminate...)
			if state.Masked {
				entry.Affected = append(entry.Affected, UnmaskAffectedVersion{Candidate: c, Masks: state.Masks})
			} else if len(state.Masks) > 0 {
				baseCover := false
				for _, other := range state.Unmasks {
					baseCover = baseCover || !other.User
				}
				if !baseCover {
					for _, other := range state.Unmasks {
						found := false
						for _, prior := range entry.CoveredBy {
							found = found || prior == other
						}
						if !found {
							entry.CoveredBy = append(entry.CoveredBy, other)
						}
					}
				}
			}
		}
		switch {
		case len(problems) > 0:
			entry.Classification = UnmaskIndeterminate
			entry.Explanation = problems[0]
			entry.Affected = nil
			entry.CoveredBy = nil
			report.Summary.Indeterminate++
		case entry.Matching == 0:
			entry.Classification = UnmaskNonMatching
			entry.Explanation = "matches no available repository versions"
			report.Summary.NonMatching++
		case len(entry.Affected) > 0:
			entry.Classification = UnmaskRequired
			entry.Explanation = fmt.Sprintf("removal would mask %d of %d matching available versions", len(entry.Affected), entry.Matching)
			report.Summary.Required++
		case len(entry.CoveredBy) > 0:
			entry.Classification = UnmaskShadowed
			entry.Explanation = "other user unmask entries cover every matching version that needs an unmask"
			report.Summary.Shadowed++
		default:
			entry.Classification = UnmaskNoChange
			entry.Explanation = fmt.Sprintf("%d matching available versions remain unmasked without this rule", entry.Matching)
			report.Summary.NoChange++
		}
		report.Entries = append(report.Entries, entry)
	}
	if report.Summary.Shadowed > 0 {
		report.Warnings = append(report.Warnings, "Entries are evaluated individually. Do not remove duplicate/shadowed entries together; re-audit after each removal.")
	}
	return report
}
