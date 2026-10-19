package lints

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arran4/g2"
)

// RepoLintContext holds pre-parsed repository-level metadata and virtual filesystem
// references to avoid redundant disk reads during per-package linting.
type RepoLintContext struct {
	FS             fs.FS
	RepoDirInFS    string
	QAPolicy       *g2.QAPolicy
	LayoutConf     *g2.LayoutConf
	UseExpandDescs map[string]*g2.UseExpandDesc
	ArchList       *g2.ArchList
	HasRepoName    bool
	HasMD5CacheDir bool

	eclassMu  sync.RWMutex
	eclassMD5 map[string]string
}

// HashEclass computes and caches the MD5 hash of an eclass from the RepoLintContext's FS.
func (c *RepoLintContext) HashEclass(eclassPath string) (string, error) {
	if c == nil || c.FS == nil {
		return "", fmt.Errorf("no filesystem in RepoLintContext")
	}
	ePath := filepath.ToSlash(eclassPath)
	relPath := ePath
	repoDirSlash := filepath.ToSlash(c.RepoDirInFS)
	if repoDirSlash != "" && repoDirSlash != "." && strings.HasPrefix(ePath, repoDirSlash+"/") {
		relPath = strings.TrimPrefix(ePath, repoDirSlash+"/")
	} else if strings.HasPrefix(ePath, "./") {
		relPath = strings.TrimPrefix(ePath, "./")
	}

	c.eclassMu.RLock()
	if c.eclassMD5 != nil {
		if sum, ok := c.eclassMD5[relPath]; ok {
			c.eclassMu.RUnlock()
			return sum, nil
		}
	}
	c.eclassMu.RUnlock()

	data, err := fs.ReadFile(c.FS, relPath)
	if err != nil {
		return "", fmt.Errorf("reading eclass %s: %w", relPath, err)
	}
	sum := fmt.Sprintf("%x", md5.Sum(data))

	c.eclassMu.Lock()
	if c.eclassMD5 == nil {
		c.eclassMD5 = make(map[string]string)
	}
	c.eclassMD5[relPath] = sum
	c.eclassMu.Unlock()
	return sum, nil
}

var (
	repoContextsMu sync.RWMutex
	repoContexts   = make(map[string]*RepoLintContext)
)

// SetRepoContext registers a RepoLintContext for the given key.
func SetRepoContext(key string, ctx *RepoLintContext) {
	repoContextsMu.Lock()
	repoContexts[key] = ctx
	repoContextsMu.Unlock()
}

// ClearRepoContext removes a registered RepoLintContext for the given key.
func ClearRepoContext(key string) {
	repoContextsMu.Lock()
	delete(repoContexts, key)
	repoContextsMu.Unlock()
}

// GetRepoContext retrieves a registered RepoLintContext for the given key, or nil if none is registered.
func GetRepoContext(key string) *RepoLintContext {
	repoContextsMu.RLock()
	ctx := repoContexts[key]
	repoContextsMu.RUnlock()
	return ctx
}

type Severity string

const (
	SeverityError   Severity = "Error"
	SeverityWarning Severity = "Warning"
	SeverityNotice  Severity = "Notice"
	SeverityInfo    Severity = "Info"
)

type Source string

const (
	SourceG2       Source = "g2"
	SourcePkgcheck Source = "pkgcheck"
	SourceQA       Source = "qa"
)

type RuleReference struct {
	URL   string `json:"url"`
	Label string `json:"label,omitempty"`
}

type RuleMetadata struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	References  []RuleReference `json:"references,omitempty"`
	Severity    Severity        `json:"severity"`
	Source      Source          `json:"source"`
	Tags        []string        `json:"tags,omitempty"`
}

// UnmarshalJSON provides backward-compatible unmarshaling for legacy "url" and "urls" fields.
func (r *RuleMetadata) UnmarshalJSON(data []byte) error {
	type Alias RuleMetadata
	aux := struct {
		*Alias
		LegacyURL  string   `json:"url,omitempty"`
		LegacyURLs []string `json:"urls,omitempty"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(r.References) == 0 {
		if aux.LegacyURL != "" {
			r.References = []RuleReference{{URL: aux.LegacyURL}}
		} else if len(aux.LegacyURLs) > 0 {
			for _, u := range aux.LegacyURLs {
				if u != "" {
					r.References = append(r.References, RuleReference{URL: u})
				}
			}
		}
	}
	return nil
}

type LintResult struct {
	RuleMetadata RuleMetadata `json:"rule"`
	Message      string       `json:"message"`
	Package      string       `json:"package,omitempty"`
	File         string       `json:"file,omitempty"`
	Line         int          `json:"line,omitempty"`
}

func (lr LintResult) String() string {
	return lr.Message
}

type LintRule interface {
	Lint(repoDir string, pkg *g2.PackageData) []LintResult
}

type RepoLintRule interface {
	LintRepo(repoDir string, site *g2.SiteData) []LintResult
}

var repoLintRules []RepoLintRule

func RegisterRepoLintRule(rule RepoLintRule) {
	repoLintRules = append(repoLintRules, rule)
}

func PerformRepoLintingResults(repoDir string, site *g2.SiteData) []LintResult {
	var results []LintResult
	for _, rule := range repoLintRules {
		results = append(results, rule.LintRepo(repoDir, site)...)
	}
	return results
}

type QAAwareLintRule interface {
	LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []LintResult
}

type EclassLintRule interface {
	Lint(repoDir string, eclass *g2.Ebuild) []LintResult
}

type QAAwareEclassLintRule interface {
	LintWithQA(repoDir string, eclass *g2.Ebuild, qa *g2.QAPolicy) []LintResult
}

var lintRules []LintRule
var eclassLintRules []EclassLintRule

func RegisterLintRule(rule LintRule) {
	lintRules = append(lintRules, rule)
}

func RegisterEclassLintRule(rule EclassLintRule) {
	eclassLintRules = append(eclassLintRules, rule)
}

// RuleSetManagedLintRule is an opt-in interface for lint rules whose enablement and
// effective severity are managed by a RuleSet.
type RuleSetManagedLintRule interface {
	MetadataAwareLintRule
	RuleSetManaged()
}

func PerformLintingResults(repoDir string, pkg *g2.PackageData) []LintResult {
	results, err := PerformLintingResultsWithRuleSet(repoDir, pkg, "default")
	if err != nil {
		panic(fmt.Sprintf("failed to perform default linting: %v", err))
	}
	return results
}

func PerformLintingResultsWithRuleSet(repoDir string, pkg *g2.PackageData, ruleSetID string) ([]LintResult, error) {
	rs, ok := GetRuleSet(ruleSetID)
	if !ok || rs == nil {
		return nil, fmt.Errorf("unknown ruleset: %s", ruleSetID)
	}

	var results []LintResult

	// Try to load QA Policy from RepoLintContext first, falling back to disk
	var qa *g2.QAPolicy
	if ctx := GetRepoContext(repoDir); ctx != nil {
		qa = ctx.QAPolicy
	} else {
		qaPolicyPath := filepath.Join(repoDir, "metadata", "qa-policy.conf")
		qa, _ = g2.ParseQAPolicy(qaPolicyPath)
	}

	for _, rule := range lintRules {
		if managedRule, ok := rule.(RuleSetManagedLintRule); ok {
			meta := managedRule.Metadata()
			entry, found := rs.Rules[meta.ID]
			if !found || !entry.Enabled {
				continue
			}

			var ruleResults []LintResult
			if qaRule, ok := rule.(QAAwareLintRule); ok {
				ruleResults = qaRule.LintWithQA(repoDir, pkg, qa)
			} else {
				ruleResults = rule.Lint(repoDir, pkg)
			}

			for i := range ruleResults {
				if entry.Severity != "" {
					ruleResults[i].RuleMetadata.Severity = entry.Severity
				}
			}
			results = append(results, ruleResults...)
		} else {
			if qaRule, ok := rule.(QAAwareLintRule); ok {
				results = append(results, qaRule.LintWithQA(repoDir, pkg, qa)...)
			} else {
				results = append(results, rule.Lint(repoDir, pkg)...)
			}
		}
	}
	return results, nil
}

func PerformLinting(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string
	for _, res := range PerformLintingResults(repoDir, pkg) {
		warnings = append(warnings, res.Message)
	}
	return warnings
}

func PerformLintingWithRuleSet(repoDir string, pkg *g2.PackageData, ruleSetID string) ([]string, error) {
	results, err := PerformLintingResultsWithRuleSet(repoDir, pkg, ruleSetID)
	if err != nil {
		return nil, err
	}
	var warnings []string
	for _, res := range results {
		warnings = append(warnings, res.Message)
	}
	return warnings, nil
}

func PerformEclassLintingResults(repoDir string, eclass *g2.Ebuild) []LintResult {
	var results []LintResult

	// Try to load QA Policy
	var qa *g2.QAPolicy
	if ctx := GetRepoContext(repoDir); ctx != nil {
		qa = ctx.QAPolicy
	} else {
		qaPolicyPath := filepath.Join(repoDir, "metadata", "qa-policy.conf")
		qa, _ = g2.ParseQAPolicy(qaPolicyPath)
	}

	for _, rule := range eclassLintRules {
		if qaRule, ok := rule.(QAAwareEclassLintRule); ok {
			results = append(results, qaRule.LintWithQA(repoDir, eclass, qa)...)
		} else {
			results = append(results, rule.Lint(repoDir, eclass)...)
		}
	}
	return results
}

type MetadataAwareLintRule interface {
	LintRule
	Metadata() RuleMetadata
}

var registeredMetadata []RuleMetadata

func RegisterRuleMetadata(meta RuleMetadata) {
	registeredMetadata = append(registeredMetadata, meta)
}

func GetAllRules() []RuleMetadata {
	return registeredMetadata
}
