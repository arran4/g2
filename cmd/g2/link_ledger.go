package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

const maxDetailedLinkIssues = 250

// LinkIssueKind classifies integrity violations detected by LinkLedger.
type LinkIssueKind string

const (
	IssueDuplicateTarget LinkIssueKind = "duplicate_target"
	IssueCaseCollision   LinkIssueKind = "case_collision"
	IssueMissingTarget   LinkIssueKind = "missing_target"
	IssueRootEscape      LinkIssueKind = "root_escape"
	IssueMalformedURL    LinkIssueKind = "malformed_url"
	IssueFailedPage      LinkIssueKind = "failed_page"
	IssueDocumentStruct  LinkIssueKind = "document_structure"
)

// LinkIssue describes a single detected link or route integrity issue.
// Site-relative paths are recorded without exposing local OS directory prefixes.
type LinkIssue struct {
	Kind             LinkIssueKind `json:"kind"`
	Source           string        `json:"source,omitempty"`
	RawURL           string        `json:"raw_url,omitempty"`
	NormalizedTarget string        `json:"normalized_target,omitempty"`
	Element          string        `json:"element,omitempty"`
	Reason           string        `json:"reason"`
	Count            int           `json:"count,omitempty"`
}

// LinkRefSample captures the first seen origin of a unique normalized target.
type LinkRefSample struct {
	Source  string
	Element string
	RawURL  string
	Count   int
}

// LinkIntegrityReport holds bounded deterministic verification metrics and issues.
type LinkIntegrityReport struct {
	Verified           bool        `json:"verified"`
	CommittedPages     int         `json:"committed_pages"`
	CommittedAssets    int         `json:"committed_assets"`
	FailedPages        int         `json:"failed_pages"`
	SkippedPages       int         `json:"skipped_pages"`
	CheckedRefs        int         `json:"checked_refs"`
	UniqueTargets      int         `json:"unique_targets"`
	ExternalRefs       int         `json:"external_refs"`
	FragmentOnlyRefs   int         `json:"fragment_only_refs"`
	MissingCount       int         `json:"missing_count"`
	DuplicateCount     int         `json:"duplicate_count"`
	RootEscapeCount    int         `json:"root_escape_count"`
	MalformedCount     int         `json:"malformed_count"`
	CaseCollisionCount int         `json:"case_collision_count"`
	StructureCount     int         `json:"structure_count"`
	TotalIssues        int         `json:"total_issues"`
	Truncated          bool        `json:"truncated,omitempty"`
	Issues             []LinkIssue `json:"issues,omitempty"`
}

// SummaryLine formats a single-line human-readable integrity summary.
func (r *LinkIntegrityReport) SummaryLine() string {
	if r == nil {
		return "not verified"
	}
	status := "VERIFIED (0 issues)"
	if !r.Verified {
		status = fmt.Sprintf("ISSUES DETECTED (%d total: %d missing, %d duplicate, %d root-escape, %d malformed, %d case-collision, %d structure, %d failed)",
			r.TotalIssues, r.MissingCount, r.DuplicateCount, r.RootEscapeCount, r.MalformedCount, r.CaseCollisionCount, r.StructureCount, r.FailedPages)
	}
	return fmt.Sprintf("%s — pages=%d assets=%d checked_refs=%d unique_targets=%d external=%d fragments=%d",
		status, r.CommittedPages, r.CommittedAssets, r.CheckedRefs, r.UniqueTargets, r.ExternalRefs, r.FragmentOnlyRefs)
}

// JSONBytes returns sanitized machine-readable JSON for local artifacts or logs.
func (r *LinkIntegrityReport) JSONBytes() ([]byte, error) {
	if r == nil {
		return []byte("{}"), nil
	}
	return json.MarshalIndent(r, "", "  ")
}

// LinkLedger is a concurrency-safe generation-scoped ledger tracking reserved/committed
// outputs and outbound internal links across all rendered pages and feeds.
type LinkLedger struct {
	mu               sync.Mutex
	outDir           string
	reserved         map[string]bool
	committedPages   map[string]int64
	committedAssets  map[string]int64
	lowerCaseTargets map[string]string
	failedPages      map[string]string
	skippedPages     map[string]string
	expectedTargets  map[string]*LinkRefSample
	immediateIssues  []LinkIssue
	issueSeen        map[string]bool
	checkedRefs      int
	externalRefs     int
	fragmentOnlyRefs int
	duplicateCount   int
	rootEscapeCount  int
	malformedCount   int
	caseColCount     int
	structureCount   int
}

// NewLinkLedger initializes a generation-scoped LinkLedger rooted at outDir.
func NewLinkLedger(outDir string) *LinkLedger {
	return &LinkLedger{
		outDir:           filepath.Clean(outDir),
		reserved:         make(map[string]bool),
		committedPages:   make(map[string]int64),
		committedAssets:  make(map[string]int64),
		lowerCaseTargets: make(map[string]string),
		failedPages:      make(map[string]string),
		skippedPages:     make(map[string]string),
		expectedTargets:  make(map[string]*LinkRefSample),
		issueSeen:        make(map[string]bool),
	}
}

// RelPath converts an output file path into a normalized POSIX site-relative path.
func (l *LinkLedger) RelPath(fullPath string) string {
	if l == nil {
		return filepath.ToSlash(fullPath)
	}
	cleanFull := filepath.Clean(fullPath)
	if l.outDir != "" && l.outDir != "." {
		if rel, err := filepath.Rel(l.outDir, cleanFull); err == nil {
			s := filepath.ToSlash(rel)
			s = strings.TrimPrefix(s, "./")
			return s
		}
	}
	if rel, err := filepath.Rel(".", cleanFull); err == nil {
		return filepath.ToSlash(strings.TrimPrefix(rel, "./"))
	}
	return filepath.ToSlash(cleanFull)
}

func (l *LinkLedger) addImmediateIssueLocked(issue LinkIssue) {
	switch issue.Kind {
	case IssueDuplicateTarget:
		l.duplicateCount++
	case IssueRootEscape:
		l.rootEscapeCount++
	case IssueMalformedURL:
		l.malformedCount++
	case IssueCaseCollision:
		l.caseColCount++
	case IssueDocumentStruct:
		l.structureCount++
	}

	key := string(issue.Kind) + "|" + issue.Source + "|" + issue.NormalizedTarget + "|" + issue.RawURL
	if l.issueSeen[key] {
		return
	}
	l.issueSeen[key] = true
	if len(l.immediateIssues) < maxDetailedLinkIssues {
		l.immediateIssues = append(l.immediateIssues, issue)
	}
}

func normalizeCommittedRelPath(rel string) string {
	cleaned := path.Clean("/" + filepath.ToSlash(rel))
	return strings.TrimPrefix(cleaned, "/")
}

// ReservePage reserves relPath before opening an output writer.
// Returns true if this reservation is unique; false if the path collides with a prior page/asset.
func (l *LinkLedger) ReservePage(relPath string) bool {
	if l == nil {
		return true
	}
	norm := normalizeCommittedRelPath(relPath)
	lower := strings.ToLower(norm)

	l.mu.Lock()
	defer l.mu.Unlock()

	if prev, exists := l.lowerCaseTargets[lower]; exists && prev != norm {
		l.addImmediateIssueLocked(LinkIssue{
			Kind:             IssueCaseCollision,
			Source:           norm,
			NormalizedTarget: norm,
			Reason:           fmt.Sprintf("case collision with already registered route %q", prev),
		})
	} else {
		l.lowerCaseTargets[lower] = norm
	}

	if l.reserved[norm] {
		l.addImmediateIssueLocked(LinkIssue{
			Kind:             IssueDuplicateTarget,
			Source:           norm,
			NormalizedTarget: norm,
			Reason:           "duplicate route write/overwrite for identical output path",
		})
		return false
	}
	l.reserved[norm] = true
	return true
}

// CommitPage records a successfully rendered and closed page.
func (l *LinkLedger) CommitPage(relPath string, size int64) {
	if l == nil {
		return
	}
	norm := normalizeCommittedRelPath(relPath)
	l.mu.Lock()
	l.reserved[norm] = true
	l.committedPages[norm] = size
	l.mu.Unlock()
}

// FailPage marks a reserved or attempted page as failed with cause.
func (l *LinkLedger) FailPage(relPath string, err error) {
	if l == nil {
		return
	}
	norm := normalizeCommittedRelPath(relPath)
	reason := "write/render failure"
	if err != nil {
		reason = err.Error()
	}
	l.mu.Lock()
	l.failedPages[norm] = reason
	l.mu.Unlock()
}

// SkipPage explicitly records an intentionally skipped route with cause.
func (l *LinkLedger) SkipPage(relPath string, reason string) {
	if l == nil {
		return
	}
	norm := normalizeCommittedRelPath(relPath)
	l.mu.Lock()
	l.skippedPages[norm] = reason
	l.mu.Unlock()
}

// CommitAsset registers a non-HTML generated file (JS, JSON, feed, etc.).
func (l *LinkLedger) CommitAsset(relPath string, size int64) {
	if l == nil {
		return
	}
	norm := normalizeCommittedRelPath(relPath)
	lower := strings.ToLower(norm)
	l.mu.Lock()
	defer l.mu.Unlock()
	if prev, exists := l.lowerCaseTargets[lower]; exists && prev != norm {
		l.addImmediateIssueLocked(LinkIssue{
			Kind:             IssueCaseCollision,
			Source:           norm,
			NormalizedTarget: norm,
			Reason:           fmt.Sprintf("case collision with already registered asset %q", prev),
		})
	} else {
		l.lowerCaseTargets[lower] = norm
	}
	if _, dupAsset := l.committedAssets[norm]; dupAsset || l.reserved[norm] {
		l.addImmediateIssueLocked(LinkIssue{
			Kind:             IssueDuplicateTarget,
			Source:           norm,
			NormalizedTarget: norm,
			Reason:           "duplicate output write for identical asset path",
		})
	}
	l.committedAssets[norm] = size
}

// ScanHTML performs a single streaming HTML tokenization pass over htmlBytes to record
// outbound references and verify basic single-document root structure.
func (l *LinkLedger) ScanHTML(sourceRelPath string, htmlBytes []byte) {
	if l == nil || len(htmlBytes) == 0 {
		return
	}
	normSource := normalizeCommittedRelPath(sourceRelPath)
	z := html.NewTokenizer(bytes.NewReader(htmlBytes))

	var refs [][2]string
	doctypeCount := 0
	htmlStartCount := 0
	headStartCount := 0
	bodyStartCount := 0

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			goto done
		case html.DoctypeToken:
			doctypeCount++
		case html.StartTagToken, html.SelfClosingTagToken:
			nameBytes, hasAttr := z.TagName()
			tagName := string(nameBytes)
			switch tagName {
			case "html":
				if tt == html.StartTagToken {
					htmlStartCount++
				}
			case "head":
				if tt == html.StartTagToken {
					headStartCount++
				}
			case "body":
				if tt == html.StartTagToken {
					bodyStartCount++
				}
			}
			if !hasAttr {
				continue
			}
			var isMetaRefresh bool
			var metaContent string
			var attrTarget string
			for {
				k, v, more := z.TagAttr()
				key := string(k)
				val := string(v)
				switch tagName {
				case "a":
					if key == "href" {
						attrTarget = val
					}
				case "link":
					if key == "href" {
						attrTarget = val
					}
				case "script", "img":
					if key == "src" {
						attrTarget = val
					}
				case "form":
					if key == "action" {
						attrTarget = val
					}
				case "meta":
					if key == "http-equiv" && strings.EqualFold(strings.TrimSpace(val), "refresh") {
						isMetaRefresh = true
					} else if key == "content" {
						metaContent = val
					}
				}
				if !more {
					break
				}
			}
			if attrTarget != "" || (tagName == "a" || tagName == "link") {
				if attrTarget != "" {
					refs = append(refs, [2]string{tagName, attrTarget})
				}
			}
			if isMetaRefresh && metaContent != "" {
				if target := parseLedgerMetaRefreshTarget(metaContent); target != "" {
					refs = append(refs, [2]string{"meta[refresh]", target})
				}
			}
		}
	}

done:
	l.mu.Lock()
	defer l.mu.Unlock()

	if doctypeCount != 1 || htmlStartCount != 1 || headStartCount != 1 || bodyStartCount != 1 {
		l.addImmediateIssueLocked(LinkIssue{
			Kind:             IssueDocumentStruct,
			Source:           normSource,
			NormalizedTarget: normSource,
			Element:          "document",
			Reason:           fmt.Sprintf("invalid HTML document root counts (doctype=%d html=%d head=%d body=%d)", doctypeCount, htmlStartCount, headStartCount, bodyStartCount),
		})
	}

	for _, ref := range refs {
		l.recordRefLocked(normSource, ref[0], ref[1])
	}
}

func parseLedgerMetaRefreshTarget(content string) string {
	lower := strings.ToLower(content)
	idx := strings.Index(lower, "url=")
	if idx == -1 {
		return ""
	}
	target := strings.TrimSpace(content[idx+4:])
	target = strings.Trim(target, `"'`)
	return target
}

// RecordRef validates and records an outbound reference from sourceRelPath.
func (l *LinkLedger) RecordRef(sourceRelPath, element, rawURL string) {
	if l == nil {
		return
	}
	normSource := normalizeCommittedRelPath(sourceRelPath)
	l.mu.Lock()
	l.recordRefLocked(normSource, element, rawURL)
	l.mu.Unlock()
}

func (l *LinkLedger) recordRefLocked(normSource, element, rawURL string) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return
	}
	if strings.ContainsAny(trimmed, "\r\n\x00") {
		l.checkedRefs++
		l.addImmediateIssueLocked(LinkIssue{
			Kind:    IssueMalformedURL,
			Source:  normSource,
			RawURL:  rawURL,
			Element: element,
			Reason:  "URL contains control characters",
		})
		return
	}
	if strings.HasPrefix(trimmed, "#") {
		l.fragmentOnlyRefs++
		l.checkedRefs++
		return
	}
	if strings.HasPrefix(trimmed, "//") {
		l.externalRefs++
		return
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		l.checkedRefs++
		l.addImmediateIssueLocked(LinkIssue{
			Kind:    IssueMalformedURL,
			Source:  normSource,
			RawURL:  rawURL,
			Element: element,
			Reason:  fmt.Sprintf("malformed URL: %v", err),
		})
		return
	}

	if u.Scheme != "" {
		scheme := strings.ToLower(u.Scheme)
		switch scheme {
		case "http", "https", "mailto", "tel", "irc", "ircs", "ftp", "ftps", "git", "ssh":
			l.externalRefs++
		case "javascript", "data", "vbscript":
			l.checkedRefs++
			l.addImmediateIssueLocked(LinkIssue{
				Kind:    IssueMalformedURL,
				Source:  normSource,
				RawURL:  rawURL,
				Element: element,
				Reason:  fmt.Sprintf("disallowed URL scheme %q", scheme),
			})
		default:
			l.externalRefs++
		}
		return
	}

	l.checkedRefs++
	if u.Host != "" {
		l.externalRefs++
		return
	}

	rawPath := u.Path
	if strings.HasPrefix(rawPath, "/") {
		l.addImmediateIssueLocked(LinkIssue{
			Kind:             IssueRootEscape,
			Source:           normSource,
			RawURL:           rawURL,
			NormalizedTarget: rawPath,
			Element:          element,
			Reason:           "root-relative path starts with '/' and escapes subpath deployments",
		})
		return
	}

	if rawPath == "" {
		// Query/fragment relative to current page (e.g., ?q=foo) resolves to normSource itself.
		l.addExpectedTargetLocked(normSource, normSource, element, rawURL)
		return
	}

	sourceDir := path.Dir(normSource)
	var segs []string
	if sourceDir != "." && sourceDir != "" {
		segs = strings.Split(sourceDir, "/")
	}

	parts := strings.Split(rawPath, "/")
	for _, p := range parts {
		switch p {
		case "", ".":
			continue
		case "..":
			if len(segs) == 0 {
				l.addImmediateIssueLocked(LinkIssue{
					Kind:             IssueRootEscape,
					Source:           normSource,
					RawURL:           rawURL,
					NormalizedTarget: "../" + strings.Join(parts, "/"),
					Element:          element,
					Reason:           "relative path traverses above generated site root",
				})
				return
			}
			segs = segs[:len(segs)-1]
		default:
			segs = append(segs, p)
		}
	}

	targetPath := strings.Join(segs, "/")
	lastPart := ""
	if len(parts) > 0 {
		lastPart = parts[len(parts)-1]
	}
	isDirTarget := targetPath == "" || strings.HasSuffix(rawPath, "/") || lastPart == "." || lastPart == ".."
	if isDirTarget {
		if targetPath == "" {
			targetPath = "index.html"
		} else {
			targetPath = targetPath + "/index.html"
		}
	}

	l.addExpectedTargetLocked(targetPath, normSource, element, rawURL)
}

func (l *LinkLedger) addExpectedTargetLocked(targetPath, source, element, rawURL string) {
	if existing, ok := l.expectedTargets[targetPath]; ok {
		existing.Count++
		return
	}
	l.expectedTargets[targetPath] = &LinkRefSample{
		Source:  source,
		Element: element,
		RawURL:  rawURL,
		Count:   1,
	}
}

// Verify reconciles all expected internal destinations against committed pages and assets
// in O(pages + assets + unique targets) time and returns a deterministic report.
func (l *LinkLedger) Verify() *LinkIntegrityReport {
	return l.VerifyAssumingCommitted()
}

// VerifyAssumingCommitted reconciles expected internal destinations assuming pendingRelPaths
// will be committed cleanly (for example rendering /stats/ last while displaying link counts).
func (l *LinkLedger) VerifyAssumingCommitted(pendingRelPaths ...string) *LinkIntegrityReport {
	if l == nil {
		return &LinkIntegrityReport{Verified: true}
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	pendingSet := make(map[string]bool, len(pendingRelPaths))
	for _, p := range pendingRelPaths {
		pendingSet[normalizeCommittedRelPath(p)] = true
	}

	report := &LinkIntegrityReport{
		CommittedPages:     len(l.committedPages),
		CommittedAssets:    len(l.committedAssets),
		FailedPages:        len(l.failedPages),
		SkippedPages:       len(l.skippedPages),
		CheckedRefs:        l.checkedRefs,
		UniqueTargets:      len(l.expectedTargets),
		ExternalRefs:       l.externalRefs,
		FragmentOnlyRefs:   l.fragmentOnlyRefs,
		DuplicateCount:     l.duplicateCount,
		RootEscapeCount:    l.rootEscapeCount,
		MalformedCount:     l.malformedCount,
		CaseCollisionCount: l.caseColCount,
		StructureCount:     l.structureCount,
	}

	var issues []LinkIssue
	issues = append(issues, l.immediateIssues...)

	for failedPath, cause := range l.failedPages {
		if len(issues) < maxDetailedLinkIssues {
			issues = append(issues, LinkIssue{
				Kind:             IssueFailedPage,
				Source:           failedPath,
				NormalizedTarget: failedPath,
				Reason:           cause,
			})
		} else {
			report.Truncated = true
		}
	}

	missingCount := 0
	for target, sample := range l.expectedTargets {
		if pendingSet[target] {
			continue
		}
		if _, ok := l.committedPages[target]; ok {
			continue
		}
		if _, ok := l.committedAssets[target]; ok {
			continue
		}
		if !strings.HasSuffix(target, ".html") && !strings.HasSuffix(target, ".js") &&
			!strings.HasSuffix(target, ".json") && !strings.HasSuffix(target, ".rss") &&
			!strings.HasSuffix(target, ".atom") && !strings.HasSuffix(target, ".css") {
			if pendingSet[target+"/index.html"] {
				continue
			}
			if _, ok := l.committedPages[target+"/index.html"]; ok {
				continue
			}
		}
		missingCount++
		if len(issues) < maxDetailedLinkIssues {
			issues = append(issues, LinkIssue{
				Kind:             IssueMissingTarget,
				Source:           sample.Source,
				RawURL:           sample.RawURL,
				NormalizedTarget: target,
				Element:          sample.Element,
				Count:            sample.Count,
				Reason:           "referenced internal page or asset was not committed",
			})
		} else {
			report.Truncated = true
		}
	}
	report.MissingCount = missingCount

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Kind != issues[j].Kind {
			return issues[i].Kind < issues[j].Kind
		}
		if issues[i].NormalizedTarget != issues[j].NormalizedTarget {
			return issues[i].NormalizedTarget < issues[j].NormalizedTarget
		}
		if issues[i].Source != issues[j].Source {
			return issues[i].Source < issues[j].Source
		}
		return issues[i].RawURL < issues[j].RawURL
	})

	if len(l.immediateIssues) >= maxDetailedLinkIssues {
		report.Truncated = true
	}

	report.Issues = issues
	report.TotalIssues = report.MissingCount + report.DuplicateCount + report.RootEscapeCount +
		report.MalformedCount + report.CaseCollisionCount + report.StructureCount + report.FailedPages
	report.Verified = report.TotalIssues == 0
	return report
}
