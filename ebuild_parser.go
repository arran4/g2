package g2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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

type AST struct {
	Value string
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

	// Sanity Check: Gentoo ebuild files are text files.
	// If we encounter a null byte, it indicates a corrupted file,
	// a binary file, or a padding issue. We should fail parsing immediately
	// to prevent propagating garbage data.
	if ch == '\x00' {
		return 0, 0, fmt.Errorf("corrupted file: null byte encountered at %v", r.Pos)
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

// EbuildParser implements a recursive descent parser for Gentoo ebuild files.
type EbuildParser struct {
	ctx context.Context
	r   *Reader
	// Lookahead
	peekRune rune
	peekErr  error
	hasPeek  bool

	Warnings []string
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

func (p *EbuildParser) consumeHeaderAndWhitespace() (string, error) {
	var header strings.Builder
	inHeader := true
	for {
		r, err := p.peek()
		if err != nil {
			return header.String(), err
		}
		if unicode.IsSpace(r) {
			_, _ = p.nextRune()
			if inHeader && r == '\n' {
				header.WriteRune(r)
			}
			continue
		}
		if r == '#' {
			// consume until newline
			_, _ = p.nextRune()
			if inHeader {
				header.WriteRune('#')
			}
			for {
				nr, err := p.nextRune()
				if err != nil {
					return strings.TrimSpace(header.String()), err
				}
				if inHeader {
					header.WriteRune(nr)
				}
				if nr == '\n' {
					break
				}
			}
			continue
		}
		return strings.TrimSpace(header.String()), nil // Not space or comment
	}
}

// ParsedEbuild contains the results of parsing an ebuild
type ParsedEbuild struct {
	Variables    map[string]string
	Functions    map[string]AST
	Order        []string
	EbuildHeader string
	Warnings     []string
}

// Parse extracts variables and functions from the ebuild using a recursive descent approach
// tailored specifically for ebuilds, bypassing full bash posix rules.
func (p *EbuildParser) Parse() (ParsedEbuild, error) {
	result := ParsedEbuild{
		Variables: make(map[string]string),
		Functions: make(map[string]AST),
		Order:     make([]string, 0),
	}

	header, err := p.consumeHeaderAndWhitespace()
	if err != nil && !errors.Is(err, io.EOF) {
		return result, err
	}
	result.EbuildHeader = header

	for {
		err := p.consumeWhitespaceAndComments()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return result, err
		}

		r, err := p.peek()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return result, err
		}

		if unicode.IsLetter(r) || r == '_' {
			// Possibly a variable assignment, function decl, or command
			ident, err := p.consumeIdent()
			if err != nil {
				return result, err
			}

			// check what's next
			err = p.consumeWhitespaceAndComments()
			if err != nil && !errors.Is(err, io.EOF) {
				return result, err
			}

			nextR, err := p.peek()
			if err != nil && !errors.Is(err, io.EOF) {
				return result, err
			}

			switch nextR {
			case '=':
				_, _ = p.nextRune() // consume '='
				val, err := p.consumeValue()
				if err != nil && !errors.Is(err, io.EOF) {
					return result, err
				}
				if strings.HasSuffix(ident, "+") {
					ident = strings.TrimSuffix(ident, "+")
					if result.Variables[ident] != "" {
						result.Variables[ident] += " " + val
					} else {
						result.Variables[ident] = val
						result.Order = append(result.Order, ident)
					}
				} else {
					if _, exists := result.Variables[ident]; !exists {
						result.Order = append(result.Order, ident)
					} else {
						result.Warnings = append(result.Warnings, fmt.Sprintf("Duplicate assignment for variable '%s'", ident))
					}
					result.Variables[ident] = val
				}
			case '(':
				// Function declaration e.g. `src_prepare() {`
				_, _ = p.nextRune() // '('
				r, _ = p.nextRune() // ')'
				if r != ')' {
					// It's not a function declaration, probably part of a bash command like `if (( PLEVEL < 0 ))`
					// We'll just skip the line.
					p.skipLine()
					continue
				}
				body, err := p.consumeFunctionBody()
				if err != nil {
					return result, err
				}
				if _, exists := result.Functions[ident]; !exists {
					result.Order = append(result.Order, ident)
				}
				result.Functions[ident] = AST{Value: body}
			default:
				if ident == "inherit" {
					// Collect inherited eclasses
					val, err := p.consumeLine()
					if err != nil && !errors.Is(err, io.EOF) {
						return result, err
					}
					if result.Variables["INHERITED"] != "" {
						result.Variables["INHERITED"] += " " + val
					} else {
						result.Variables["INHERITED"] = val
					}
				} else if ident == "if" || ident == "elif" || ident == "while" || ident == "until" || ident == "for" || ident == "case" {
					// These reserved words open bash blocks, we shouldn't skip the whole line blindly
					// and just ignore the keyword itself so the parser continues into the block
					// We just skip the condition part
					err := p.consumeCondition()
					if err != nil && !errors.Is(err, io.EOF) {
						return result, err
					}
					continue
				} else if ident == "fi" || ident == "done" || ident == "esac" || ident == "then" || ident == "else" || ident == "do" {
					// Block closers / continuations. Ignore.
					continue
				} else {
					// Bare command or reserved word. Skip line.
					p.skipLine()
				}
			}
		} else if r == ';' {
			// Statement separator
			_, _ = p.nextRune()
		} else if r == ')' {
			// Case statement condition close e.g. `a)`
			_, _ = p.nextRune()
			r, err := p.peek()
			if err == nil && r == ';' {
				_, _ = p.nextRune()
			}
		} else {
			// Not an assignment or function we care about right now at top level.
			p.skipLine()
		}
	}

	return result, nil
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
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '[' || r == ']' || r == '+' {
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
		if r == '"' || r == '\'' {
			quote := r
			escape := false
			sb.WriteRune(r)
			for {
				nr, err := p.nextRune()
				if err != nil {
					break
				}
				sb.WriteRune(nr)
				if escape {
					escape = false
					continue
				}
				if nr == '\\' {
					escape = true
					continue
				}
				if nr == quote {
					break
				}
			}
			continue
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

func (p *EbuildParser) consumeCondition() error {
	// Consumes up to the next `;` or `\n` to skip the conditional test.
	// E.g., `if use x; then` -> we skip `use x;`
	// Or `case $V1 in` -> we skip `$V1 in`
	for {
		r, err := p.peek()
		if err != nil {
			return err
		}
		if r == ';' || r == '\n' {
			_, _ = p.nextRune() // consume it
			break
		}
		_, _ = p.nextRune()
	}
	return nil
}

func (p *EbuildParser) consumeFunctionBody() (string, error) {
	err := p.consumeWhitespaceAndComments()
	if err != nil {
		return "", err
	}
	r, err := p.nextRune()
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	opener := '{'
	closer := '}'
	if r != '{' {
		if r == '(' {
			msg := "expected '{' for function body but found '(', treating as subshell body"
			log.Printf("Warning: parsing ebuild variables: %s", msg)
			p.Warnings = append(p.Warnings, msg)
			opener = '('
			closer = ')'
		} else {
			return "", fmt.Errorf("%w: expected '{' for function body", ErrSyntaxError)
		}
	}
	sb.WriteRune(r)

	braces := 1
	for {
		r, err := p.nextRune()
		if err != nil {
			// If we hit EOF, it's just the end of the file. Ignore unterminated function for best-effort parsing.
			if errors.Is(err, io.EOF) {
				return sb.String(), nil
			}
			return "", fmt.Errorf("%w: unterminated function %v", ErrSyntaxError, err)
		}
		sb.WriteRune(r)
		if r == opener {
			braces++
		} else if r == closer {
			braces--
			if braces == 0 {
				break
			}
		} else if r == '"' || r == '\'' {
			// Skip strings inside functions so we don't accidentally match braces
			// We cannot just use consumeQuotedString if we don't handle backslashes over newlines properly in all cases
			quote := r
			escape := false
			for {
				nr, nerr := p.nextRune()
				if nerr != nil {
					break
				}
				sb.WriteRune(nr)
				if escape {
					escape = false
					continue
				}
				if nr == '\\' {
					escape = true
					continue
				}
				if nr == quote {
					break
				}
			}
		} else if r == '`' {
			// Skip backticks as well
			for {
				br, berr := p.nextRune()
				if berr != nil {
					break
				}
				sb.WriteRune(br)
				if br == '`' {
					break
				}
			}
		} else if r == '#' {
			// skip comments
			for {
				nr, err := p.nextRune()
				if err != nil {
					break
				}
				sb.WriteRune(nr)
				if nr == '\n' {
					break
				}
			}
		}
	}
	return sb.String(), nil
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
		if r == '\\' {
			nextR, nextErr := p.peek()
			if nextErr == nil && nextR == '\n' {
				// line continuation, skip the newline
				_, _ = p.nextRune()
				continue
			}
		}
		if r == ';' {
			// Statement separator - end of this line's command
			break
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
	// clean up any excessive whitespace or escaped newlines
	out := strings.ReplaceAll(sb.String(), "\\\n", " ")
	out = strings.Join(strings.Fields(out), " ")
	return out, nil
}
