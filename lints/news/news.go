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

type Headers map[string][]string

type Finding struct {
	Severity   lints.Severity
	Message    string
	ValueIndex int
}

type HeaderValidator func(key string, values []string) []Finding
type Validator func(headers Headers) []Finding

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

func required() HeaderValidator {
	return func(key string, values []string) []Finding {
		if len(values) == 0 {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Missing required header: %s", key)}}
		}
		return nil
	}
}

func single() HeaderValidator {
	return func(key string, values []string) []Finding {
		if len(values) > 1 {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Header %s must appear at most once", key), ValueIndex: 1}}
		}
		return nil
	}
}

func nonEmpty() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if len(val) == 0 {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("%s cannot be empty", key)}}
		}
		return nil
	}
}

func maxLength(max int) func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if len(val) > max {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("%s exceeds maximum length of %d characters", key, max)}}
		}
		return nil
	}
}

func each(valFn func(key string, val string) []Finding) HeaderValidator {
	return func(key string, values []string) []Finding {
		var findings []Finding
		for i, val := range values {
			valFindings := valFn(key, val)
			for j := range valFindings {
				valFindings[j].ValueIndex = i
			}
			findings = append(findings, valFindings...)
		}
		return findings
	}
}

func validEmail() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if !checkEmailFormat(val) {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Invalid %s format, expected 'Name <email@domain.com>': '%s'", key, val)}}
		}
		return nil
	}
}

func validDate() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if _, err := time.Parse("2006-01-02", val); err != nil {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Invalid %s date format, expected YYYY-MM-DD: '%s'", key, val)}}
		}
		return nil
	}
}

func validInteger() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if len(val) == 0 {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("%s cannot be empty", key)}}
		}
		if _, err := strconv.Atoi(val); err != nil {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("%s must be an integer: '%s'", key, val)}}
		}
		return nil
	}
}

func validContentType() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if val != "text/plain" {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Content-Type must be 'text/plain', got: '%s'", val)}}
		}
		return nil
	}
}

func validNewsItemFormat() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if val != "1.0" && val != "2.0" {
			return []Finding{{Severity: lints.SeverityWarning, Message: fmt.Sprintf("Unsupported News-Item-Format: '%s'", val)}}
		}
		return nil
	}
}

func validInstalledPackage() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if strings.ContainsAny(val, " \t") {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Invalid %s format, should not contain multiple values or spaces: '%s'", key, val)}}
		}
		if !strings.Contains(val, "/") {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Invalid Display-If-Installed format, expected category/package: '%s'", val)}}
		}
		return nil
	}
}

func validKeyword() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if strings.ContainsAny(val, " \t") {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Invalid %s format, should not contain multiple values or spaces: '%s'", key, val)}}
		}
		return nil
	}
}

func validProfile() func(key string, val string) []Finding {
	return func(key string, val string) []Finding {
		if strings.ContainsAny(val, " \t") {
			return []Finding{{Severity: lints.SeverityError, Message: fmt.Sprintf("Invalid %s format, should not contain multiple values or spaces: '%s'", key, val)}}
		}
		return nil
	}
}

type HeaderRule struct {
	Key        string
	Validators []HeaderValidator
}

func header(key string, validators ...HeaderValidator) HeaderRule {
	return HeaderRule{
		Key:        key,
		Validators: validators,
	}
}

var headerRules = []HeaderRule{
	header("Title", required(), single(), each(nonEmpty()), each(maxLength(50))),
	header("Author", required(), each(validEmail())),
	header("Translator", each(validEmail())),
	header("Posted", required(), single(), each(validDate())),
	header("Revision", required(), single(), each(validInteger())),
	header("Content-Type", single(), each(validContentType())),
	header("News-Item-Format", required(), single(), each(validNewsItemFormat())),
	header("Display-If-Installed", each(validInstalledPackage())),
	header("Display-If-Keyword", each(validKeyword())),
	header("Display-If-Profile", each(validProfile())),
}

var headerRegistry map[string][]HeaderValidator

func init() {
	headerRegistry = make(map[string][]HeaderValidator)
	for _, rule := range headerRules {
		headerRegistry[rule.Key] = rule.Validators
	}
}

func validateFormatContentTypeCrossCheck(headers Headers) []Finding {
	if formatVals, hasFormat := headers["News-Item-Format"]; hasFormat && len(formatVals) > 0 && formatVals[0] == "1.0" {
		contentTypeVals, hasContentType := headers["Content-Type"]
		if !hasContentType || len(contentTypeVals) == 0 || contentTypeVals[0] != "text/plain" {
			return []Finding{{Severity: lints.SeverityError, Message: "Content-Type: text/plain is mandatory for News-Item-Format 1.0"}}
		}
	}
	return nil
}

var postValidators = []Validator{
	validateFormatContentTypeCrossCheck,
}

func convertFinding(f Finding, relPath string, lineNum int) lints.LintResult {
	res := lints.LintResult{
		RuleMetadata: ruleNewsValidity,
		Message:      fmt.Sprintf("[%s] %s", f.Severity, f.Message),
		File:         relPath,
	}
	if lineNum > 0 {
		res.Line = lineNum
	}
	res.RuleMetadata.Severity = f.Severity
	return res
}

func (r *NewsValidityLintRule) lintNewsItem(content string, relPath string) []lints.LintResult {
	var results []lints.LintResult
	lines := strings.Split(content, "\n")

	inBody := false
	headers := make(Headers)
	lineNumbers := make(map[string][]int)
	var orderedKeys []string

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

		if len(headers[key]) == 0 {
			orderedKeys = append(orderedKeys, key)
		}
		headers[key] = append(headers[key], val)
		lineNumbers[key] = append(lineNumbers[key], lineNum+1)
	}

	// Validate declared headers in deterministic order
	for _, rule := range headerRules {
		vals := headers[rule.Key]
		for _, validator := range rule.Validators {
			findings := validator(rule.Key, vals)
			for _, f := range findings {
				lineNum := 0
				if f.ValueIndex >= 0 && f.ValueIndex < len(lineNumbers[rule.Key]) {
					lineNum = lineNumbers[rule.Key][f.ValueIndex]
				} else if len(lineNumbers[rule.Key]) > 0 {
					lineNum = lineNumbers[rule.Key][0] // fallback to first occurrence
				}
				results = append(results, convertFinding(f, relPath, lineNum))
			}
		}
	}

	// Validate unknown headers in source order
	for _, key := range orderedKeys {
		if _, ok := headerRegistry[key]; !ok {
			for i := range headers[key] {
				f := Finding{Severity: lints.SeverityWarning, Message: fmt.Sprintf("Unknown header: '%s'", key), ValueIndex: i}
				lineNum := lineNumbers[key][i]
				results = append(results, convertFinding(f, relPath, lineNum))
			}
		}
	}

	// Run cross-header post validators
	for _, validator := range postValidators {
		findings := validator(headers)
		for _, f := range findings {
			// Since cross header doesn't tie cleanly to a single line number, pass 0
			results = append(results, convertFinding(f, relPath, 0))
		}
	}

	return results
}
