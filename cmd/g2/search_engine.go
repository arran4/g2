package main

import (
	"fmt"
	"regexp"
	"strings"
)

type SearchEngine struct {
	documents   []SearchDocument
}

func NewSearchEngine() *SearchEngine {
	return &SearchEngine{}
}

func (e *SearchEngine) LoadDocuments(docs []SearchDocument) {
	e.documents = append(e.documents, docs...)
}

func (e *SearchEngine) Search(query string) []SearchDocument {
	if strings.TrimSpace(query) == "" {
		return e.documents
	}

	parser := NewSearchParser(query)
	ast := parser.Parse()

	if ast == nil {
		return e.documents
	}

	var results []SearchDocument
	for _, doc := range e.documents {
		if e.evaluateAST(doc, ast) {
			results = append(results, doc)
		}
	}

	return results
}

func (e *SearchEngine) evaluateAST(doc SearchDocument, ast *ASTNode) bool {
	if ast == nil {
		return true
	}

	switch ast.Type {
	case AND:
		return e.evaluateAST(doc, ast.Left) && e.evaluateAST(doc, ast.Right)
	case OR:
		return e.evaluateAST(doc, ast.Left) || e.evaluateAST(doc, ast.Right)
	case NOT:
		return !e.evaluateAST(doc, ast.Expr)
	case GROUP:
		return e.evaluateAST(doc, ast.Expr)
	case TERM:
		return e.matchTerm(doc, ast.Value)
	case FIELD:
		return e.matchField(doc, ast.Field, ast.Value)
	case SEQUENCE:
		return e.matchSequence(doc, ast.Value)
	default:
		return false
	}
}

func (e *SearchEngine) matchWildcard(text, pattern string) bool {
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
		return strings.Contains(text, pattern)
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return findQuestionContains(text, pattern) != -1
	}

	prefix := parts[0]
	if len(prefix) > 0 {
		foundIdx := findQuestionContains(text, prefix)
		if foundIdx == -1 {
			return false
		}
		text = text[foundIdx+len(prefix):]
	}

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if len(part) == 0 {
			continue
		}
		foundIdx := findQuestionContains(text, part)
		if foundIdx == -1 {
			return false
		}
		text = text[foundIdx+len(part):]
	}
	return true
}

func findQuestionContains(text, pattern string) int {
	if len(pattern) == 0 {
		return 0
	}
	if len(text) < len(pattern) {
		return -1
	}
	for i := 0; i <= len(text)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if pattern[j] != '?' && pattern[j] != text[i+j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func (e *SearchEngine) matchTerm(doc SearchDocument, term string) bool {
	termLower := strings.ToLower(term)
	return e.matchWildcard(doc.SearchText, termLower)
}

func (e *SearchEngine) matchField(doc SearchDocument, field string, value string) bool {
	valLower := strings.ToLower(value)

	switch field {
	case "category":
		return e.matchWildcard(doc.Category, valLower)
	case "package":
		return e.matchWildcard(doc.Package, valLower)
	case "name", "fullname":
		return e.matchWildcard(doc.FullName, valLower)
	case "desc", "description":
		return e.matchWildcard(doc.Description, valLower)
	case "license":
		for _, l := range doc.Licenses {
			if e.matchWildcard(l, valLower) {
				return true
			}
		}
		return false
	case "use":
		for _, u := range doc.Uses {
			if e.matchWildcard(u, valLower) {
				return true
			}
		}
		for _, u := range doc.UseDescriptions {
			if e.matchWildcard(u, valLower) {
				return true
			}
		}
		return false
	case "keyword", "keywords", "arch", "arches":
		for _, k := range doc.Keywords {
			if e.matchWildcard(k, valLower) {
				return true
			}
		}
		for _, a := range doc.Arches {
			if e.matchWildcard(a, valLower) {
				return true
			}
		}
		return false
	case "mask":
		return doc.Mask == valLower // Already lowercase
	case "depend", "depends", "rdepend", "rdepends":
		for _, d := range doc.Depends {
			if e.matchWildcard(d, valLower) {
				return true
			}
		}
		for _, d := range doc.Rdepends {
			if e.matchWildcard(d, valLower) {
				return true
			}
		}
		return false
	case "overlay":
		return e.matchWildcard(doc.Overlay, valLower)
	case "version":
		return e.matchVersion(doc, value)
	default:
		return e.matchTerm(doc, value)
	}
}

func (e *SearchEngine) matchSequence(doc SearchDocument, seq string) bool {
	seqLower := strings.ToLower(seq)
	words := strings.Fields(seqLower)
	if len(words) == 0 {
		return true
	}

	lastIndex := -1
	for _, word := range words {
		idx := strings.Index(doc.SearchText[lastIndex+1:], word)
		if idx == -1 {
			return false
		}
		lastIndex = lastIndex + 1 + idx
	}
	return true
}

var (
	reVersionRev = regexp.MustCompile(`-r(\d+)$`)
	reDigits     = regexp.MustCompile(`\d+`)
)

func (e *SearchEngine) matchVersion(doc SearchDocument, queryVersion string) bool {
	op := "=="
	v := queryVersion
	if strings.HasPrefix(queryVersion, ">=") {
		op = ">="
		v = queryVersion[2:]
	} else if strings.HasPrefix(queryVersion, "<=") {
		op = "<="
		v = queryVersion[2:]
	} else if strings.HasPrefix(queryVersion, ">") {
		op = ">"
		v = queryVersion[1:]
	} else if strings.HasPrefix(queryVersion, "<") {
		op = "<"
		v = queryVersion[1:]
	}

	padVersion := func(ver string) string {
		if ver == "" {
			return ""
		}

		pVer := reVersionRev.ReplaceAllString(ver, "+r$1")

		return reDigits.ReplaceAllStringFunc(pVer, func(s string) string {
			return fmt.Sprintf("%010s", s)
		})
	}

	docVersionPadded := doc.VersionSortKey
	if docVersionPadded == "" {
		docVersionPadded = padVersion(doc.Version)
	}
	queryVersionPadded := padVersion(v)

	switch op {
	case "==":
		return docVersionPadded == queryVersionPadded
	case ">":
		return docVersionPadded > queryVersionPadded
	case "<":
		return docVersionPadded < queryVersionPadded
	case ">=":
		return docVersionPadded >= queryVersionPadded
	case "<=":
		return docVersionPadded <= queryVersionPadded
	default:
		return false
	}
}
