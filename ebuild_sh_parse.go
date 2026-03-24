package g2

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ShParseData represents the data extracted from an ebuild using sh/v3 syntax.
type ShParseData struct {
	Vars map[string]string `json:"Vars"`
	// In the future, we could add Functions, Dependencies, etc.
}

// ShParseEbuild parses the ebuild using mvdan.cc/sh and extracts variables.
func ShParseEbuild(r io.Reader, filename string) (*ShParseData, error) {
	parser := syntax.NewParser(
		syntax.Variant(syntax.LangBash),
		syntax.KeepComments(true),
	)

	f, err := parser.Parse(r, filename)
	if err != nil {
		return nil, fmt.Errorf("sh parsing failed: %w", err)
	}

	data := &ShParseData{
		Vars: make(map[string]string),
	}

	for _, stmt := range f.Stmts {
		syntax.Walk(stmt.Cmd, func(node syntax.Node) bool {
			switch x := node.(type) {
			case *syntax.Assign:
				if x.Name != nil {
					key := x.Name.Value
					var valBuilder strings.Builder
					if x.Value != nil {
						for _, part := range x.Value.Parts {
							if lit, ok := part.(*syntax.Lit); ok {
								valBuilder.WriteString(lit.Value)
							} else if dblQuote, ok := part.(*syntax.DblQuoted); ok {
								for _, dqPart := range dblQuote.Parts {
									if lit, ok := dqPart.(*syntax.Lit); ok {
										valBuilder.WriteString(lit.Value)
									} else if param, ok := dqPart.(*syntax.ParamExp); ok {
										valBuilder.WriteString("${" + param.Param.Value + "}")
									}
								}
							} else if sglQuote, ok := part.(*syntax.SglQuoted); ok {
								valBuilder.WriteString(sglQuote.Value)
							}
						}
					}
					data.Vars[key] = valBuilder.String()
				}
			}
			return true
		})
	}

	return data, nil
}

// ShParseDataToJSON serializes the parsed data to JSON.
func ShParseDataToJSON(data *ShParseData) ([]byte, error) {
	return json.MarshalIndent(data, "", "\t")
}
