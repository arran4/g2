package g2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// ParseMakeConf evaluates a make.conf file and extracts its variables.
func ParseMakeConf(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseMakeConfContent(string(content), path)
}

// ParseMakeConfContent parses and evaluates the content of a make.conf file
func ParseMakeConfContent(content, filename string) (map[string]string, error) {
	parser := syntax.NewParser()
	f, err := parser.Parse(strings.NewReader(content), filename)
	if err != nil {
		return nil, fmt.Errorf("parsing make.conf: %w", err)
	}

	var outBuf strings.Builder

	runner, err := interp.New(
		interp.StdIO(nil, &outBuf, nil),
		interp.Dir(filepath.Dir(filename)),
		interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return func(execCtx context.Context, args []string) error {
				if len(args) > 0 && args[0] == "echo" {
					return next(execCtx, args)
				}
				// Deny all other external commands
				return fmt.Errorf("external command execution denied in make.conf: %s", args[0])
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating interpreter: %w", err)
	}

	_ = runner.Run(context.Background(), f)

	vars := make(map[string]string)

	rVal := reflect.ValueOf(runner).Elem()
	varsField := rVal.FieldByName("Vars")

	if varsField.IsValid() {
		for _, k := range varsField.MapKeys() {
			name := k.String()

			// Exclude typical bash built-in variables we don't care about
			if name == "PWD" || name == "OLDPWD" || name == "SHLVL" || name == "IFS" || name == "OPTIND" || name == "PS4" {
				continue
			}

			script := fmt.Sprintf(`echo "${%s[@]}"`, name)
			dumpFile, _ := parser.Parse(strings.NewReader(script), "")
			outBuf.Reset()
			_ = runner.Run(context.Background(), dumpFile)

			val := strings.TrimSuffix(outBuf.String(), "\n")
			if val != "" {
				vars[name] = val
			}
		}
	}

	return vars, nil
}
