package news

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var (
	newsDirNameRegex  = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-([a-z0-9+_-]{1,20})$`)
	newsFileNameRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-([a-z0-9+_-]{1,20})\.([a-zA-Z0-9-]+)\.txt$`)
)

var ruleNewsValidity = lints.RuleMetadata{
	ID:          "NewsValidity",
	Title:       "News Item Validity",
	Description: "Checks if news items have valid formats and structure.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"news"},
}

func init() {
	lints.RegisterRuleMetadata(ruleNewsValidity)
	lints.RegisterLintRule(&NewsValidityLintRule{
		checkedRepos: make(map[string]bool),
	})
}

type NewsValidityLintRule struct {
	mu           sync.RWMutex
	checkedRepos map[string]bool

	// optional injection for testing
	fs fs.FS
}

// WithFS allows injecting a custom filesystem for testing.
func WithFS(fsys fs.FS) func(*NewsValidityLintRule) {
	return func(r *NewsValidityLintRule) {
		r.fs = fsys
	}
}

// NewNewsValidityLintRule creates a new rule instance with optional configuration.
func NewNewsValidityLintRule(opts ...func(*NewsValidityLintRule)) *NewsValidityLintRule {
	rule := &NewsValidityLintRule{
		checkedRepos: make(map[string]bool),
	}
	for _, opt := range opts {
		opt(rule)
	}
	return rule
}

// SkipForSiteGen disables this rule when it is already pre-calculated manually by site.go.
var SkipForSiteGen bool

func (r *NewsValidityLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *NewsValidityLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	if SkipForSiteGen {
		return nil
	}

	r.mu.RLock()
	done := r.checkedRepos[repoDir]
	r.mu.RUnlock()

	if done {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checkedRepos[repoDir] {
		return nil
	}
	r.checkedRepos[repoDir] = true

	var results []lints.LintResult

	newsDir := filepath.Join(repoDir, "metadata", "news")

	var entries []fs.DirEntry
	var err error

	// Determine the filesystem to use
	if r.fs != nil {
		entries, err = fs.ReadDir(r.fs, "metadata/news")
	} else {
		entries, err = os.ReadDir(newsDir)
	}

	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()

		dirMatch := newsDirNameRegex.FindStringSubmatch(dirName)
		if dirMatch == nil {
			res := lints.LintResult{
				RuleMetadata: ruleNewsValidity,
				Message:      fmt.Sprintf("[%s] Invalid news directory name format: '%s'", lints.SeverityError, dirName),
				File:         filepath.Join("metadata", "news", dirName),
				Package:      "metadata/news/" + dirName,
			}
			res.RuleMetadata.Severity = lints.SeverityError
			results = append(results, res)
		}

		var txtFiles []string

		if r.fs != nil {
			matches, err := fs.Glob(r.fs, filepath.Join("metadata", "news", dirName, "*.txt"))
			if err == nil {
				txtFiles = matches
			}
		} else {
			matches, err := filepath.Glob(filepath.Join(newsDir, dirName, "*.txt"))
			if err == nil {
				txtFiles = matches
			}
		}

		if len(txtFiles) == 0 {
			res := lints.LintResult{
				RuleMetadata: ruleNewsValidity,
				Message:      fmt.Sprintf("[%s] News directory '%s' has no .txt files", lints.SeverityError, dirName),
				File:         filepath.Join("metadata", "news", dirName),
			}
			res.RuleMetadata.Severity = lints.SeverityError
			results = append(results, res)
			continue
		}

		for _, txtFile := range txtFiles {
			var content []byte
			if r.fs != nil {
				content, err = fs.ReadFile(r.fs, filepath.ToSlash(txtFile))
			} else {
				content, err = os.ReadFile(txtFile)
			}

			if err != nil {
				continue
			}

			relPath, _ := filepath.Rel(repoDir, txtFile)
			if r.fs != nil {
				// in test mode relPath is exactly the matched file path because repoDir is essentially root
				relPath = txtFile
			}

			fileName := filepath.Base(txtFile)
			fileMatch := newsFileNameRegex.FindStringSubmatch(fileName)
			if fileMatch == nil {
				res := lints.LintResult{
					RuleMetadata: ruleNewsValidity,
					Message:      fmt.Sprintf("[%s] Invalid news file name format: '%s'", lints.SeverityError, fileName),
					File:         relPath,
					Package:      "metadata/news/" + dirName,
				}
				res.RuleMetadata.Severity = lints.SeverityError
				results = append(results, res)
			} else if dirMatch != nil && (fileMatch[1] != dirMatch[1] || fileMatch[2] != dirMatch[2]) {
				res := lints.LintResult{
					RuleMetadata: ruleNewsValidity,
					Message:      fmt.Sprintf("[%s] News file prefix '%s-%s' does not match directory name '%s'", lints.SeverityError, fileMatch[1], fileMatch[2], dirName),
					File:         relPath,
					Package:      "metadata/news/" + dirName,
				}
				res.RuleMetadata.Severity = lints.SeverityError
				results = append(results, res)
			}

			res := r.lintNewsItem(string(content), relPath)
			// Associate package with the result so that we can distinguish this rule
			for i := range res {
				res[i].Package = "metadata/news/" + dirName
			}
			results = append(results, res...)
		}
	}

	return results
}

type NewsItemState struct {
	Headers map[string]string
}

type HeaderContext struct {
	Key     string
	Value   string
	RelPath string
	LineNum int
}

type HeaderValidator func(ctx *HeaderContext, state *NewsItemState) []lints.LintResult

type PostValidator func(state *NewsItemState, relPath string) []lints.LintResult

func checkEmailFormat(val string) bool {
	// Simple manual check for "^.+ <.+@.+>$" logic to avoid complex regex as per guidelines
	startBracket := strings.Index(val, "<")
	endBracket := strings.LastIndex(val, ">")
	if startBracket == -1 || endBracket == -1 || startBracket >= endBracket {
		return false
	}

	namePart := strings.TrimSpace(val[:startBracket])
	if len(namePart) == 0 {
		return false
	}

	emailPart := val[startBracket+1 : endBracket]
	atIndex := strings.Index(emailPart, "@")
	if atIndex == -1 || atIndex == 0 || atIndex == len(emailPart)-1 {
		return false
	}

	if endBracket != len(val)-1 {
		return false
	}

	return true
}

func validateTitleLength(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	if len(ctx.Value) == 0 {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Title cannot be empty", lints.SeverityError), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	} else if len(ctx.Value) > 50 {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Title exceeds maximum length of 50 characters", lints.SeverityError), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	return nil
}

func validateEmailFormat(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	if !checkEmailFormat(ctx.Value) {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Invalid %s format, expected 'Name <email@domain.com>': '%s'", lints.SeverityError, ctx.Key, ctx.Value), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	return nil
}

func validatePostedDate(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	_, err := time.Parse("2006-01-02", ctx.Value)
	if err != nil {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Invalid Posted date format, expected YYYY-MM-DD: '%s'", lints.SeverityError, ctx.Value), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	return nil
}

func validateRevisionInteger(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	if len(ctx.Value) == 0 {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Revision cannot be empty", lints.SeverityError), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	_, err := strconv.Atoi(ctx.Value)
	if err != nil {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Revision must be an integer: '%s'", lints.SeverityError, ctx.Value), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	return nil
}

func validateContentType(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	if ctx.Value != "text/plain" {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Content-Type must be 'text/plain', got: '%s'", lints.SeverityError, ctx.Value), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	return nil
}

func validateNewsItemFormat(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	if ctx.Value != "1.0" && ctx.Value != "2.0" {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Unsupported News-Item-Format: '%s'", lints.SeverityWarning, ctx.Value), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityWarning
		return []lints.LintResult{res}
	}
	return nil
}

func validateNoSpacesOrTabs(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	if strings.ContainsAny(ctx.Value, " \t") {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Invalid %s format, should not contain multiple values or spaces: '%s'", lints.SeverityError, ctx.Key, ctx.Value), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	return nil
}

func validateCategoryPackage(ctx *HeaderContext, state *NewsItemState) []lints.LintResult {
	if !strings.Contains(ctx.Value, "/") {
		res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Invalid Display-If-Installed format, expected category/package: '%s'", lints.SeverityError, ctx.Value), File: ctx.RelPath, Line: ctx.LineNum}
		res.RuleMetadata.Severity = lints.SeverityError
		return []lints.LintResult{res}
	}
	return nil
}

var headerValidators = map[string][]HeaderValidator{
	"Title": {
		validateTitleLength,
	},
	"Author": {
		validateEmailFormat,
	},
	"Translator": {
		validateEmailFormat,
	},
	"Posted": {
		validatePostedDate,
	},
	"Revision": {
		validateRevisionInteger,
	},
	"Content-Type": {
		validateContentType,
	},
	"News-Item-Format": {
		validateNewsItemFormat,
	},
	"Display-If-Installed": {
		validateNoSpacesOrTabs,
		validateCategoryPackage,
	},
	"Display-If-Keyword": {
		validateNoSpacesOrTabs,
	},
	"Display-If-Profile": {
		validateNoSpacesOrTabs,
	},
}

func validateRequiredHeader(header string) PostValidator {
	return func(state *NewsItemState, relPath string) []lints.LintResult {
		if _, ok := state.Headers[header]; !ok {
			res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Missing required header: %s", lints.SeverityError, header), File: relPath}
			res.RuleMetadata.Severity = lints.SeverityError
			return []lints.LintResult{res}
		}
		return nil
	}
}

func validateFormatContentTypeCrossCheck(state *NewsItemState, relPath string) []lints.LintResult {
	if formatVal, hasFormat := state.Headers["News-Item-Format"]; hasFormat && formatVal == "1.0" {
		contentType, hasContentType := state.Headers["Content-Type"]
		if !hasContentType || contentType != "text/plain" {
			res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Content-Type: text/plain is mandatory for News-Item-Format 1.0", lints.SeverityError), File: relPath}
			res.RuleMetadata.Severity = lints.SeverityError
			return []lints.LintResult{res}
		}
	}
	return nil
}

var postValidators = []PostValidator{
	validateRequiredHeader("Title"),
	validateRequiredHeader("Author"),
	validateRequiredHeader("Posted"),
	validateRequiredHeader("Revision"),
	validateRequiredHeader("News-Item-Format"),
	validateFormatContentTypeCrossCheck,
}

func (r *NewsValidityLintRule) lintNewsItem(content string, relPath string) []lints.LintResult {
	var results []lints.LintResult
	lines := strings.Split(content, "\n")

	inBody := false
	state := &NewsItemState{Headers: make(map[string]string)}

	for lineNum, line := range lines {
		if inBody {
			if strings.Contains(line, "\t") {
				res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Body lines should not contain tab characters", lints.SeverityWarning), File: relPath, Line: lineNum + 1}
				res.RuleMetadata.Severity = lints.SeverityWarning
				results = append(results, res)
			}
			if len(line) > 72 {
				res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Body lines should wrap at 72 characters", lints.SeverityWarning), File: relPath, Line: lineNum + 1}
				res.RuleMetadata.Severity = lints.SeverityWarning
				results = append(results, res)
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			inBody = true
			continue
		}

		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// Folded header continuation, ignore since format validation happens anyway
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			res := lints.LintResult{
				RuleMetadata: ruleNewsValidity,
				Message:      fmt.Sprintf("[%s] Invalid header format: '%s'", lints.SeverityError, line),
				File:         relPath,
				Line:         lineNum + 1,
			}
			res.RuleMetadata.Severity = lints.SeverityError
			results = append(results, res)
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		state.Headers[key] = val

		ctx := &HeaderContext{
			Key:     key,
			Value:   val,
			RelPath: relPath,
			LineNum: lineNum + 1,
		}

		if validators, ok := headerValidators[key]; ok {
			for _, validator := range validators {
				results = append(results, validator(ctx, state)...)
			}
		} else {
			res := lints.LintResult{RuleMetadata: ruleNewsValidity, Message: fmt.Sprintf("[%s] Unknown header: '%s'", lints.SeverityWarning, key), File: relPath, Line: lineNum + 1}
			res.RuleMetadata.Severity = lints.SeverityWarning
			results = append(results, res)
		}
	}

	for _, validator := range postValidators {
		results = append(results, validator(state, relPath)...)
	}

	return results
}
