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

	if item.NewsItemFormat == "2.0" {
		item.BodyAST = parseNewsBodyAST(item.Body)
	}

	return item
}

// parseNewsBodyAST processes the raw text of a Format 2.0 news item body into an AST representation.
func parseNewsBodyAST(body string) []NewsNode {
	lines := strings.Split(body, "\n")
	var nodes []NewsNode

	var currentList *NewsNode
	var currentCode *NewsNode
	var currentText *NewsNode

	flushText := func() {
		if currentText != nil {
			nodes = append(nodes, *currentText)
			currentText = nil
		}
	}

	flushCode := func() {
		if currentCode != nil {
			nodes = append(nodes, *currentCode)
			currentCode = nil
		}
	}

	flushList := func() {
		if currentList != nil {
			nodes = append(nodes, *currentList)
			currentList = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		isListStart := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")

		if isListStart {
			flushText()
			flushCode()

			if currentList == nil {
				currentList = &NewsNode{Type: NewsNodeList, Lines: []string{}}
			}

			prefix := "- "
			if strings.HasPrefix(trimmed, "* ") {
				prefix = "* "
			}

			listItem := []string{strings.TrimPrefix(trimmed, prefix)}

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
					listItem = append(listItem, nextTrimmed)
					j++
				} else {
					break
				}
			}

			currentList.Lines = append(currentList.Lines, strings.Join(listItem, " "))
			i = j - 1
			continue
		}

		isCodeLine := strings.HasPrefix(line, "  ") && trimmed != ""

		if isCodeLine {
			flushText()
			flushList()

			if currentCode == nil {
				currentCode = &NewsNode{Type: NewsNodeCode, Lines: []string{}}
			}
			if strings.HasPrefix(line, "  ") {
				currentCode.Lines = append(currentCode.Lines, line[2:])
			} else {
				currentCode.Lines = append(currentCode.Lines, line)
			}
		} else {
			flushList()

			if currentCode != nil {
				if trimmed == "" {
					// Blank line within code block
					currentCode.Lines = append(currentCode.Lines, "")
				} else {
					flushCode()
					currentText = &NewsNode{Type: NewsNodeText, Lines: []string{line}}
				}
			} else {
				// Text context
				if currentText == nil {
					currentText = &NewsNode{Type: NewsNodeText, Lines: []string{}}
				}
				currentText.Lines = append(currentText.Lines, line)
			}
		}
	}

	flushList()
	flushCode()
	flushText()

	return nodes
}

// ToHTMLTemplate converts the NewsItem body into an HTML template representation.
func (n NewsItem) ToHTMLTemplate() template.HTML {
	if n.NewsItemFormat == "2.0" {
		var out []string
		for _, node := range n.BodyAST {
			switch node.Type {
			case NewsNodeText:
				for _, line := range node.Lines {
					if strings.TrimSpace(line) == "" {
						out = append(out, "<br><br>")
					} else {
						out = append(out, template.HTMLEscapeString(line))
					}
				}
			case NewsNodeList:
				out = append(out, "<ul>")
				for _, item := range node.Lines {
					out = append(out, "<li>"+template.HTMLEscapeString(item)+"</li>")
				}
				out = append(out, "</ul>")
			case NewsNodeCode:
				var codeLines []string
				for _, codeLine := range node.Lines {
					codeLines = append(codeLines, template.HTMLEscapeString(codeLine))
				}
				out = append(out, "<pre><code>"+strings.Join(codeLines, "\n")+"</code></pre>")
			}
		}
		return template.HTML(strings.Join(out, "\n"))
	}
	escaped := template.HTMLEscapeString(n.Body)
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	return template.HTML(escaped)
}

// ToText returns the plain text body of the NewsItem.
// It will reconstruct it from the AST for 2.0 format to demonstrate serialization.
func (n NewsItem) ToText(compliant ...bool) string {
	isCompliant := len(compliant) > 0 && compliant[0]
	if n.NewsItemFormat == "2.0" {
		var out []string
		for i, node := range n.BodyAST {
			if isCompliant && i > 0 {
				out = append(out, "")
			}
			switch node.Type {
			case NewsNodeText:
				out = append(out, node.Lines...)
			case NewsNodeList:
				for _, item := range node.Lines {
					if isCompliant {
						out = append(out, " * "+item)
					} else {
						out = append(out, " - "+item)
					}
				}
			case NewsNodeCode:
				for _, codeLine := range node.Lines {
					if codeLine == "" {
						out = append(out, "")
					} else {
						if isCompliant {
							out = append(out, " "+codeLine)
						} else {
							out = append(out, "  "+codeLine)
						}
					}
				}
			}
		}
		return strings.Join(out, "\n")
	}

	return n.Body
}
