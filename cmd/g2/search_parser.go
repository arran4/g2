package main

import (
	"strings"
	"unicode"
)

type TokenType string

const (
	PAREN    TokenType = "PAREN"
	NOT      TokenType = "NOT"
	AND      TokenType = "AND"
	OR       TokenType = "OR"
	SEQUENCE TokenType = "SEQUENCE"
	TERM     TokenType = "TERM"
	FIELD    TokenType = "FIELD"
	GROUP    TokenType = "GROUP"
)

type Token struct {
	Type  TokenType
	Value string
}

type ASTNode struct {
	Type  TokenType
	Value string
	Field string
	Left  *ASTNode
	Right *ASTNode
	Expr  *ASTNode
}

type SearchParser struct {
	query      string
	tokens     []Token
	tokenIndex int
}

func NewSearchParser(query string) *SearchParser {
	parser := &SearchParser{
		query: query,
	}
	parser.tokens = parser.tokenize(query)
	return parser
}

func (p *SearchParser) tokenize(query string) []Token {
	var tokens []Token
	pos := 0
	runes := []rune(query)

	for pos < len(runes) {
		for pos < len(runes) && unicode.IsSpace(runes[pos]) {
			pos++
		}
		if pos >= len(runes) {
			break
		}

		char := runes[pos]

		if char == '(' || char == ')' {
			tokens = append(tokens, Token{Type: PAREN, Value: string(char)})
			pos++
		} else if char == '-' || char == '!' {
			tokens = append(tokens, Token{Type: NOT, Value: string(char)})
			pos++
		} else if char == '&' && pos+1 < len(runes) && runes[pos+1] == '&' {
			tokens = append(tokens, Token{Type: AND, Value: "&&"})
			pos += 2
		} else if char == '|' && pos+1 < len(runes) && runes[pos+1] == '|' {
			tokens = append(tokens, Token{Type: OR, Value: "||"})
			pos += 2
		} else if char == '\'' {
			pos++
			var seq strings.Builder
			for pos < len(runes) && runes[pos] != '\'' {
				seq.WriteRune(runes[pos])
				pos++
			}
			if pos < len(runes) && runes[pos] == '\'' {
				pos++
			}
			tokens = append(tokens, Token{Type: SEQUENCE, Value: seq.String()})
		} else {
			var term strings.Builder
			inQuotes := false

			for pos < len(runes) {
				if runes[pos] == '"' {
					inQuotes = !inQuotes
					pos++
					continue
				}
				if !inQuotes && unicode.IsSpace(runes[pos]) {
					break
				}
				if !inQuotes && (runes[pos] == '(' || runes[pos] == ')') {
					break
				}
				term.WriteRune(runes[pos])
				pos++
			}

			termStr := term.String()
			if termStr == "AND" {
				tokens = append(tokens, Token{Type: AND, Value: termStr})
			} else if termStr == "OR" {
				tokens = append(tokens, Token{Type: OR, Value: termStr})
			} else if termStr == "NOT" {
				tokens = append(tokens, Token{Type: NOT, Value: termStr})
			} else {
				tokens = append(tokens, Token{Type: TERM, Value: termStr})
			}
		}
	}
	return tokens
}

func (p *SearchParser) peek() *Token {
	if p.tokenIndex < len(p.tokens) {
		return &p.tokens[p.tokenIndex]
	}
	return nil
}

func (p *SearchParser) consume() *Token {
	if p.tokenIndex < len(p.tokens) {
		tok := &p.tokens[p.tokenIndex]
		p.tokenIndex++
		return tok
	}
	return nil
}

func (p *SearchParser) Parse() *ASTNode {
	if len(p.tokens) == 0 {
		return nil
	}
	return p.parseOr()
}

func (p *SearchParser) parseOr() *ASTNode {
	left := p.parseAnd()
	for p.peek() != nil && p.peek().Type == OR {
		p.consume()
		right := p.parseAnd()
		left = &ASTNode{Type: OR, Left: left, Right: right}
	}
	return left
}

func (p *SearchParser) parseAnd() *ASTNode {
	left := p.parseUnary()
	for p.peek() != nil && p.peek().Type != OR && p.peek().Value != ")" {
		if p.peek().Type == AND {
			p.consume()
		}
		right := p.parseUnary()
		left = &ASTNode{Type: AND, Left: left, Right: right}
	}
	return left
}

func (p *SearchParser) parseUnary() *ASTNode {
	if p.peek() != nil && p.peek().Type == NOT {
		p.consume()
		expr := p.parsePrimary()
		return &ASTNode{Type: NOT, Expr: expr}
	}
	return p.parsePrimary()
}

func (p *SearchParser) parsePrimary() *ASTNode {
	token := p.peek()
	if token == nil {
		return nil
	}

	if token.Type == PAREN && token.Value == "(" {
		p.consume()
		expr := p.parseOr()
		if p.peek() != nil && p.peek().Type == PAREN && p.peek().Value == ")" {
			p.consume()
		}
		return &ASTNode{Type: GROUP, Expr: expr}
	}

	if token.Type == SEQUENCE {
		p.consume()
		return &ASTNode{Type: SEQUENCE, Value: token.Value}
	}

	if token.Type == TERM {
		p.consume()
		colonIndex := strings.Index(token.Value, ":")
		if colonIndex > 0 {
			field := token.Value[:colonIndex]
			val := token.Value[colonIndex+1:]
			return &ASTNode{Type: FIELD, Field: strings.ToLower(field), Value: val}
		}
		return &ASTNode{Type: TERM, Value: token.Value}
	}

	p.consume()
	return &ASTNode{Type: TERM, Value: token.Value}
}
