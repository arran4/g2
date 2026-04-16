package g2

import (
	"fmt"
	"strings"
)

// RequiredUseNode represents a node in the REQUIRED_USE AST.
type RequiredUseNode interface {
	String() string
	Evaluate(context map[string]bool) bool
}

// RequiredUseFlag represents a single USE flag.
type RequiredUseFlag struct {
	Name string
}

func (f RequiredUseFlag) String() string {
	return f.Name
}

func (f RequiredUseFlag) Evaluate(context map[string]bool) bool {
	// A flag starting with ! means it must NOT be set
	if strings.HasPrefix(f.Name, "!") {
		flagName := strings.TrimPrefix(f.Name, "!")
		return !context[flagName]
	}
	return context[f.Name]
}

// RequiredUseAllOf represents a list of requirements, ALL of which must be met.
type RequiredUseAllOf struct {
	Nodes []RequiredUseNode
}

func (a RequiredUseAllOf) String() string {
	var parts []string
	for _, n := range a.Nodes {
		parts = append(parts, n.String())
	}
	return strings.Join(parts, " ")
}

func (a RequiredUseAllOf) Evaluate(context map[string]bool) bool {
	if len(a.Nodes) == 0 {
		return true
	}
	for _, n := range a.Nodes {
		if !n.Evaluate(context) {
			return false
		}
	}
	return true
}

// RequiredUseAnyOf represents a list of requirements, ANY of which must be met (||).
type RequiredUseAnyOf struct {
	Nodes []RequiredUseNode
}

func (a RequiredUseAnyOf) String() string {
	var parts []string
	for _, n := range a.Nodes {
		parts = append(parts, n.String())
	}
	return fmt.Sprintf("|| ( %s )", strings.Join(parts, " "))
}

func (a RequiredUseAnyOf) Evaluate(context map[string]bool) bool {
	if len(a.Nodes) == 0 {
		return false
	}
	for _, n := range a.Nodes {
		if n.Evaluate(context) {
			return true
		}
	}
	return false
}

// RequiredUseExactlyOneOf represents a list of requirements, EXACTLY ONE of which must be met (^^).
type RequiredUseExactlyOneOf struct {
	Nodes []RequiredUseNode
}

func (e RequiredUseExactlyOneOf) String() string {
	var parts []string
	for _, n := range e.Nodes {
		parts = append(parts, n.String())
	}
	return fmt.Sprintf("^^ ( %s )", strings.Join(parts, " "))
}

func (e RequiredUseExactlyOneOf) Evaluate(context map[string]bool) bool {
	count := 0
	for _, n := range e.Nodes {
		if n.Evaluate(context) {
			count++
		}
	}
	return count == 1
}

// RequiredUseAtMostOneOf represents a list of requirements, AT MOST ONE of which must be met (??).
type RequiredUseAtMostOneOf struct {
	Nodes []RequiredUseNode
}

func (a RequiredUseAtMostOneOf) String() string {
	var parts []string
	for _, n := range a.Nodes {
		parts = append(parts, n.String())
	}
	return fmt.Sprintf("?? ( %s )", strings.Join(parts, " "))
}

func (a RequiredUseAtMostOneOf) Evaluate(context map[string]bool) bool {
	count := 0
	for _, n := range a.Nodes {
		if n.Evaluate(context) {
			count++
		}
	}
	return count <= 1
}

// RequiredUseConditional represents a conditional requirement (flag? ( ... ) or !flag? ( ... )).
type RequiredUseConditional struct {
	Condition string
	Nodes     RequiredUseAllOf
}

func (c RequiredUseConditional) String() string {
	return fmt.Sprintf("%s? ( %s )", c.Condition, c.Nodes.String())
}

func (c RequiredUseConditional) Evaluate(context map[string]bool) bool {
	cond := c.Condition
	negated := false
	if strings.HasPrefix(cond, "!") {
		negated = true
		cond = strings.TrimPrefix(cond, "!")
	}

	val := context[cond]
	if negated {
		val = !val
	}

	if val {
		return c.Nodes.Evaluate(context)
	}
	// If the condition is not met, the requirement is vacuously true
	return true
}

type RequiredUseConfig struct {
	// Add config fields if needed
}

type RequiredUseOption interface {
	Apply(*RequiredUseConfig)
}

// ParseRequiredUse parses a REQUIRED_USE string into an AST.
func ParseRequiredUse(input string, opts ...any) (RequiredUseNode, error) {
	cfg := RequiredUseConfig{}

	for _, opt := range opts {
		switch o := opt.(type) {
		case RequiredUseOption:
			o.Apply(&cfg)
		}
	}

	tokens := tokenizeRequiredUse(input)
	if len(tokens) == 0 {
		return RequiredUseAllOf{}, nil
	}

	nodes, _, err := parseTokens(tokens, 0, len(tokens))
	if err != nil {
		return nil, err
	}

	if len(nodes) == 1 {
		if _, isAllOf := nodes[0].(RequiredUseAllOf); isAllOf {
			return nodes[0], nil
		}
		if _, isAnyOf := nodes[0].(RequiredUseAnyOf); isAnyOf {
			return nodes[0], nil
		}
		if _, isExactly := nodes[0].(RequiredUseExactlyOneOf); isExactly {
			return nodes[0], nil
		}
		if _, isAtMost := nodes[0].(RequiredUseAtMostOneOf); isAtMost {
			return nodes[0], nil
		}
		if _, isCond := nodes[0].(RequiredUseConditional); isCond {
			return nodes[0], nil
		}
	}

	return RequiredUseAllOf{Nodes: nodes}, nil
}

func tokenizeRequiredUse(input string) []string {
	var tokens []string
	fields := strings.Fields(input)
	tokens = append(tokens, fields...)
	return tokens
}

func parseTokens(tokens []string, start int, end int) ([]RequiredUseNode, int, error) {
	var nodes []RequiredUseNode
	i := start

	for i < end {
		tok := tokens[i]

		if tok == ")" {
			// This should be handled by the caller, if we see it here it's an error unless returning
			return nodes, i, nil
		}

		if tok == "||" || tok == "^^" || tok == "??" {
			if i+1 >= end || tokens[i+1] != "(" {
				return nil, i, fmt.Errorf("expected '(' after %s at token %d", tok, i)
			}

			childList, nextI, err := parseTokens(tokens, i+2, end)
			if err != nil {
				return nil, nextI, err
			}

			if nextI >= end || tokens[nextI] != ")" {
				return nil, nextI, fmt.Errorf("expected ')' closing %s block", tok)
			}

			switch tok {
			case "||":
				nodes = append(nodes, RequiredUseAnyOf{Nodes: childList})
			case "^^":
				nodes = append(nodes, RequiredUseExactlyOneOf{Nodes: childList})
			case "??":
				nodes = append(nodes, RequiredUseAtMostOneOf{Nodes: childList})
			}

			i = nextI + 1
			continue
		}

		if strings.HasSuffix(tok, "?") {
			cond := strings.TrimSuffix(tok, "?")
			if i+1 >= end || tokens[i+1] != "(" {
				return nil, i, fmt.Errorf("expected '(' after conditional %s at token %d", tok, i)
			}

			childList, nextI, err := parseTokens(tokens, i+2, end)
			if err != nil {
				return nil, nextI, err
			}

			if nextI >= end || tokens[nextI] != ")" {
				return nil, nextI, fmt.Errorf("expected ')' closing conditional %s block", tok)
			}

			nodes = append(nodes, RequiredUseConditional{
				Condition: cond,
				Nodes:     RequiredUseAllOf{Nodes: childList},
			})

			i = nextI + 1
			continue
		}

		// Just a regular flag
		nodes = append(nodes, RequiredUseFlag{Name: tok})
		i++
	}

	return nodes, i, nil
}
