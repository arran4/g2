package g2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// ShParseData represents the data extracted from an ebuild using the custom parser.
type ShParseData struct {
	Vars map[string]string `json:"Vars"`
	// Future nodes (Dependencies, Functions) could go here
}

// ShParseEbuild parses the ebuild using the recursive descent ebuild parser.
func ShParseEbuild(r io.Reader, filename string) (*ShParseData, error) {
	ctx := context.Background() // Can be passed in later for robust timeout control
	p := NewEbuildParser(ctx, r)
	ebuild, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("ebuild parsing failed: %w", err)
	}

	data := &ShParseData{
		Vars: ebuild.Variables,
	}

	return data, nil
}

// ShParseDataToJSON serializes the parsed data to JSON.
func ShParseDataToJSON(data *ShParseData) ([]byte, error) {
	return json.MarshalIndent(data, "", "\t")
}
