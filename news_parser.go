package g2

import (
	"html/template"
	"strings"
	"time"
)

// ParseNewsItem parses the content of a Gentoo news file into a NewsItem structure.
func ParseNewsItem(content string) NewsItem {
	lines := strings.Split(content, "\n")
	var item NewsItem

	inBody := false
	var bodyLines []string

	for _, line := range lines {
		if inBody {
			bodyLines = append(bodyLines, line)
			continue
		}

		if strings.TrimSpace(line) == "" {
			inBody = true
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Title":
			item.Title = val
		case "Author":
			item.Author = StripEmail(val)
		case "Translator":
			item.Translator = append(item.Translator, StripEmail(val))
		case "Posted":
			t, err := time.Parse("2006-01-02", val)
			if err == nil {
				item.Posted = t
			}
		case "Revision":
			item.Revision = val
		case "News-Item-Format":
			item.NewsItemFormat = val
		case "Display-If-Installed":
			item.DisplayIfInstalled = append(item.DisplayIfInstalled, val)
		case "Display-If-Keyword":
			item.DisplayIfKeyword = append(item.DisplayIfKeyword, val)
		case "Display-If-Profile":
			item.DisplayIfProfile = append(item.DisplayIfProfile, val)
		}
	}

	item.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return item
}

// ToHTMLTemplate converts the NewsItem body into an HTML template representation.
func (n NewsItem) ToHTMLTemplate() template.HTML {
	if n.NewsItemFormat == "2.0" {
		return parseNewsBodyHTML(n.Body)
	}
	escaped := template.HTMLEscapeString(n.Body)
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	return template.HTML(escaped)
}

// ToText returns the plain text body of the NewsItem.
func (n NewsItem) ToText() string {
	return n.Body
}

func parseNewsBodyHTML(body string) template.HTML {
	lines := strings.Split(body, "\n")
	var out []string

	inList := false
	inCode := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		isListStart := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")

		if isListStart {
			if inCode {
				out = append(out, "</code></pre>")
				inCode = false
			}
			if !inList {
				out = append(out, "<ul>")
				inList = true
			}

			prefix := "- "
			if strings.HasPrefix(trimmed, "* ") {
				prefix = "* "
			}

			listItem := []string{template.HTMLEscapeString(strings.TrimPrefix(trimmed, prefix))}

			indent := len(line) - len(strings.TrimLeft(line, " \t"))

			j := i + 1
			for j < len(lines) {
				nextLine := lines[j]
				nextTrimmed := strings.TrimSpace(nextLine)
				nextIndent := len(nextLine) - len(strings.TrimLeft(nextLine, " \t"))

				if nextTrimmed == "" {
					break
				}

				isNextListStart := strings.HasPrefix(nextTrimmed, "- ") || strings.HasPrefix(nextTrimmed, "* ")

				if nextIndent > indent && !isNextListStart {
					listItem = append(listItem, template.HTMLEscapeString(nextTrimmed))
					j++
				} else {
					break
				}
			}

			out = append(out, "<li>"+strings.Join(listItem, " ")+"</li>")
			i = j - 1
			continue
		}

		isCodeLine := strings.HasPrefix(line, "  ") && trimmed != ""

		if isCodeLine {
			if inList {
				out = append(out, "</ul>")
				inList = false
			}
			if !inCode {
				out = append(out, "<pre><code>")
				inCode = true
			}
			if strings.HasPrefix(line, "  ") {
				out = append(out, template.HTMLEscapeString(line[2:]))
			} else {
				out = append(out, template.HTMLEscapeString(line))
			}
		} else {
			if inList {
				out = append(out, "</ul>")
				inList = false
			}
			if inCode {
				if trimmed == "" {
					out = append(out, "")
				} else {
					out = append(out, "</code></pre>")
					inCode = false
					out = append(out, template.HTMLEscapeString(line))
				}
			} else {
				if trimmed == "" {
					out = append(out, "<br><br>")
				} else {
					out = append(out, template.HTMLEscapeString(line))
				}
			}
		}
	}

	if inList {
		out = append(out, "</ul>")
	}
	if inCode {
		out = append(out, "</code></pre>")
	}

	return template.HTML(strings.Join(out, "\n"))
}
