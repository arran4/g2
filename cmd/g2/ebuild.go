package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"

	_ "github.com/arran4/g2/lints/ebuild"
)

type CmdEbuildArgConfig struct {
	*MainArgConfig
}

func (cfg *MainArgConfig) cmdEbuild(args []string) error {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "init", "Initialize an ebuild from a template")
		fmt.Printf("\t\t %s \t\t %s\n", "templates", "Manage ebuild templates")
		fmt.Printf("\t\t %s \t\t %s\n", "sh-parse-to-json", "Parse ebuild using shell parser and output JSON")
		fmt.Printf("\t\t %s \t\t %s\n", "as-json", "Parse ebuild using native parser and output JSON")
		fmt.Printf("\t\t %s \t\t %s\n", "check", "A lightweight structural validator for ebuild files (alias: lint)")
		fmt.Printf("\t\t %s \t\t %s\n", "deps", "Extract and format dependency fields")
		fmt.Printf("\t\t %s \t\t %s\n", "query", "Query specific fields from parsed output")
	}

	config := &CmdEbuildArgConfig{
		MainArgConfig: cfg,
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	switch cmd {
	case "check", "lint":
		if err := config.cmdEbuildCheck(fs.Args()[1:]); err != nil {
			return fmt.Errorf("ebuild %s: %w", cmd, err)
		}
	case "init":
		if err := config.cmdEbuildInit(fs.Args()[1:]); err != nil {
			return fmt.Errorf("ebuild init: %w", err)
		}
	case "as-json":
		if err := config.cmdEbuildAsJson(fs.Args()[1:]); err != nil {
			return fmt.Errorf("ebuild as-json: %w", err)
		}
	case "sh-parse-to-json":
		if err := config.cmdEbuildShParseToJson(fs.Args()[1:]); err != nil {
			return fmt.Errorf("ebuild sh-parse-to-json: %w", err)
		}
	case "templates":
		if err := config.cmdEbuildTemplates(fs.Args()[1:]); err != nil {
			return fmt.Errorf("ebuild templates: %w", err)
		}
	case "deps":
		if err := config.cmdEbuildDeps(fs.Args()[1:]); err != nil {
			return fmt.Errorf("ebuild deps: %w", err)
    }
	case "query":
		if err := config.cmdEbuildQuery(fs.Args()[1:]); err != nil {
			return fmt.Errorf("ebuild query: %w", err)
		}
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(-1)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
	return nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)

	// Basic flags
	eapi := fs.String("eapi", "8", "EAPI version")
	desc := fs.String("description", "A short description", "DESCRIPTION")
	home := fs.String("homepage", "https://example.com", "HOMEPAGE")
	srcURI := fs.String("src-uri", "https://example.com/${P}.tar.gz", "SRC_URI")
	license := fs.String("license", "", "LICENSE")
	slot := fs.String("slot", "0", "SLOT")
	keywords := fs.String("keywords", "~amd64 ~x86", "KEYWORDS")
	depend := fs.String("depend", "", "DEPEND")
	rdepend := fs.String("rdepend", "", "RDEPEND")
	bdepend := fs.String("bdepend", "", "BDEPEND")

	// Template-specific flags
	pyCompat := fs.String("python-compat", "python3_{10..12}", "PYTHON_COMPAT (for python templates)")
	pep517 := fs.String("pep517", "setuptools", "DISTUTILS_USE_PEP517 (for python templates)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: g2 ebuild init <ebuild name>.ebuild")
	}

	filename := fs.Arg(0)
	tmpl := getTemplate("default")
	if tmpl == nil {
		return fmt.Errorf("default template not found")
	}

	params := EbuildParams{
		EAPI:         *eapi,
		Description:  *desc,
		Homepage:     *home,
		SrcURI:       *srcURI,
		License:      *license,
		Slot:         *slot,
		Keywords:     *keywords,
		Depend:       *depend,
		RDepend:      *rdepend,
		BDepend:      *bdepend,
		PythonCompat: *pyCompat,
		Pep517:       *pep517,
	}

	content, err := tmpl.Execute(params)
	if err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	log.Printf("Initialized %s with default template", filename)
	return nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildTemplates(args []string) error {
	fs := flag.NewFlagSet("templates", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "list", "List available ebuild templates")
		fmt.Printf("\t\t %s \t\t %s\n", "init", "Initialize an ebuild from a specific template")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	switch cmd {
	case "list":
		if err := cfg.cmdEbuildTemplatesList(fs.Args()[1:]); err != nil {
			return err
		}
	case "init":
		if err := cfg.cmdEbuildTemplatesInit(fs.Args()[1:]); err != nil {
			return err
		}
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}

	return nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildTemplatesList(args []string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION")
	for _, t := range ebuildTemplates {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Description)
	}
	_ = w.Flush()
	return nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildTemplatesInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)

	// Basic flags
	eapi := fs.String("eapi", "8", "EAPI version")
	desc := fs.String("description", "A short description", "DESCRIPTION")
	home := fs.String("homepage", "", "HOMEPAGE")
	srcURI := fs.String("src-uri", "https://example.com/${P}.tar.gz", "SRC_URI")
	license := fs.String("license", "", "LICENSE")
	slot := fs.String("slot", "0", "SLOT")
	keywords := fs.String("keywords", "~amd64 ~x86", "KEYWORDS")
	depend := fs.String("depend", "", "DEPEND")
	rdepend := fs.String("rdepend", "", "RDEPEND")
	bdepend := fs.String("bdepend", "", "BDEPEND")

	// Template-specific flags
	pyCompat := fs.String("python-compat", "python3_{10..12}", "PYTHON_COMPAT (for python templates)")
	pep517 := fs.String("pep517", "setuptools", "DISTUTILS_USE_PEP517 (for python templates)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 2 {
		return fmt.Errorf("usage: g2 ebuild templates init <template name> <ebuild name>.ebuild")
	}

	templateName := fs.Arg(0)
	filename := fs.Arg(1)

	tmpl := getTemplate(templateName)
	if tmpl == nil {
		return fmt.Errorf("template %q not found", templateName)
	}

	params := EbuildParams{
		EAPI:         *eapi,
		Description:  *desc,
		Homepage:     *home,
		SrcURI:       *srcURI,
		License:      *license,
		Slot:         *slot,
		Keywords:     *keywords,
		Depend:       *depend,
		RDepend:      *rdepend,
		BDepend:      *bdepend,
		PythonCompat: *pyCompat,
		Pep517:       *pep517,
	}

	content, err := tmpl.Execute(params)
	if err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	log.Printf("Initialized %s with template %q", filename, templateName)
	return nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildShParseToJson(args []string) error {
	fs := flag.NewFlagSet("sh-parse-to-json", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: g2 ebuild sh-parse-to-json <ebuild file>")
	}
	filename := fs.Arg(0)

	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer func() { _ = f.Close() }()

	ebuild, err := g2.ParseEbuild(os.DirFS(filepath.Dir(filename)), filepath.Base(filename), g2.ParseVariables)
	if err != nil {
		return fmt.Errorf("parsing ebuild %s: %w", filename, err)
	}

	jsonBytes, err := json.MarshalIndent(ebuild.Vars, "", "\t")
	if err != nil {
		return fmt.Errorf("serializing to json: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}

type DepNodeJSON struct {
	Type      string         `json:"type"`
	Value     string         `json:"value,omitempty"`
	Flag      string         `json:"flag,omitempty"`
	IsNegated bool           `json:"is_negated,omitempty"`
	Children  []*DepNodeJSON `json:"children,omitempty"`
}

func convertDepNodeToJSON(node g2.DepNode) *DepNodeJSON {
	switch n := node.(type) {
	case g2.DepString:
		return &DepNodeJSON{Type: "string", Value: string(n)}
	case g2.DepAnyOf:
		var children []*DepNodeJSON
		for _, c := range n.Children {
			children = append(children, convertDepNodeToJSON(c))
		}
		return &DepNodeJSON{Type: "any-of", Children: children}
	case g2.DepAllOf:
		var children []*DepNodeJSON
		for _, c := range n.Children {
			children = append(children, convertDepNodeToJSON(c))
		}
		return &DepNodeJSON{Type: "all-of", Children: children}
	case g2.DepUseConditional:
		var children []*DepNodeJSON
		for _, c := range n.Children {
			children = append(children, convertDepNodeToJSON(c))
		}
		return &DepNodeJSON{Type: "use-conditional", Flag: n.Flag, IsNegated: n.IsNegated, Children: children}
	}
	return nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildDeps(args []string) error {
	fs := flag.NewFlagSet("deps", flag.ExitOnError)
	flatten := fs.Bool("flatten", false, "Flatten dependency trees into a list")
	atomsOnly := fs.Bool("atoms-only", false, "Extract only package atoms without operators (e.g. dev-libs/foo instead of >=dev-libs/foo-1.2.3)")
	jsonOutput := fs.Bool("json", false, "Output results in JSON format")
	onePerLine := fs.Bool("one-per-line", false, "Output each dependency on a new line (only applies to text output)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: g2 ebuild deps [flags] <ebuild files...>")
	}

	depFields := []string{"DEPEND", "RDEPEND", "BDEPEND", "PDEPEND", "IDEPEND"}

	// To structure JSON across files
	type FileDeps struct {
		File string         `json:"file"`
		Deps map[string]any `json:"deps"`
	}
	var allFilesDeps []FileDeps

	for _, filename := range fs.Args() {
		dir := filepath.Dir(filename)
		base := filepath.Base(filename)

		ebuild, err := g2.ParseEbuild(os.DirFS(dir), base, g2.ParseVariables)
		if err != nil {
			log.Printf("failed to parse %s: %v", filename, err)
			continue
		}

		fileDeps := FileDeps{
			File: filename,
			Deps: make(map[string]any),
		}

		for _, field := range depFields {
			val := ebuild.Vars[field]
			if val == "" {
				continue
			}

			if *flatten {
				tree := g2.ParseDepTree(val)
				flatDeps, err := tree.Evaluate(g2.IgnoreUseFlags(true))
				if err != nil {
					log.Printf("error evaluating %s in %s: %v", field, filename, err)
					continue
				}

				if *atomsOnly {
					for i, d := range flatDeps {
						flatDeps[i] = g2.ExtractPackageNameFromDep(d)
					}
				}

				if *jsonOutput {
					fileDeps.Deps[field] = flatDeps
				} else {
					if *onePerLine {
						fmt.Printf("### %s - %s\n", filename, field)
						for _, d := range flatDeps {
							fmt.Println(d)
						}
						fmt.Println()
					} else {
						fmt.Printf("### %s - %s\n%s\n\n", filename, field, strings.Join(flatDeps, " "))
					}
				}
			} else {
				if *jsonOutput {
					tree := g2.ParseDepTree(val)
					var children []*DepNodeJSON
					for _, n := range tree.Nodes {
						children = append(children, convertDepNodeToJSON(n))
					}
					fileDeps.Deps[field] = children
				} else {
					if *onePerLine {
						// For raw string output, splitting by space is a naive approach,
						// but since users requested one-per-line for standard format,
						// printing evaluated/flattened is better, or splitting the raw string.
						fmt.Printf("### %s - %s\n", filename, field)
						for _, token := range strings.Fields(val) {
							fmt.Println(token)
						}
						fmt.Println()
					} else {
						fmt.Printf("### %s - %s\n%s\n\n", filename, field, val)
					}
				}
			}
		}

		if *jsonOutput {
			allFilesDeps = append(allFilesDeps, fileDeps)
		}
	}

	if *jsonOutput {
		out, err := json.MarshalIndent(allFilesDeps, "", "\t")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(out))
	}

	return nil
}
func (cfg *CmdEbuildArgConfig) cmdEbuildAsJson(args []string) error {
	fs := flag.NewFlagSet("as-json", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: g2 ebuild as-json <ebuild files...>")
	}

	var ebuilds []*g2.Ebuild
	for _, filename := range fs.Args() {
		// ebuild parser expects fs.FS and filename.
		// we can use os.DirFS of the file's directory and its base name.
		dir := filepath.Dir(filename)
		base := filepath.Base(filename)

		ebuild, err := g2.ParseEbuild(os.DirFS(dir), base, g2.ParseFull)
		if err != nil {
			return fmt.Errorf("parsing ebuild %s: %w", filename, err)
		}
		ebuilds = append(ebuilds, ebuild)
	}

	var jsonBytes []byte
	var err error
	if len(ebuilds) == 1 {
		jsonBytes, err = json.MarshalIndent(ebuilds[0], "", "\t")
	} else {
		jsonBytes, err = json.MarshalIndent(ebuilds, "", "\t")
	}

	if err != nil {
		return fmt.Errorf("serializing to json: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}
func (cfg *CmdEbuildArgConfig) cmdEbuildCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	format := fs.String("format", "text", "Output format: text or json")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: g2 ebuild check <ebuild files...>")
	}

	hasErrors := false
	var allResults []lints.LintResult

	for _, filename := range fs.Args() {
		dir := filepath.Dir(filename)
		base := filepath.Base(filename)

		ebuild, err := g2.ParseEbuild(os.DirFS(dir), base, g2.ParseFull)
		if err != nil {
			return fmt.Errorf("parsing ebuild %s: %w", filename, err)
		}

		// Convert parser warnings to lint results
		for _, w := range ebuild.ParseWarnings {
			res := lints.LintResult{
				RuleMetadata: lints.RuleMetadata{
					ID:          "ParserWarning",
					Title:       "Parser Warning",
					Description: "A warning generated by the native ebuild parser",
					Severity:    lints.SeverityWarning,
					Source:      lints.SourceG2,
				},
				Message: w,
				File:    filename,
			}
			allResults = append(allResults, res)
		}

		// Try to deduce category and package from path if it looks like a standard gentoo repo
		// e.g., app-editors/vim/vim-9.0.ebuild
		parts := strings.Split(filepath.ToSlash(dir), "/")
		category := "unknown-category"
		pkgName := "unknown-package"
		if len(parts) >= 2 {
			category = parts[len(parts)-2]
			pkgName = parts[len(parts)-1]
		}

		// Reconstruct PN and PV from filename for better linting context
		pkgData := g2.PackageData{
			Name:     pkgName,
			Category: category,
		}

		vData := g2.VersionData{
			Version: strings.TrimSuffix(base, ".ebuild"), // Approximate, sufficient for some lints
			Ebuild:  ebuild,
		}

		// Adjust version by stripping PN if it matches
		if strings.HasPrefix(vData.Version, pkgName+"-") {
			vData.Version = strings.TrimPrefix(vData.Version, pkgName+"-")
		}

		pkgData.Versions = append(pkgData.Versions, vData)

		lintWarnings := lints.PerformLintingResults(dir, &pkgData)

		for _, w := range lintWarnings {
			w.File = filename
			if w.Package == "" {
				w.Package = category + "/" + pkgName
			}
			allResults = append(allResults, w)
		}
	}

	// Filter warnings
	var filteredWarnings []lints.LintResult
	for _, w := range allResults {
		if *severityFilter != "" && !strings.EqualFold(string(w.RuleMetadata.Severity), *severityFilter) {
			continue
		}
		if *sourceFilter != "" && string(w.RuleMetadata.Source) != *sourceFilter {
			continue
		}
		if *tagFilter != "" {
			hasTag := false
			for _, t := range w.RuleMetadata.Tags {
				if t == *tagFilter {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		filteredWarnings = append(filteredWarnings, w)
	}

	if len(filteredWarnings) > 0 {
		hasErrors = true
	}

	if *format == "json" {
		out, err := json.MarshalIndent(filteredWarnings, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting json: %w", err)
		}
		fmt.Println(string(out))
	} else {
		for _, w := range filteredWarnings {
			fmt.Printf("%s: [%s] %s\n", w.File, w.RuleMetadata.Severity, w.Message)
		}
	}

	if hasErrors {
		return fmt.Errorf("linting found errors")
	}

	if *format == "text" {
		fmt.Println("Check passed successfully.")
	}
  return nil
}


func (cfg *CmdEbuildArgConfig) cmdEbuildQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	key := fs.String("key", "", "Key to query (e.g. EAPI, DESCRIPTION, IUSE, SRC_URI)")
	format := fs.String("format", "", "Output format (e.g. lines)")

	// Custom flag parsing to allow `<ebuild file>` before flags like `--key`
	// Go's flag.Parse() stops at the first non-flag argument.
	var flagArgs []string
	var positionalArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			// Handle flag values that don't use '=' (e.g., --key EAPI instead of --key=EAPI)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++ // Skip the value in the next iteration
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if len(positionalArgs) != 1 {
		return fmt.Errorf("usage: g2 ebuild query <ebuild file> --key <key> [--format lines]")
	}

	if *key == "" {
		return fmt.Errorf("missing --key argument")
	}

	filename := positionalArgs[0]
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)

	ebuild, err := g2.ParseEbuild(os.DirFS(dir), base, g2.ParseFull)
	if err != nil {
		return fmt.Errorf("parsing ebuild %s: %w", filename, err)
	}

	val, ok := ebuild.Vars[*key]
	if !ok {
		return nil
	}

	if *format == "lines" {
		fields := strings.Fields(val)
		for _, field := range fields {
			fmt.Println(field)
		}
	} else {
		fmt.Println(val)
	}

	return nil
}
