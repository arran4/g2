package g2

import "strings"

// EbuildBuilder provides a fluent interface for generating ebuilds.
type EbuildBuilder struct {
	e *Ebuild
}

// NewEbuildBuilder creates a new EbuildBuilder.
func NewEbuildBuilder() *EbuildBuilder {
	return &EbuildBuilder{
		e: &Ebuild{
			Vars:      make(map[string]string),
			Functions: make(map[string]AST),
			Mode:      ParseFull, // Ensure SRC_URI gets printed properly
		},
	}
}

// SetHeader sets the copyright and license header at the top of the ebuild.
func (b *EbuildBuilder) SetHeader(header string) *EbuildBuilder {
	b.e.EbuildHeader = strings.TrimSpace(header)
	return b
}

// SetEAPI sets the EAPI variable.
func (b *EbuildBuilder) SetEAPI(eapi string) *EbuildBuilder {
	return b.SetVariable("EAPI", eapi)
}

// SetVariable adds or updates a variable and records its order.
func (b *EbuildBuilder) SetVariable(key, value string) *EbuildBuilder {
	b.e.Vars[key] = value
	b.appendOrder(key)
	return b
}

// SetFunction adds or updates a function and records its order.
func (b *EbuildBuilder) SetFunction(name, body string) *EbuildBuilder {
	b.e.Functions[name] = AST{Value: body}
	b.appendOrder(name)
	return b
}

// AddSrcURI adds a source URI entry.
func (b *EbuildBuilder) AddSrcURI(url, filename string) *EbuildBuilder {
	b.e.SrcUri = append(b.e.SrcUri, URIEntry{
		URL:      url,
		Filename: filename,
	})
	return b
}

// Build returns the constructed Ebuild.
func (b *EbuildBuilder) Build() *Ebuild {
	return b.e
}

func (b *EbuildBuilder) appendOrder(name string) {
	for _, v := range b.e.orderOverride {
		if v == name {
			return
		}
	}
	b.e.orderOverride = append(b.e.orderOverride, name)
}
