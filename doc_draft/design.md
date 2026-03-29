# Lint Framework Design

## Goals
- Provide a robust, CI-friendly, and maintainable linting framework for Gentoo repositories and overlays.
- Detect actual correctness and standard issues, policy/QA issues, and site-facing metadata issues.
- Design lints specifically with `g2`'s capabilities and output representations in mind.
- Offer clear separation between `pkgcheck`-backed lints (future) and native `g2` lints.
- Supply structured lint metadata, severity levels, and actionable remediation guidance.

## Non-Goals
- Complete parity with `pkgcheck` (we should defer to upstream rather than duplicating complex checks, except where g2 has superior context for site generation).
- Auto-fixing logic beyond simple metadata generation (already covered by `g2 use discover` etc).
- Writing a full bash parser (we use g2's existing naive `EbuildParser` unless we integrate with a true bash parser or `pkgdev`).

## Architecture
- `lints/registry.go` already exists and defines `LintRule` and `QAAwareLintRule`.
- We will expand the registry to include structured `RuleMetadata` (ID, Title, Description, References).
- Provide a CLI flag system (e.g., `g2 lint --format=json`, `--severity=warning`, `--only-source=g2`).
- Provide an interface for wrapping `pkgcheck` calls and mapping them to `g2` warnings.

## Rule Taxonomy

### A. Repository layout and structure
- `LayoutConfPresent`: Missing `layout.conf`
- `CategorySanity`: Invalid category layout (already exists)

### B. Ebuild metadata correctness
- `EapiDeprecated`: Detect old EAPIs (already exists)
- `LicenseSanity`: Detect malformed LICENSE strings (already exists)
- `SubshellFunction`: Detect subshell functions in ebuilds (already exists)
- `MissingKeyword`: Ebuild is missing keywords.
- `InvalidSlot`: Invalid SLOT definition.

### C. metadata.xml quality
- `MetadataMissing`: Missing `metadata.xml` (already exists partially)
- `MaintainerMissing`: Missing maintainer (already exists)
- `IuseDocumented`: Missing IUSE descriptions in metadata.xml (already exists)

### D. Manifest and distfile checks
- `Md5CacheMissing`: Cache missing (already exists)
- `ManifestMissing`: Missing Manifest.

### F. Site-facing/data-quality checks
- `MissingDescription`: `DESCRIPTION` is empty or too short.
- `MissingHomepage`: `HOMEPAGE` is missing or invalid.

## Output Format
- Terminal (default)
- JSON (`--format=json`)

## Integration Strategy
- We will define a `LintResult` struct containing the message, severity, and rule metadata.
- `PerformLinting` will be updated to return `[]LintResult` instead of `[]string` to support JSON.
## Linting Strategy

We will update `lints/registry.go` to include RuleMetadata, Severity, and Source.

```go
type Severity string
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type Source string
const (
	SourceG2 Source = "g2"
	SourcePkgcheck Source = "pkgcheck"
)

type RuleMetadata struct {
	ID          string
	Title       string
	Description string
	URL         string
	Severity    Severity
	Source      Source
	Tags        []string
}

type LintResult struct {
	RuleMetadata RuleMetadata
	Message      string
	Package      string
	File         string
	Line         int
}
```

Since we don't want to overengineer, we can maintain the simple string array return interface `[]string` for backwards compatibility in html templates (`.LintWarnings`), but we will also expose a function that returns full `[]LintResult`. Actually, `cmd/g2/lint.go` should just call the function that returns `[]LintResult` directly so that we can support structured output formatting like `--format=json`.

For site templates, we can just join or map `[]LintResult` into strings for `LintWarnings`.

## Implementation Tasks

1. Update `lints/registry.go` to include the new metadata structures and functions.
2. Refactor existing lint rules (`lints/ebuild/*`, `lints/md5cache/*`, `lints/metadata/*`) to implement the new interface.
3. Update `cmd/g2/lint.go` to support `--format=json` and other flags, parsing the new `LintResult`.
4. Update `cmd/g2/site.go` to convert `[]LintResult` into `[]string` for the HTML templates, or update `PackageData.LintWarnings` to be `[]LintResult` and adjust templates.
5. Create new lints:
   - `MissingDescription`: Flag packages with poor/missing `DESCRIPTION`.
   - `MissingHomepage`: Flag packages with missing `HOMEPAGE`.
   - `ManifestMissing`: Flag missing Manifest files.
   - `InvalidSlot`: Check for bad SLOTs.
   - `LayoutConfPresent`: Check for missing layout.conf.
6. Write unit, fixture, and snapshot tests.

### Detailed Implementation Details

1. **`lints/registry.go` modifications**:
```go
package lints

import (
	"path/filepath"
	"github.com/arran4/g2"
)

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
)

type RuleMetadata struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	URL         string   `json:"url,omitempty"`
	Severity    Severity `json:"severity"`
	Source      Source   `json:"source"`
	Tags        []string `json:"tags,omitempty"`
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

type QAAwareLintRule interface {
	LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []LintResult
}

// Global registry
var lintRules []LintRule

func RegisterLintRule(rule LintRule) {
	lintRules = append(lintRules, rule)
}

func PerformLintingResults(repoDir string, pkg *g2.PackageData) []LintResult {
	var results []LintResult

	qaPolicyPath := filepath.Join(repoDir, "metadata", "qa-policy.conf")
	qa, _ := g2.ParseQAPolicy(qaPolicyPath)

	for _, rule := range lintRules {
		if qaRule, ok := rule.(QAAwareLintRule); ok {
			results = append(results, qaRule.LintWithQA(repoDir, pkg, qa)...)
		} else {
			results = append(results, rule.Lint(repoDir, pkg)...)
		}
	}
	return results
}

// Kept for backwards compatibility and ease of use in templates where a flat string slice is needed
func PerformLinting(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string
	for _, res := range PerformLintingResults(repoDir, pkg) {
		warnings = append(warnings, res.Message) // Or potentially a formatted string combining severity and message
	}
	return warnings
}
```

2. **Refactor existing rules**:
   - `lints/ebuild/category_sanity.go`
   - `lints/ebuild/eapi_deprecated.go`
   - `lints/ebuild/iuse_documented.go`
   - `lints/ebuild/layout_conf_rules.go`
   - `lints/ebuild/license_sanity.go`
   - `lints/ebuild/subshell_function.go`
   - `lints/md5cache/md5cache_missing.go`
   - `lints/metadata/maintainer_missing.go`
   - `lints/metadata/manifest.go`
   - `lints/metadata/metadata_missing_invalid.go`

3. **Create new lints**:
   - `MissingDescription`: If `DESCRIPTION` length < 10, error.
   - `MissingHomepage`: If `HOMEPAGE` is empty, error.
   - `InvalidSlot`: Check for bad characters in `SLOT`. (Regex: `^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)?$`)
   - `MissingKeyword`: Ensure that packages that are not virtual have KEYWORDS.
   - `LayoutConfPresent`: `metadata/layout.conf` does not exist or cannot be parsed. (This is a repo-level rule, but can be evaluated lazily or passed through).

4. **Update `cmd/g2/lint.go`**:
   Add flags:
   - `-format`: json or text
   - `-severity`: limit by severity
   - `-only-source`: filter by g2 or pkgcheck (future)
   - `-only-tag`: filter by tag

5. **Update tests**:
   Update existing tests to reflect the new `LintResult` return values.
   Add tests for the new lint rules.
