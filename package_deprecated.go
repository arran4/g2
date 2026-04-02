package g2

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

// PackageDeprecatedEntry represents a single deprecation entry from profiles/package.deprecated.
type PackageDeprecatedEntry struct {
	Package string
}

// PackageDeprecated represents a block of deprecation entries grouped by author and reason.
type PackageDeprecated struct {
	Reason      string
	Date        string
	Author      string
	AuthorEmail string
	Entries     []PackageDeprecatedEntry
}

// Match: # Michał Górny <mgorny@gentoo.org> (2025-11-25)
var authorLineRegex = regexp.MustCompile(`^#\s*(.*?)\s*<(.*?)>\s*\((.*?)\)$`)

func parsePackageDeprecatedReader(r io.Reader) ([]PackageDeprecated, error) {
	var results []PackageDeprecated
	scanner := bufio.NewScanner(r)

	var currentReason []string
	var currentAuthor, currentEmail, currentDate string
	var currentEntries []PackageDeprecatedEntry

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if len(currentEntries) > 0 {
				reasonStr := strings.Join(currentReason, " ")
				reasonStr = strings.Join(strings.Fields(reasonStr), " ")
				results = append(results, PackageDeprecated{
					Reason:      reasonStr,
					Date:        currentDate,
					Author:      currentAuthor,
					AuthorEmail: currentEmail,
					Entries:     currentEntries,
				})
				currentEntries = nil
			}
			currentReason = nil
			currentAuthor = ""
			currentEmail = ""
			currentDate = ""
			continue
		}

		if strings.HasPrefix(line, "#") {
			matches := authorLineRegex.FindStringSubmatch(line)
			if len(matches) == 4 {
				if len(currentEntries) > 0 {
					reasonStr := strings.Join(currentReason, " ")
					reasonStr = strings.Join(strings.Fields(reasonStr), " ")
					results = append(results, PackageDeprecated{
						Reason:      reasonStr,
						Date:        currentDate,
						Author:      currentAuthor,
						AuthorEmail: currentEmail,
						Entries:     currentEntries,
					})
					currentEntries = nil
					currentReason = nil
				}
				currentAuthor = strings.TrimSpace(matches[1])
				currentEmail = strings.TrimSpace(matches[2])
				currentDate = strings.TrimSpace(matches[3])
			} else if currentAuthor != "" {
				// Only capture reasons if we've seen an author header
				reasonLine := strings.TrimSpace(strings.TrimPrefix(line, "#"))
				if reasonLine != "" {
					currentReason = append(currentReason, reasonLine)
				}
			}
		} else {
			// Not a comment, assume package name
			if currentAuthor != "" && currentEmail != "" {
				currentEntries = append(currentEntries, PackageDeprecatedEntry{
					Package: line,
				})
			}
		}
	}

	if len(currentEntries) > 0 {
		reasonStr := strings.Join(currentReason, " ")
		reasonStr = strings.Join(strings.Fields(reasonStr), " ")
		results = append(results, PackageDeprecated{
			Reason:      reasonStr,
			Date:        currentDate,
			Author:      currentAuthor,
			AuthorEmail: currentEmail,
			Entries:     currentEntries,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading package.deprecated: %w", err)
	}

	return results, nil
}

func ParsePackageDeprecatedFS(sysFS fs.FS, path string) ([]PackageDeprecated, error) {
	f, err := sysFS.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parsePackageDeprecatedReader(f)
}

func ParsePackageDeprecated(path string) ([]PackageDeprecated, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parsePackageDeprecatedReader(f)
}

func SerializePackageDeprecated(w io.Writer, deprecated []PackageDeprecated) error {
	for i, d := range deprecated {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "# %s <%s> (%s)\n", d.Author, d.AuthorEmail, d.Date); err != nil {
			return err
		}
		// Split reason by roughly 70 chars to keep typical width, or just print lines
		// We'll write a simple word wrap
		if d.Reason != "" {
			wrappedReason := wrapText(d.Reason, 70)
			for _, line := range wrappedReason {
				if _, err := fmt.Fprintf(w, "# %s\n", line); err != nil {
					return err
				}
			}
		}
		for _, entry := range d.Entries {
			if _, err := fmt.Fprintf(w, "%s\n", entry.Package); err != nil {
				return err
			}
		}
	}
	return nil
}

func wrapText(text string, width int) []string {
	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	var currentLine strings.Builder
	currentLine.WriteString(words[0])

	for _, word := range words[1:] {
		if currentLine.Len()+1+len(word) > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			currentLine.WriteByte(' ')
			currentLine.WriteString(word)
		}
	}
	lines = append(lines, currentLine.String())
	return lines
}
