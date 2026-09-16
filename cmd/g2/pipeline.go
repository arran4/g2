package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/arran4/g2/pipeline"
)

// stringStringMapFlag implements flag.Value to parse multiple -s KEY=VALUE args
type stringStringMapFlag map[string]string

func (m stringStringMapFlag) String() string {
	var pairs []string
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, " ")
}

func (m stringStringMapFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("substitution must be in KEY=VALUE format, got: %s", value)
	}
	m[parts[0]] = parts[1]
	return nil
}

func (cfg *MainArgConfig) cmdPipeline(args []string) error {
	fs := flag.NewFlagSet("pipeline", flag.ExitOnError)
	subs := make(stringStringMapFlag)
	fs.Var(&subs, "s", "Variable substitutions in KEY=VALUE format (can be specified multiple times)")

	fs.Usage = func() {
		fmt.Printf("Usage: g2 pipeline [flags] <pipeline_string>\n\n")
		fmt.Printf("Evaluates a data extraction pipeline expression.\n\n")
		fmt.Printf("Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("pipeline command requires exactly one argument: the pipeline expression")
	}

	pipelineStr := fs.Arg(0)

	var err error
	pipelineStr, err = pipeline.PerformSubstitutions(pipelineStr, subs)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	evaluator := pipeline.NewEvaluator(nil) // Uses http.DefaultClient
	val, err := evaluator.Evaluate(pipelineStr)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	if val != nil && !val.IsEmpty {
		if val.IsList() {
			for _, item := range val.List {
				if item != nil {
					fmt.Println(item)
				}
			}
		} else {
			strVal := val.GetString()
			fmt.Println(strVal)
		}
	}

	return nil
}
