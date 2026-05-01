package g2

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func isAlphaUnderscore(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isAlnumUnderscore(c byte) bool {
	return isAlphaUnderscore(c) || (c >= '0' && c <= '9')
}

// fastResolveBash attempts to quickly resolve simple bash variables like $VAR or ${VAR}
// without invoking the full parser. Returns the resolved string and true if successful.
// If it encounters complex bash syntax, it returns an empty string and false.
func fastResolveBash(text string, variables map[string]string) (string, bool) {
	// If it contains quotes, subshells, command substitution, globbing, or escapes, fallback to full parser.
	// We want to be very conservative to avoid subtle bash evaluation differences.
	if strings.ContainsAny(text, "'\"`\\!?*") || strings.Contains(text, "$(") {
		return "", false
	}

	if !strings.Contains(text, "$") {
		// We still need to check for bash keywords if we want to fallback
		// (Same as resolveBash does).
		if strings.Contains(text, "if ") || strings.Contains(text, "case ") || strings.Contains(text, "for ") || strings.Contains(text, "while ") || strings.Contains(text, "&&") || strings.Contains(text, "||") || strings.Contains(text, "elif ") {
			return "", false
		}
		return text, true
	}

	// Fallback if keywords are found. Note: resolveBash also only does simple string matching.
	if strings.Contains(text, "if ") || strings.Contains(text, "case ") || strings.Contains(text, "for ") || strings.Contains(text, "while ") || strings.Contains(text, "&&") || strings.Contains(text, "||") || strings.Contains(text, "elif ") {
		return "", false
	}

	var buf strings.Builder
	buf.Grow(len(text) * 2)

	i := 0
	for i < len(text) {
		c := text[i]
		if c != '$' {
			buf.WriteByte(c)
			i++
			continue
		}

		i++
		if i >= len(text) {
			buf.WriteByte('$')
			break
		}

		if text[i] == '{' {
			i++
			start := i
			complex := false
			for i < len(text) && text[i] != '}' {
				if !isAlnumUnderscore(text[i]) {
					complex = true
					break
				}
				i++
			}
			if i >= len(text) || complex {
				return "", false
			}
			varName := text[start:i]
			buf.WriteString(variables[varName])
			i++ // skip }
		} else if isAlphaUnderscore(text[i]) {
			start := i
			for i < len(text) && isAlnumUnderscore(text[i]) {
				i++
			}
			varName := text[start:i]
			buf.WriteString(variables[varName])
		} else {
			// e.g. $$, $!, $?, $#
			return "", false
		}
	}

	return buf.String(), true
}

// resolveBash replaces ${VAR} and evaluates bash code in text using mvdan.cc/sh.
func resolveBash(ctx context.Context, text string, variables map[string]string, opts ...interp.RunnerOption) string {
	if !strings.Contains(text, "$") && !strings.Contains(text, "if ") && !strings.Contains(text, "case ") && !strings.Contains(text, "for ") && !strings.Contains(text, "while ") && !strings.Contains(text, "&&") && !strings.Contains(text, "||") && !strings.Contains(text, "elif ") {
		return text
	}

	var env []string
	for k, v := range variables {
		env = append(env, k+"="+v)
	}
	environ := expand.ListEnviron(env...)

	parser := syntax.NewParser()

	// Parse as a full file and evaluate
	file, err := parser.Parse(strings.NewReader(text), "")
	if err == nil && len(file.Stmts) > 0 {
		var buf bytes.Buffer

		runnerOpts := []interp.RunnerOption{
			interp.Env(environ),
			interp.StdIO(nil, &buf, nil),
			interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
				return func(execCtx context.Context, args []string) error {
					if len(args) > 0 && args[0] == "echo" {
						return next(execCtx, args)
					}
					// Deny all other external commands
					return fmt.Errorf("external command execution denied: %s", args[0])
				}
			}),
		}
		runnerOpts = append(runnerOpts, opts...)

		runner, err := interp.New(runnerOpts...)
		if err == nil {
			err = runner.Run(ctx, file)
			if err == nil {
				// if there was output, return the output instead of text
				out := buf.String()
				if len(out) > 0 {
					return strings.TrimSuffix(out, "\n")
				}
			}
		}
	}

	// For standard variable substitution (preserving quotes), evaluate it as a string inside a dummy assignment
	file2, err2 := parser.Parse(strings.NewReader("dummy=\""+strings.ReplaceAll(text, "\"", "\\\"")+"\""), "")
	if err2 == nil && len(file2.Stmts) > 0 {
		if call, ok := file2.Stmts[0].Cmd.(*syntax.CallExpr); ok && len(call.Assigns) > 0 {
			cfg := &expand.Config{
				Env: environ,
			}
			val, err := expand.Literal(cfg, call.Assigns[0].Value)
			if err == nil {
				return val
			}
		}
	}

	return text
}
