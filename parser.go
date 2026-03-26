package g2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

var (
	ErrUnexpectedEOF = errors.New("unexpected EOF")
	ErrSyntaxError   = errors.New("syntax error")
)

type Position struct {
	Line   int
	Column int
}

func (p Position) String() string {
	return fmt.Sprintf("line %d, col %d", p.Line, p.Column)
}

type ParseError struct {
	Pos Position
	Err error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: %v", e.Pos, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// Wrapper for tracking position
type Reader struct {
	r   *bufio.Reader
	Pos Position
}

func NewReader(r io.Reader) *Reader {
	return &Reader{
		r:   bufio.NewReader(r),
		Pos: Position{Line: 1, Column: 1},
	}
}

func (r *Reader) ReadRune() (rune, int, error) {
	ch, size, err := r.r.ReadRune()
	if err != nil {
		return ch, size, err
	}
	if ch == '\n' {
		r.Pos.Line++
		r.Pos.Column = 1
	} else {
		r.Pos.Column++
	}
	return ch, size, nil
}

func (r *Reader) UnreadRune() error {
	err := r.r.UnreadRune()
	if err == nil {
		r.Pos.Column--
	}
	return err
}

type EbuildNode struct {
	Variables map[string]string
}

// EbuildParser implements a recursive descent parser for Gentoo ebuild files.
type EbuildParser struct {
	ctx context.Context
	r   *Reader
	// Lookahead
	peekRune rune
	peekErr  error
	hasPeek  bool
}

func NewEbuildParser(ctx context.Context, reader io.Reader) *EbuildParser {
	return &EbuildParser{
		ctx: ctx,
		r:   NewReader(reader),
	}
}

func (p *EbuildParser) nextRune() (rune, error) {
	select {
	case <-p.ctx.Done():
		return 0, fmt.Errorf("%w: %w", ErrSyntaxError, p.ctx.Err())
	default:
	}

	if p.hasPeek {
		p.hasPeek = false
		return p.peekRune, p.peekErr
	}
	r, _, err := p.r.ReadRune()
	return r, err
}

func (p *EbuildParser) peek() (rune, error) {
	if p.hasPeek {
		return p.peekRune, p.peekErr
	}
	r, _, err := p.r.ReadRune()
	p.peekRune = r
	p.peekErr = err
	p.hasPeek = true
	return r, err
}

func (p *EbuildParser) consumeWhitespaceAndComments() error {
	for {
		r, err := p.peek()
		if err != nil {
			return err
		}
		if unicode.IsSpace(r) {
			_, _ = p.nextRune()
			continue
		}
		if r == '#' {
			// consume until newline
			for {
				nr, err := p.nextRune()
				if err != nil {
					return err
				}
				if nr == '\n' {
					break
				}
			}
			continue
		}
		return nil // Not space or comment
	}
}

// Parse extracts variables from the ebuild using a recursive descent approach
// tailored specifically for ebuilds, bypassing full bash posix rules.
func (p *EbuildParser) Parse() (*EbuildNode, error) {
	ebuild := &EbuildNode{
		Variables: make(map[string]string),
	}

	for {
		err := p.consumeWhitespaceAndComments()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		r, err := p.peek()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		if unicode.IsLetter(r) || r == '_' {
			// Possibly a variable assignment, function decl, or command
			ident, err := p.consumeIdent()
			if err != nil {
				return nil, err
			}

			// check what's next
			err = p.consumeWhitespaceAndComments()
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}

			nextR, err := p.peek()
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}

			switch nextR {
			case '=':
				_, _ = p.nextRune() // consume '='
				val, err := p.consumeValue()
				if err != nil && !errors.Is(err, io.EOF) {
					return nil, err
				}
				ebuild.Variables[ident] = val
			case '(':
				// Function declaration e.g. `src_prepare() {`
				_, _ = p.nextRune() // '('
				r, _ = p.nextRune() // ')'
				if r != ')' {
					return nil, fmt.Errorf("%w: expected ')' at %s", ErrSyntaxError, p.r.Pos)
				}
				err = p.skipFunctionBody()
				if err != nil {
					return nil, err
				}
			default:
				if ident == "inherit" {
					// Collect inherited eclasses
					val, err := p.consumeLine()
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if ebuild.Variables["INHERITED"] != "" {
						ebuild.Variables["INHERITED"] += " " + val
					} else {
						ebuild.Variables["INHERITED"] = val
					}
				} else {
					// Bare command or reserved word. Skip line.
					p.skipLine()
				}
			}
		} else {
			// Not an assignment or function we care about right now at top level.
			p.skipLine()
		}
	}

	return ebuild, nil
}

func (p *EbuildParser) consumeIdent() (string, error) {
	var sb strings.Builder
	for {
		r, err := p.peek()
		if err != nil {
			if errors.Is(err, io.EOF) && sb.Len() > 0 {
				return sb.String(), nil
			}
			return "", err
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '[' || r == ']' {
			sb.WriteRune(r)
			_, _ = p.nextRune()
		} else {
			break
		}
	}
	return sb.String(), nil
}

func (p *EbuildParser) consumeValue() (string, error) {
	// Skip optional spaces after =
	for {
		r, err := p.peek()
		if err != nil {
			return "", err
		}
		if r == ' ' || r == '\t' {
			_, _ = p.nextRune()
		} else {
			break
		}
	}

	r, err := p.peek()
	if err != nil {
		return "", err
	}

	switch r {
	case '"', '\'':
		return p.consumeQuotedString()
	case '(':
		return p.consumeArray()
	}

	// Bare word until space or newline
	var sb strings.Builder
	for {
		r, err := p.peek()
		if err != nil {
			return sb.String(), nil
		}
		if unicode.IsSpace(r) || r == '#' {
			break
		}
		sb.WriteRune(r)
		_, _ = p.nextRune()
	}
	return sb.String(), nil
}

func (p *EbuildParser) consumeQuotedString() (string, error) {
	quote, _ := p.nextRune()
	var sb strings.Builder
	escape := false
	for {
		r, err := p.nextRune()
		if err != nil {
			return "", fmt.Errorf("%w: unterminated string %v", ErrSyntaxError, err)
		}
		if escape {
			sb.WriteRune(r)
			escape = false
			continue
		}
		if r == '\\' {
			escape = true
			sb.WriteRune(r) // Keep the escape in the AST representation for now
			continue
		}
		if r == quote {
			break
		}
		sb.WriteRune(r)
	}
	return sb.String(), nil
}

func (p *EbuildParser) consumeArray() (string, error) {
	_, _ = p.nextRune() // consume '('
	var sb strings.Builder
	sb.WriteString("(")
	// read until matching ')'
	parens := 1
	for {
		r, err := p.nextRune()
		if err != nil {
			return "", fmt.Errorf("%w: unterminated array %v", ErrSyntaxError, err)
		}
		if r == '(' {
			parens++
		} else if r == ')' {
			parens--
			if parens == 0 {
				sb.WriteRune(r)
				break
			}
		}
		if r == '#' {
			// Comment inside array! Common ebuild issue.
			for {
				nr, err := p.nextRune()
				if err != nil || nr == '\n' {
					break
				}
			}
			continue // Skipped comment
		}
		sb.WriteRune(r)
	}
	return sb.String(), nil
}

func (p *EbuildParser) skipFunctionBody() error {
	err := p.consumeWhitespaceAndComments()
	if err != nil {
		return err
	}
	r, err := p.nextRune()
	if err != nil {
		return err
	}
	if r != '{' {
		return fmt.Errorf("%w: expected '{' for function body", ErrSyntaxError)
	}

	braces := 1
	for {
		r, err := p.nextRune()
		if err != nil {
			return fmt.Errorf("%w: unterminated function %v", ErrSyntaxError, err)
		}
		if r == '{' {
			braces++
		} else if r == '}' {
			braces--
			if braces == 0 {
				break
			}
		} else if r == '"' || r == '\'' {
			// Skip strings inside functions so we don't accidentally match braces
			p.peekRune = r
			p.hasPeek = true
			_, _ = p.consumeQuotedString()
		} else if r == '#' {
			// skip comments
			for {
				nr, err := p.nextRune()
				if err != nil || nr == '\n' {
					break
				}
			}
		}
	}
	return nil
}

func (p *EbuildParser) skipLine() {
	for {
		r, err := p.nextRune()
		if err != nil || r == '\n' {
			break
		}
	}
}

func (p *EbuildParser) consumeLine() (string, error) {
	var sb strings.Builder
	for {
		r, err := p.nextRune()
		if err != nil {
			return strings.TrimSpace(sb.String()), err
		}
		if r == '\n' {
			break
		}
		if r == '#' {
			p.skipLine()
			break
		}
		sb.WriteRune(r)
	}
	return strings.TrimSpace(sb.String()), nil
}
