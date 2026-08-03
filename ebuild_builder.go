package g2

import (
	"fmt"
	"strings"
)

// EbuildBuilder provides a fluent interface for generating ebuilds.
type EbuildBuilder struct {
	e *Ebuild
}

// EbuildBuilderOption represents an option that modifies the ebuild.
type EbuildBuilderOption interface {
	apply(b *EbuildBuilder)
}

// BuildEbuild constructs an ebuild programmatically using the provided options.
func BuildEbuild(opts ...any) *Ebuild {
	b := &EbuildBuilder{
		e: &Ebuild{
			Vars:      make(map[string]string),
			Functions: make(map[string]AST),
			Mode:      ParseFull, // Ensure SRC_URI gets printed properly
		},
	}
	b.Apply(opts...)
	return b.e
}

// Apply parses and applies a variadic list of options.
func (b *EbuildBuilder) Apply(opts ...any) *EbuildBuilder {
	for _, opt := range opts {
		switch o := opt.(type) {
		case string: // raw text fallback maybe?
		case EbuildHeader:
			b.e.EbuildHeader = strings.TrimSpace(string(o))
		case EbuildEAPI:
			b.SetVariable("EAPI", string(o))
		case EbuildVar:
			b.SetVariable(o.Key, o.Value)
		case EbuildFunc:
			b.SetFunction(o.Name, o.Body)
		case EbuildSrcURI:
			b.AddSrcURI(o.URL, o.Filename)
		case EbuildInherit:
			b.SetVariable("INHERIT", strings.Join(o, " "))
		case EbuildRestrict:
			b.SetVariable("RESTRICT", strings.Join(o, " "))
		case EbuildRequiredUse:
			b.SetVariable("REQUIRED_USE", string(o))
		case EbuildSlot:
			b.SetVariable("SLOT", string(o))
		case EbuildDescription:
			b.SetVariable("DESCRIPTION", string(o))
		case EbuildHomepage:
			b.SetVariable("HOMEPAGE", string(o))
		case EbuildLicense:
			b.SetVariable("LICENSE", string(o))
		case EbuildKeywords:
			b.SetVariable("KEYWORDS", strings.Join(o, " "))
		case EbuildIUse:
			b.SetVariable("IUSE", strings.Join(o, " "))
		case EbuildRDepend:
			b.SetVariable("RDEPEND", string(o))
		case EbuildDepend:
			b.SetVariable("DEPEND", string(o))
		case EbuildBDepend:
			b.SetVariable("BDEPEND", string(o))
		case EbuildPDepend:
			b.SetVariable("PDEPEND", string(o))
		case EbuildSrcURIRaw:
			b.SetVariable("SRC_URI", string(o))
		default:
			if bo, ok := opt.(EbuildBuilderOption); ok {
				bo.apply(b)
			} else if slice, ok := opt.([]any); ok {
				b.Apply(slice...)
			} else {
				panic(fmt.Sprintf("unknown option type %T in EbuildBuilder", opt))
			}
		}
	}
	return b
}

type EbuildHeader string
type EbuildEAPI string
type EbuildVar struct{ Key, Value string }
type EbuildFunc struct{ Name, Body string }
type EbuildSrcURI struct{ URL, Filename string }
type EbuildSrcURIRaw string
type EbuildInherit []string
type EbuildRestrict []string
type EbuildRequiredUse string
type EbuildSlot string
type EbuildDescription string
type EbuildHomepage string
type EbuildLicense string
type EbuildKeywords []string
type EbuildIUse []string
type EbuildRDepend string
type EbuildDepend string
type EbuildBDepend string
type EbuildPDepend string

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
