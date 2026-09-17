package pipeline

import (
	"errors"
	"fmt"
	"strings"
)

// TokenizePipeline parses a pipeline string into individual commands.
func TokenizePipeline(pipelineStr string) ([]string, error) {
	var commands []string
	var currentCmd strings.Builder
	parenDepth := 0
	var inQuote rune
	escapeNext := false

	for _, char := range pipelineStr {
		if escapeNext {
			currentCmd.WriteRune(char)
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			currentCmd.WriteRune(char)
			continue
		}

		switch inQuote {
		case 0:
			if char == '\'' || char == '"' {
				inQuote = char
				currentCmd.WriteRune(char)
				continue
			}
			switch char {
			case '(':
				parenDepth++
			case ')':
				parenDepth--
				if parenDepth < 0 {
					return nil, errors.New("unbalanced parentheses in pipeline")
				}
			case '|':
				if parenDepth == 0 {
					cmd := strings.TrimSpace(currentCmd.String())
					if cmd == "" {
						return nil, errors.New("empty pipeline stage")
					}
					commands = append(commands, cmd)
					currentCmd.Reset()
					continue
				}
			}
		default:
			if char == inQuote {
				inQuote = 0
			}
			currentCmd.WriteRune(char)
			continue
		}

		currentCmd.WriteRune(char)
	}

	if escapeNext {
		return nil, errors.New("dangling escape in pipeline")
	}
	if inQuote != 0 {
		return nil, errors.New("unterminated quote in pipeline")
	}
	if parenDepth > 0 {
		return nil, errors.New("unbalanced parentheses in pipeline")
	}

	cmd := strings.TrimSpace(currentCmd.String())
	if cmd == "" {
		return nil, errors.New("empty pipeline stage")
	}
	commands = append(commands, cmd)

	return commands, nil
}

// PerformSubstitutions replaces placeholders like ${VAR} with values from the map.
func PerformSubstitutions(pipelineStr string, subs map[string]string) (string, error) {
	var result strings.Builder
	inSubst := false
	var currentVar strings.Builder

	for i := 0; i < len(pipelineStr); i++ {
		char := pipelineStr[i]
		if !inSubst {
			if char == '$' && i+1 < len(pipelineStr) && pipelineStr[i+1] == '{' {
				inSubst = true
				i++ // skip '{'
				continue
			}
			result.WriteByte(char)
		} else {
			if char == '}' {
				varName := currentVar.String()
				val, ok := subs[varName]
				if !ok {
					return "", fmt.Errorf("missing required substitution: ${%s}", varName)
				}
				result.WriteString(val)
				inSubst = false
				currentVar.Reset()
			} else {
				currentVar.WriteByte(char)
			}
		}
	}

	if inSubst {
		return "", fmt.Errorf("unterminated substitution: ${%s", currentVar.String())
	}

	return result.String(), nil
}
