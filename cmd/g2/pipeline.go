package main

import (
	"flag"
	"fmt"
	"io"
	"os"
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
	return runPipeline(args, os.Stdout, os.Stderr)
}

func runPipeline(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pipeline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	subs := make(stringStringMapFlag)
	fs.Var(&subs, "s", "Variable substitutions in KEY=VALUE format (can be specified multiple times)")

	printUsage := func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Usage: g2 pipeline [flags] <pipeline_string>\n\n")
		_, _ = fmt.Fprintf(w, "Evaluates a data extraction pipeline expression.\n\n")
		_, _ = fmt.Fprintf(w, "Flags:\n")
		fs.SetOutput(w)
		fs.PrintDefaults()
	}
	fs.Usage = func() {}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printUsage(stdout)
			return nil
		}
		_, _ = fmt.Fprintln(stderr, err)
		printUsage(stderr)
		return &ExitError{Code: 2, Err: nil}
	}

	if fs.NArg() != 1 {
		printUsage(stderr)
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
					_, _ = fmt.Fprintln(stdout, item)
				}
			}
		} else {
			strVal := val.GetString()
			if strVal != "" {
				_, _ = fmt.Fprintln(stdout, strVal)
			}
		}
	}

	return nil
}
