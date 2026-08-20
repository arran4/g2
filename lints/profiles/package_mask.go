package profiles

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleGlep84Format = lints.RuleMetadata{
	ID:          "Glep84Format",
	Title:       "GLEP 84 Format",
	Description: "Verifies that profiles/package.mask conforms to GLEP 84 format specifications.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"profiles", "package.mask", "glep-84"},
}

func init() {
	lints.RegisterRuleMetadata(ruleGlep84Format)
	lints.RegisterRepoLintRule(&Glep84FormatLintRule{})
}

type Glep84FormatLintRule struct{}

var (
	glep84HeaderRegex   = regexp.MustCompile(`^# Uses GLEP 84 format$`)
	separationLineRegex = regexp.MustCompile(`^# -{5,}.*-{5,}$`)
	authorLineRegex     = regexp.MustCompile(`^# (.+) <([^>]+)> \(([0-9]{4}-[0-9]{2}-[0-9]{2})\)$`)
	lastRiteRegex       = regexp.MustCompile(`^Removal on ([0-9]{4}-[0-9]{2}-[0-9]{2})[.,]?\s+(.*)$`)
	bugsRegex           = regexp.MustCompile(`^(?i)Bugs? (#[0-9]+(?:,\s*#[0-9]+)*)\.?$`)
)

func (l *Glep84FormatLintRule) LintRepo(repoDir string, site *g2.SiteData) []lints.LintResult {
	maskPath := filepath.Join(repoDir, "profiles", "package.mask")
	f, err := os.Open(maskPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []lints.LintResult{{
			RuleMetadata: ruleGlep84Format,
			Message:      fmt.Sprintf("Failed to open package.mask: %v", err),
			File:         "profiles/package.mask",
		}}
	}
	defer func() { _ = f.Close() }()

	var results []lints.LintResult
	scanner := bufio.NewScanner(f)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	usesGlep84 := false
	headerIndex := -1
	for i, line := range lines {
		if glep84HeaderRegex.MatchString(line) {
			usesGlep84 = true
			headerIndex = i
			break
		}
	}

	if !usesGlep84 {
		return nil
	}

	startIndex := headerIndex + 1

	// Find separation lines
	firstSepIndex := -1
	secondSepIndex := -1

	for i := startIndex; i < len(lines); i++ {
		if separationLineRegex.MatchString(lines[i]) {
			if firstSepIndex == -1 {
				firstSepIndex = i
			} else if secondSepIndex == -1 {
				secondSepIndex = i
				break
			}
		}
	}

	if firstSepIndex != -1 {
		startIndex = firstSepIndex + 1
	}

	endIndex := len(lines)
	if secondSepIndex != -1 {
		endIndex = secondSepIndex
	}

	entriesLines := lines[startIndex:endIndex]

	// Parse entries
	type Entry struct {
		Comments   []string
		Packages   []string
		StartLine  int // 1-based index in the file
		EndLine    int
		CommentLNs []int
		PackageLNs []int
	}

	var entries []Entry
	var currentEntry Entry
	inEntry := false
	inPackages := false

	for i, line := range entriesLines {
		fileLineNum := startIndex + i + 1

		if !inEntry && strings.TrimSpace(line) == "" {
			continue
		}

		if !inEntry {
			inEntry = true
			inPackages = false
			currentEntry = Entry{StartLine: fileLineNum}
		}

		isComment := strings.HasPrefix(line, "#")
		if !inPackages {
			if isComment {
				currentEntry.Comments = append(currentEntry.Comments, line)
				currentEntry.CommentLNs = append(currentEntry.CommentLNs, fileLineNum)
			} else {
				if strings.TrimSpace(line) != "" {
					inPackages = true
					currentEntry.Packages = append(currentEntry.Packages, line)
					currentEntry.PackageLNs = append(currentEntry.PackageLNs, fileLineNum)
				}
			}
		} else {
			if isComment {
				// new entry started
				currentEntry.EndLine = fileLineNum - 1
				entries = append(entries, currentEntry)
				currentEntry = Entry{StartLine: fileLineNum, Comments: []string{line}, CommentLNs: []int{fileLineNum}}
				inPackages = false
			} else {
				if strings.TrimSpace(line) != "" {
					currentEntry.Packages = append(currentEntry.Packages, line)
					currentEntry.PackageLNs = append(currentEntry.PackageLNs, fileLineNum)
				}
			}
		}
	}
	if inEntry {
		currentEntry.EndLine = len(lines)
		entries = append(entries, currentEntry)
	}

	for _, entry := range entries {
		// Validations for Comments Block
		if len(entry.Comments) == 0 {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleGlep84Format,
				Message:      "Missing comments block in entry",
				File:         "profiles/package.mask",
				Line:         entry.StartLine,
			})
			continue
		}

		authorLine := entry.Comments[0]
		if !authorLineRegex.MatchString(authorLine) {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleGlep84Format,
				Message:      "First line of comment block must be a valid author line",
				File:         "profiles/package.mask",
				Line:         entry.CommentLNs[0],
			})
		}

		var explanationLines []string
		var explanationLNs []int
		for j, c := range entry.Comments {
			if j == 0 {
				continue // Skip author line
			}

			// Validate line length <= 80
			if len(c) > 80 {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleGlep84Format,
					Message:      "Comment line exceeds 80 characters",
					File:         "profiles/package.mask",
					Line:         entry.CommentLNs[j],
				})
			}

			// Validate comment spacing
			if c == "#" || strings.HasPrefix(c, "# ") {
				// OK
			} else {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleGlep84Format,
					Message:      "Comment must start with '# ' or be an empty comment '#'",
					File:         "profiles/package.mask",
					Line:         entry.CommentLNs[j],
				})
			}

			// Strip prefix for processing paragraphs
			stripped := strings.TrimPrefix(c, "#")
			stripped = strings.TrimPrefix(stripped, " ")
			explanationLines = append(explanationLines, stripped)
			explanationLNs = append(explanationLNs, entry.CommentLNs[j])
		}

		// Ensure no multiple blank lines between paragraphs
		blankCount := 0
		for j, expl := range explanationLines {
			if expl == "" {
				blankCount++
				if blankCount > 1 {
					results = append(results, lints.LintResult{
						RuleMetadata: ruleGlep84Format,
						Message:      "Multiple blank lines between paragraphs are prohibited",
						File:         "profiles/package.mask",
						Line:         explanationLNs[j],
					})
				}
			} else {
				blankCount = 0
			}
		}

		// Last-rite epilogue
		var lastParagraph []string
		var lastParagraphLineNum int
		for j := len(explanationLines) - 1; j >= 0; j-- {
			if explanationLines[j] == "" {
				break
			}
			lastParagraph = append([]string{explanationLines[j]}, lastParagraph...)
			lastParagraphLineNum = explanationLNs[j]
		}

		if len(lastParagraph) > 0 && strings.HasPrefix(lastParagraph[0], "Removal on") {
			lastRiteText := strings.Join(lastParagraph, " ") // join to handle wrapped text

			matches := lastRiteRegex.FindStringSubmatch(lastRiteText)
			if len(matches) < 3 {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleGlep84Format,
					Message:      "Invalid last-rite epilogue format",
					File:         "profiles/package.mask",
					Line:         lastParagraphLineNum,
				})
			} else {
				bugsPart := matches[2]
				if !bugsRegex.MatchString(bugsPart) {
					results = append(results, lints.LintResult{
						RuleMetadata: ruleGlep84Format,
						Message:      "Invalid bugs list format in last-rite epilogue",
						File:         "profiles/package.mask",
						Line:         lastParagraphLineNum,
					})
				}
			}
		}

		// Packages List validations
		// no comments in package list -> we already grouped them properly (a # starts a new entry)
		// no leading or trailing whitespace -> we checked if strings.TrimSpace(line) != line
		for j, pkgLine := range entry.Packages {
			ln := entry.PackageLNs[j]
			if strings.TrimSpace(pkgLine) != pkgLine {
				results = append(results, lints.LintResult{
					RuleMetadata: ruleGlep84Format,
					Message:      "Package line has leading or trailing whitespace",
					File:         "profiles/package.mask",
					Line:         ln,
				})
			}
		}

		// Check for blank line separating comment block and package list
		// The package list must immediately follow the comment block (no blank line)
		firstPkgLn := entry.PackageLNs[0]
		lastCommentLn := entry.CommentLNs[len(entry.CommentLNs)-1]
		if firstPkgLn != lastCommentLn + 1 {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleGlep84Format,
				Message:      "No blank line is allowed between comments block and packages list",
				File:         "profiles/package.mask",
				Line:         firstPkgLn,
			})
		}
	}

	// Check for mandatory blank line between entries
	for i := 0; i < len(entries)-1; i++ {
		curr := entries[i]
		next := entries[i+1]

		// The next entry must start AT LEAST 2 lines after the previous entry's last package line
		if next.StartLine <= curr.PackageLNs[len(curr.PackageLNs)-1] + 1 {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleGlep84Format,
				Message:      "A mandatory blank line must appear between entries",
				File:         "profiles/package.mask",
				Line:         next.StartLine,
			})
		}
	}

	return results
}
