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

type ResolveBashOption func(*ResolveBashOptions)

type ResolveBashOptions struct {
	InterpOptions []interp.RunnerOption
	FastPath      bool
	Variables     map[string]string
}

func WithInterpOption(opt interp.RunnerOption) ResolveBashOption {
	return func(o *ResolveBashOptions) {
		o.InterpOptions = append(o.InterpOptions, opt)
	}
}

func WithFastPath() ResolveBashOption {
	return func(o *ResolveBashOptions) {
		o.FastPath = true
	}
}

func WithVars(vars map[string]string) ResolveBashOption {
	return func(o *ResolveBashOptions) {
		if o.Variables == nil {
			o.Variables = make(map[string]string)
		}
		for k, v := range vars {
			o.Variables[k] = v
		}
	}
}

// resolveBash replaces ${VAR} and evaluates bash code in text using mvdan.cc/sh.
// If WithFastPath() is passed, it attempts a pre-emptive fast-path string substitution for simple variables.
func resolveBash(ctx context.Context, text string, variables map[string]string, opts ...ResolveBashOption) string {
	options := ResolveBashOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	if !strings.Contains(text, "$") && !strings.Contains(text, "if ") && !strings.Contains(text, "case ") && !strings.Contains(text, "for ") && !strings.Contains(text, "while ") && !strings.Contains(text, "&&") && !strings.Contains(text, "||") && !strings.Contains(text, "elif ") && !strings.Contains(text, "echo ") && !strings.Contains(text, "printf ") {
		return text
	}

	if options.FastPath {
		// Trade-off: A manual string builder loop is extremely fast but only supports basic ${VAR} or $VAR syntax.
		// It does not support complex Bash parameter expansions (e.g., ${VAR:-default}, ${VAR/x/y}),
		// arrays, command substitution, arithmetic, or execution of logic (if/case/for).
		//
		// Mitigation: To prevent silent failures or incorrect parsing of complex Bash syntax,
		// we employ a highly restrictive character blocklist ("'\"`\\!?*:/#%-=[]+;|<>&^,~@").
		// If ANY of these characters appear, we conservatively bypass this fast path and fall back
		// to the full mvdan.cc/sh/v3/interp interpreter.
		//
		// Note: A side effect of this strict blocklist is that strings containing these characters
		// innocently (e.g., URLs containing `:` or `/`, package names containing `-`) will
		// skip the fast path even if their bash variable usage is simple. This prioritizes
		// absolute correctness over maximum performance coverage.
		if !strings.ContainsAny(text, "()\"'`\\!?*:/#%-=[]+;|<>&^,~@") && !strings.Contains(text, "$$") {
			if !strings.Contains(text, "if ") && !strings.Contains(text, "case ") && !strings.Contains(text, "for ") && !strings.Contains(text, "while ") && !strings.Contains(text, "&&") && !strings.Contains(text, "||") && !strings.Contains(text, "elif ") && !strings.Contains(text, "echo ") && !strings.Contains(text, "printf ") {

				// Defer merged map creation to the last possible moment to save allocations for pure literals
				mergedVars := variables
				if len(options.Variables) > 0 {
					mergedVars = make(map[string]string, len(variables)+len(options.Variables))
					for k, v := range variables {
						mergedVars[k] = v
					}
					for k, v := range options.Variables {
						mergedVars[k] = v
					}
				}

				var buf strings.Builder
				buf.Grow(len(text) * 2)

				i := 0
				complex := false
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
						innerComplex := false
						for i < len(text) && text[i] != '}' {
							if !isAlnumUnderscore(text[i]) {
								innerComplex = true
								break
							}
							i++
						}
						if i >= len(text) || innerComplex {
							complex = true
							break
						}
						varName := text[start:i]
						buf.WriteString(mergedVars[varName])
						i++ // skip }
					} else if isAlphaUnderscore(text[i]) {
						start := i
						for i < len(text) && isAlnumUnderscore(text[i]) {
							i++
						}
						varName := text[start:i]
						buf.WriteString(mergedVars[varName])
					} else {
						// e.g. $$, $!, $?, $#
						complex = true
						break
					}
				}
				if !complex {
					return buf.String()
				}
			}
		}
	}

	mergedVars := variables
	if len(options.Variables) > 0 {
		mergedVars = make(map[string]string, len(variables)+len(options.Variables))
		for k, v := range variables {
			mergedVars[k] = v
		}
		for k, v := range options.Variables {
			mergedVars[k] = v
		}
	}

	var env []string
	for k, v := range mergedVars {
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
		runnerOpts = append(runnerOpts, options.InterpOptions...)

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
