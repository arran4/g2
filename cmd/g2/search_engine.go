package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type SearchEngine struct {
	documents   []SearchDocument
	regexCache  map[string]*regexp.Regexp
	regexCacheMu sync.RWMutex
}

func NewSearchEngine() *SearchEngine {
	return &SearchEngine{
		regexCache: make(map[string]*regexp.Regexp),
	}
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

	e.regexCacheMu.RLock()
	re, ok := e.regexCache[pattern]
	e.regexCacheMu.RUnlock()

	if !ok {
		regexPattern := regexp.QuoteMeta(pattern)
		regexPattern = strings.ReplaceAll(regexPattern, "\\*", ".*")
		regexPattern = strings.ReplaceAll(regexPattern, "\\?", ".")
		var err error
		re, err = regexp.Compile(regexPattern)
		if err != nil {
			return strings.Contains(text, pattern)
		}
		e.regexCacheMu.Lock()
		e.regexCache[pattern] = re
		e.regexCacheMu.Unlock()
	}

	return re.MatchString(text)
}

func (e *SearchEngine) matchTerm(doc SearchDocument, term string) bool {
	termLower := strings.ToLower(term)
	return e.matchWildcard(doc.SearchText, termLower)
}

func (e *SearchEngine) matchField(doc SearchDocument, field string, value string) bool {
	valLower := strings.ToLower(value)

	switch field {
	case "category":
		return e.matchWildcard(strings.ToLower(doc.Category), valLower)
	case "package":
		return e.matchWildcard(strings.ToLower(doc.Package), valLower)
	case "name", "fullname":
		return e.matchWildcard(strings.ToLower(doc.FullName), valLower)
	case "desc", "description":
		return e.matchWildcard(strings.ToLower(doc.Description), valLower)
	case "license":
		for _, l := range doc.Licenses {
			if e.matchWildcard(strings.ToLower(l), valLower) {
				return true
			}
		}
		return false
	case "use":
		for _, u := range doc.Uses {
			if e.matchWildcard(strings.ToLower(u), valLower) {
				return true
			}
		}
		for _, u := range doc.UseDescriptions {
			if e.matchWildcard(strings.ToLower(u), valLower) {
				return true
			}
		}
		return false
	case "keyword", "keywords", "arch", "arches":
		for _, k := range doc.Keywords {
			if e.matchWildcard(strings.ToLower(k), valLower) {
				return true
			}
		}
		for _, a := range doc.Arches {
			if e.matchWildcard(strings.ToLower(a), valLower) {
				return true
			}
		}
		return false
	case "mask":
		return strings.ToLower(doc.Mask) == valLower
	case "depend", "depends", "rdepend", "rdepends":
		for _, d := range doc.Depends {
			if e.matchWildcard(strings.ToLower(d), valLower) {
				return true
			}
		}
		for _, d := range doc.Rdepends {
			if e.matchWildcard(strings.ToLower(d), valLower) {
				return true
			}
		}
		return false
	case "overlay":
		return e.matchWildcard(strings.ToLower(doc.Overlay), valLower)
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
