package g2

import (
	"context"
	"bytes"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// resolveBash replaces ${VAR} and evaluates bash code in text using mvdan.cc/sh.
func resolveBash(text string, variables map[string]string) string {
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
		runner, err := interp.New(
			interp.Env(environ),
			interp.StdIO(nil, &buf, nil),
			interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
				return func(ctx context.Context, args []string) error {
					if len(args) > 0 && args[0] == "echo" {
						return next(ctx, args)
					}
					// Deny all other external commands
					return fmt.Errorf("external command execution denied: %s", args[0])
				}
			}),
		)
		if err == nil {
			err = runner.Run(context.Background(), file)
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
