package g2

import (
	"context"
	"fmt"
	"os"
	"strings"

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

func ParseMakeConfContent(content, filename string) (map[string]string, error) {
    parser := syntax.NewParser()
	f, err := parser.Parse(strings.NewReader(content), filename)
	if err != nil {
		return nil, fmt.Errorf("parsing make.conf: %w", err)
	}

	vars := make(map[string]string)
	printer := syntax.NewPrinter()

	processAssign := func(assign *syntax.Assign) {
	    if assign.Name != nil && assign.Value != nil {
	        name := assign.Name.Value
	        var b strings.Builder
            _ = printer.Print(&b, assign.Value)
            valStr := b.String()

            // To evaluate string values that don't natively contain bash evaluation triggers (like "$"),
            // we wrap the value in an echo statement with a dummy variable to force a complete evaluation
            // via the resolveBash runner, which handles string interpolation and quote trimming.
            script := fmt.Sprintf("echo %s${_G2_DUMMY_}", valStr)
            val := resolveBash(context.Background(), script, vars)
            vars[name] = val
	    }
	}

	for _, stmt := range f.Stmts {
	    switch cmd := stmt.Cmd.(type) {
	    case *syntax.CallExpr:
	        for _, assign := range cmd.Assigns {
	            processAssign(assign)
	        }
	    case *syntax.DeclClause:
	        for _, assign := range cmd.Args {
	            processAssign(assign)
	        }
	    }
	}

	return vars, nil
}
