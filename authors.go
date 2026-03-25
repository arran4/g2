package g2

import (
	"bufio"
	"io"
	"strings"
)

// Author represents an entry in the metadata/AUTHORS file.
type Author struct {
	Name  string
	Email string
}

// ParseAuthors parses the metadata/AUTHORS file format.
// It ignores blank lines and comments starting with '#'.
// Expected format: "Name <email@example.com>" or just "Name".
func ParseAuthors(r io.Reader) ([]Author, error) {
	var authors []Author
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		author := Author{}
		startEmail := strings.LastIndex(line, "<")
		endEmail := strings.LastIndex(line, ">")

		if startEmail != -1 && endEmail != -1 && endEmail > startEmail {
			author.Name = strings.TrimSpace(line[:startEmail])
			author.Email = strings.TrimSpace(line[startEmail+1 : endEmail])
		} else {
			author.Name = line
		}

		authors = append(authors, author)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return authors, nil
}
