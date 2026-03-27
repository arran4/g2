package g2

import (
	"strings"
	"time"
)

// NewsNodeType identifies the type of an AST node in a news item body.
type NewsNodeType int

const (
	NewsNodeText NewsNodeType = iota
	NewsNodeList
	NewsNodeCode
)

// NewsNode represents a single abstract syntax tree element in a news item body.
type NewsNode struct {
	Type  NewsNodeType
	Lines []string // Contains the text for text/code, or list items
}

// NewsItem represents a parsed Gentoo News Item (GLEP 42).
type NewsItem struct {
	Title              string
	Author             string
	Translator         []string
	Posted             time.Time
	Revision           string
	NewsItemFormat     string
	DisplayIfInstalled []string
	DisplayIfKeyword   []string
	DisplayIfProfile   []string
	Body               string
	BodyAST            []NewsNode
	DirName            string
	FileName           string
}

// ToHTML is here for template interface compatibility
func (n NewsItem) ToHTML() string {
	return string(n.ToHTMLTemplate())
}

// StripEmail removes the email portion "<email@example.com>" from a string.
func StripEmail(s string) string {
	start := strings.Index(s, "<")
	end := strings.Index(s, ">")
	if start != -1 && end != -1 && start < end {
		s = s[:start] + s[end+1:]
	}
	s = strings.ReplaceAll(s, "  ", " ")
	return strings.TrimSpace(s)
}
