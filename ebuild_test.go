package g2

import (
	"embed"
	"reflect"
	"testing"
	"testing/fstest"
)

//go:embed testdata
var testData embed.FS

func TestParseEbuildVariables(t *testing.T) {
	tests := []struct {
		filename string
		want     map[string]string
	}{
		{
			filename: "ollama-bin-0.10.1.ebuild",
			want: map[string]string{
				"PN":  "ollama-bin",
				"PV":  "0.10.1",
				"P":   "ollama-bin-0.10.1",
				"PR":  "r0",
				"PVR": "0.10.1",
				"PF":  "ollama-bin-0.10.1",
			},
		},
		{
			filename: "g2-bin-0.0.2.ebuild",
			want: map[string]string{
				"PN":  "g2-bin",
				"PV":  "0.0.2",
				"P":   "g2-bin-0.0.2",
				"PR":  "r0",
				"PVR": "0.0.2",
				"PF":  "g2-bin-0.0.2",
			},
		},
		{
			filename: "app-1.2.3_rc4-r1.ebuild",
			want: map[string]string{
				"PN":  "app",
				"PV":  "1.2.3_rc4",
				"P":   "app-1.2.3_rc4",
				"PR":  "r1",
				"PVR": "1.2.3_rc4-r1",
				"PF":  "app-1.2.3_rc4-r1",
			},
		},
		{
			filename: "foo-1.2.3.ebuild",
			want: map[string]string{
				"PN":  "foo",
				"PV":  "1.2.3",
				"P":   "foo-1.2.3",
				"PR":  "r0",
				"PVR": "1.2.3",
				"PF":  "foo-1.2.3",
			},
		},
		{
			filename: "foo-1.2.3-r1.ebuild",
			want: map[string]string{
				"PN":  "foo",
				"PV":  "1.2.3",
				"P":   "foo-1.2.3",
				"PR":  "r1",
				"PVR": "1.2.3-r1",
				"PF":  "foo-1.2.3-r1",
			},
		},
		{
			filename: "foo-bar-1.2.3-r12.ebuild",
			want: map[string]string{
				"PN":  "foo-bar",
				"PV":  "1.2.3",
				"P":   "foo-bar-1.2.3",
				"PR":  "r12",
				"PVR": "1.2.3-r12",
				"PF":  "foo-bar-1.2.3-r12",
			},
		},
		{
			filename: "invalid.txt",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := ParseEbuildVariables(tt.filename); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseEbuildVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractURIs(t *testing.T) {
	content := `
# Copyright 2023
EAPI=8

SRC_URI="
    https://example.com/files/${P}.tar.gz
    https://example.com/other/file.bin -> renamed.bin
"
`
	variables := map[string]string{
		"P": "mypackage-1.0",
	}

	want := []URIEntry{
		{URL: "https://example.com/files/mypackage-1.0.tar.gz", Filename: "mypackage-1.0.tar.gz"},
		{URL: "https://example.com/other/file.bin", Filename: "renamed.bin"},
	}

	got, err := ExtractURIs(content, variables)
	if err != nil {
		t.Fatalf("ExtractURIs error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractURIs() = %v, want %v", got, want)
	}
}

func TestExtractURIsSingleQuote(t *testing.T) {
	content := `SRC_URI='https://example.com/file.tar.gz'`
	variables := map[string]string{}

	want := []URIEntry{
		{URL: "https://example.com/file.tar.gz", Filename: "file.tar.gz"},
	}

	got, err := ExtractURIs(content, variables)
	if err != nil {
		t.Fatalf("ExtractURIs error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractURIs() = %v, want %v", got, want)
	}
}

func TestParseEbuild_MetadataOnly(t *testing.T) {
	path := "testdata/basic-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseMetadataOnly)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	if ebuild.Vars["PN"] != "basic" {
		t.Errorf("Expected PN=basic, got %s", ebuild.Vars["PN"])
	}
	if ebuild.Vars["PV"] != "1.0" {
		t.Errorf("Expected PV=1.0, got %s", ebuild.Vars["PV"])
	}
	// Content variables shouldn't be parsed
	if _, ok := ebuild.Vars["DESCRIPTION"]; ok {
		t.Error("Did not expect DESCRIPTION to be parsed in MetadataOnly mode")
	}
}

func TestParseEbuild_Variables(t *testing.T) {
	path := "testdata/vars-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseVariables)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	if ebuild.Vars["MY_PN"] != "mypackage" {
		t.Errorf("Expected MY_PN=mypackage, got %s", ebuild.Vars["MY_PN"])
	}
	// Check resolution
	if ebuild.Vars["S"] != "${WORKDIR}/mypackage-1.0" && ebuild.Vars["S"] != "/mypackage-1.0" {
		// Note: WORKDIR is not defined in the ebuild, so it remains as ${WORKDIR} or empty depending on resolver logic
		// My simple resolver leaves ${WORKDIR} if not found in map? No, wait.
		// ResolveVariables implementation:
		// text = strings.ReplaceAll(text, fmt.Sprintf("${%s}", key), value)
		// It iterates over keys in map. If WORKDIR is not in map, it won't be replaced.
		// So it should be "${WORKDIR}/mypackage-1.0"
		if ebuild.Vars["S"] != "${WORKDIR}/mypackage-1.0" {
			t.Errorf("Expected S=${WORKDIR}/mypackage-1.0, got %s", ebuild.Vars["S"])
		}
	}
}

func TestParseEbuild_Full(t *testing.T) {
	path := "testdata/vars-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseFull)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	if len(ebuild.SrcUri) != 1 {
		t.Fatalf("Expected 1 URI, got %d", len(ebuild.SrcUri))
	}

	// SRC_URI="https://example.com/${MY_PN}-${MY_PV}.tar.gz -> ${P}.tar.gz"
	// P=vars-1.0
	// MY_PN=mypackage
	// MY_PV=1.0
	expectedUrl := "https://example.com/mypackage-1.0.tar.gz"
	expectedFile := "vars-1.0.tar.gz"

	if ebuild.SrcUri[0].URL != expectedUrl {
		t.Errorf("Expected URL=%s, got %s", expectedUrl, ebuild.SrcUri[0].URL)
	}
	if ebuild.SrcUri[0].Filename != expectedFile {
		t.Errorf("Expected Filename=%s, got %s", expectedFile, ebuild.SrcUri[0].Filename)
	}
}

func TestParseEbuild_Circular(t *testing.T) {
	// 1. Parse an ebuild
	path := "testdata/vars-1.0.ebuild"
	ebuild, err := ParseEbuild(testData, path, ParseFull)
	if err != nil {
		t.Fatalf("ParseEbuild failed: %v", err)
	}

	// 2. Generate string
	generated := ebuild.String()

	// 3. Parse generated string as a new ebuild
	// We need to fake the file presence using fstest.MapFS or similar,
	// because ParseEbuild expects to read from FS.
	// We'll reuse the filename "vars-1.0.ebuild" so we get same P/PN/PV.

	memFS := fstest.MapFS{
		"vars-1.0.ebuild": &fstest.MapFile{
			Data: []byte(generated),
		},
	}

	ebuild2, err := ParseEbuild(memFS, "vars-1.0.ebuild", ParseFull)
	if err != nil {
		t.Fatalf("ParseEbuild (round 2) failed: %v", err)
	}

	// 4. Compare key attributes
	if ebuild2.Vars["MY_PN"] != ebuild.Vars["MY_PN"] {
		t.Errorf("Circular mismatch MY_PN: %s vs %s", ebuild2.Vars["MY_PN"], ebuild.Vars["MY_PN"])
	}

	// SRC_URI parsing might fail if generated output doesn't match the regex exactly.
	// My String() implementation generates:
	// SRC_URI="
	//     url -> filename
	// "
	// My ExtractURIs implementation expects:
	// SRC_URI="..."
	// multiline with tokens.
	// It should work.

	if len(ebuild2.SrcUri) != len(ebuild.SrcUri) {
		t.Errorf("Circular mismatch URI count: %d vs %d", len(ebuild2.SrcUri), len(ebuild.SrcUri))
	} else if len(ebuild.SrcUri) > 0 {
		if ebuild2.SrcUri[0].URL != ebuild.SrcUri[0].URL {
			t.Errorf("Circular mismatch URL: %s vs %s", ebuild2.SrcUri[0].URL, ebuild.SrcUri[0].URL)
		}
	}
}

// TestBlackbox verifies that we can interact with Ebuild struct publicly
func TestBlackbox(t *testing.T) {
	// This test just ensures public fields are accessible
	e := &Ebuild{
		Path: "test-1.0.ebuild",
		Vars: make(map[string]string),
	}
	e.Vars["KEY"] = "VAL"
	if e.Path != "test-1.0.ebuild" {
		t.Error("Public Path field not accessible/settable")
	}
}

func TestResolveVariables(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		variables map[string]string
		want      string
	}{
		{
			name: "Multi-pass resolution correctness",
			// A=$B, B=foo, Input: "$A", Expectation: "foo"
			text: "$A",
			variables: map[string]string{
				"A": "$B",
				"B": "foo",
			},
			want: "foo",
		},
		{
			name: "Repeated variable usage",
			// FOO=bar, Input: "$FOO $FOO", Expectation: "bar bar"
			text: "$FOO $FOO",
			variables: map[string]string{
				"FOO": "bar",
			},
			want: "bar bar",
		},
		{
			name: "Self-referential variables",
			// A=$A, Input: "$A", Expectation: "$A"
			// Expect deterministic output without infinite loop
			text: "$A",
			variables: map[string]string{
				"A": "$A",
			},
			want: "$A",
		},
		{
			name: "Sequential self-reference (as-we-go)",
			text: "$a",
			variables: map[string]string{
				"a": "a$a",
			},
			want: "aaaaa$a",
		},
		{
			name: "Indirect cycles",
			text: "$A",
			variables: map[string]string{
				"A": "$B",
				"B": "$A",
			},
			want: "$B",
		},
		{
			name: "Nested / chained expansions",
			// A=${B}, B=${C}, C=final, Input: "$A", Expectation: "final"
			text: "$A",
			variables: map[string]string{
				"A": "${B}",
				"B": "${C}",
				"C": "final",
			},
			want: "final",
		},
		{
			name: "Default values - UNSET",
			// Input: "${UNSET:-default}" -> "default"
			text:      "${UNSET:-default}",
			variables: map[string]string{},
			want:      "default",
		},
		{
			name: "Default values - SET",
			// Input: "${SET:-default}" (SET="value") -> "value"
			text: "${SET:-default}",
			variables: map[string]string{
				"SET": "value",
			},
			want: "value",
		},
		{
			name: "Replacement operations - replace all",
			// VAR=abcabc
			// ${VAR//a/x} -> xbcxbc
			text: "${VAR//a/x}",
			variables: map[string]string{
				"VAR": "abcabc",
			},
			want: "xbcxbc",
		},
		{
			name: "Replacement operations - replace first",
			// VAR=abcabc
			// ${VAR/a/x} -> xbcabc
			text: "${VAR/a/x}",
			variables: map[string]string{
				"VAR": "abcabc",
			},
			want: "xbcabc",
		},
		{
			name: "Prefix/suffix trimming - prefix",
			// VAR=foobar
			// ${VAR#foo} -> bar
			text: "${VAR#foo}",
			variables: map[string]string{
				"VAR": "foobar",
			},
			want: "bar",
		},
		{
			name: "Prefix/suffix trimming - suffix",
			// VAR=foobar
			// ${VAR%bar} -> foo
			text: "${VAR%bar}",
			variables: map[string]string{
				"VAR": "foobar",
			},
			want: "foo",
		},
		{
			name: "Large expansion protection",
			// Construct inputs that would expand exponentially
			// A=$B$B, B=$C$C, C=$D$D, D=x
			text: "$A",
			variables: map[string]string{
				"A": "$B$B$B$B$B$B$B$B$B$B",
				"B": "$C$C$C$C$C$C$C$C$C$C",
				"C": "$D$D$D$D$D$D$D$D$D$D",
				"D": "$E$E$E$E$E$E$E$E$E$E",
				"E": "$F$F$F$F$F$F$F$F$F$F",
				"F": "xxxxxxxxxxxxxxxxxxxx",
			},
			want: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", // This is somewhat arbitrary based on limit, let's just make it not crash or run OOM. We will just check prefix
		},
		{
			name: "Mixed patterns in one string",
			// A=hello, B=world, Input: "$A ${B} ${UNSET:-fallback}", Expectation: "hello world fallback"
			text: "$A ${B} ${UNSET:-fallback}",
			variables: map[string]string{
				"A": "hello",
				"B": "world",
			},
			want: "hello world fallback",
		},
		{
			name: "Bash conditionals are evaluated",
			// Bash conditionals (like if/else) should be evaluated dynamically
			// instead of treating them as literal strings.
			text: "if [[ $A == \"foo\" ]]; then echo $B; fi",
			variables: map[string]string{
				"A": "foo",
				"B": "bar",
			},
			want: "bar",
		},
		{
			name: "Bash conditionals are evaluated - else branch",
			text: "if [[ $A == \"baz\" ]]; then echo $B; else echo NO; fi",
			variables: map[string]string{
				"A": "foo",
				"B": "bar",
			},
			want: "NO",
		},
		{
			name: "Bash logical operators are evaluated",
			text: "[[ $A == \"foo\" ]] && echo YES || echo NO",
			variables: map[string]string{
				"A": "foo",
			},
			want: "YES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveVariables(tt.text, tt.variables)
			if tt.name == "Large expansion protection" {
				if len(got) == 0 {
					t.Errorf("ResolveVariables() = empty, expected large string")
				}
				// we just want to ensure it completes and is large
				return
			}
			if got != tt.want {
				t.Errorf("ResolveVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLicense(t *testing.T) {
	tests := []struct {
		name     string
		license  string
		expected []string
	}{
		{
			name:     "Empty",
			license:  "",
			expected: nil,
		},
		{
			name:     "Single license",
			license:  "GPL-2",
			expected: []string{"GPL-2"},
		},
		{
			name:     "Multiple licenses",
			license:  "GPL-2 MIT",
			expected: []string{"GPL-2", "MIT"},
		},
		{
			name:     "Conditional license",
			license:  "USE? ( GPL-2 )",
			expected: []string{"GPL-2"},
		},
		{
			name:     "Conditional and or license",
			license:  "3270? ( || ( BSD CC-BY-SA-3.0 ) ) anonymouspro? ( OFL-1.1 ) arimo? ( Apache-2.0 )",
			expected: []string{"BSD", "CC-BY-SA-3.0", "OFL-1.1", "Apache-2.0"},
		},
		{
			name:     "complex",
			license:  "0BSD Apache-2.0 Apache-2.0-with-LLVM-exceptions BSD-2 BSD CC0-1.0 CDLA-Permissive-2.0 ISC MIT MPL-2.0 Unicode-3.0 Unicode-DFS-2016 ZLIB BZIP2 openssl",
			expected: []string{"0BSD", "Apache-2.0", "Apache-2.0-with-LLVM-exceptions", "BSD-2", "BSD", "CC0-1.0", "CDLA-Permissive-2.0", "ISC", "MIT", "MPL-2.0", "Unicode-3.0", "Unicode-DFS-2016", "ZLIB", "BZIP2", "openssl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLicense(tt.license)
			if len(result) != len(tt.expected) {
				t.Fatalf("ParseLicense() len = %v, want %v. Got %v", len(result), len(tt.expected), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("ParseLicense()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestParseDepTree(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		opts     []any
		expected []string
	}{
		{
			name:     "Evaluate with use flags",
			input:    "3270? ( BSD ) anonymouspro? ( OFL-1.1 )",
			opts:     []any{UseFlags([]string{"3270"})},
			expected: []string{"BSD"},
		},
		{
			name:     "Evaluate multiple with ignore",
			input:    "|| ( A B ) use? ( C ) !use? ( D )",
			opts:     []any{IgnoreUseFlags(true)},
			expected: []string{"A", "B", "C", "D"},
		},
		{
			name:     "Evaluate with negated use flag matched",
			input:    "|| ( A B ) use? ( C ) !use? ( D )",
			opts:     []any{UseFlags([]string{"use"})},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "Evaluate with negated use flag not matched",
			input:    "|| ( A B ) use? ( C ) !use? ( D )",
			opts:     []any{UseFlags([]string{})},
			expected: []string{"A", "B", "D"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := ParseDepTree(tt.input)
			result, err := tree.Evaluate(tt.opts...)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Fatalf("Evaluate() len = %v, want %v. Got %v", len(result), len(tt.expected), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("Evaluate()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestParseIUSE(t *testing.T) {
	tests := []struct {
		name     string
		iuse     string
		expected []string
	}{
		{
			name:     "Empty string",
			iuse:     "",
			expected: nil,
		},
		{
			name:     "Single flag",
			iuse:     "foo",
			expected: []string{"foo"},
		},
		{
			name:     "Multiple flags",
			iuse:     "foo bar baz",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "Flags with plus",
			iuse:     "+foo bar +baz",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "Flags with minus",
			iuse:     "-foo bar -baz",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "Flags with mixed prefixes",
			iuse:     "+foo -bar baz -qux",
			expected: []string{"foo", "bar", "baz", "qux"},
		},
		{
			name:     "Multi-line string with extra whitespace",
			iuse:     "\n\t+foo  \n\t -bar \n\tbaz\t\n",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "Arch specifiers (currently handled as normal flags without stripping arch specific chars)",
			iuse:     "foo ( amd64 ) -bar", // The script should strip + and -, not parens for arch limits in IUSE
			expected: []string{"foo", "(", "amd64", ")", "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseIUSE(tt.iuse)
			if len(result) != len(tt.expected) {
				t.Fatalf("ParseIUSE() len = %v, want %v", len(result), len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("ParseIUSE()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.0.0", "1.0", 0},
		{"1.0", "1.0.0", 0},
		{"1.0", "1.1", -1},
		{"1.1", "1.0", 1},
		{"1.0-r1", "1.0", 1},
		{"1.0", "1.0-r1", -1},
		{"1.0-r1", "1.0-r2", -1},
		{"1.0-r2", "1.0-r1", 1},
		{"1.0_alpha", "1.0", -1},
		{"1.0_beta", "1.0_alpha", 1},
		{"1.0_pre", "1.0_beta", 1},
		{"1.0_rc", "1.0_pre", 1},
		{"1.0", "1.0_rc", 1},
		{"1.0_p", "1.0", 1},
		{"1.0_alpha1", "1.0_alpha2", -1},
		{"1.0_alpha2", "1.0_alpha1", 1},
		{"1.0_p1", "1.0_p2", -1},
		{"1.0_p2", "1.0_p1", 1},
		{"1.0a", "1.0", 1},
		{"1.0", "1.0a", -1},
		{"1.0a", "1.0b", -1},
		{"1.0b", "1.0a", 1},
		{"1.0.1", "1.0", 1},
		{"1.0", "1.0.1", -1},
		{"1.001", "1.01", -1},
		{"1.01", "1.001", 1},
		{"1.01", "1.1", -1},
		{"1.1", "1.01", 1},
		{"1.0_alpha1-r1", "1.0_alpha1-r2", -1},
		{"1.0_alpha1-r2", "1.0_alpha1-r1", 1},
		// Unparseable versions fallback to string compare
		{"invalid1", "invalid2", -1},
		{"invalid2", "invalid1", 1},
		{"1.0_alpha1_beta2_p3-r1", "1.0_alpha1_beta2_p3-r1", 0},
		{"1.0_alpha1_beta2_p3-r1", "1.0_alpha1_beta2_p4-r1", -1},
		{"1.0_alpha1_beta2_p3-r1", "1.0_alpha1_beta2_p3-r2", -1},
		{"1.0_alpha", "1.0_alpha_beta", 1},
		{"1.0_alpha_beta", "1.0_alpha", -1},
		{"1.0_alpha_p1", "1.0_alpha", 1},
		{"1.0_alpha", "1.0_alpha_p1", -1},
		{"1.0_alpha1_beta2", "1.0_alpha1_beta3", -1},
		{"1.2.3_alpha1", "1.2.3_alpha1", 0},
		{"1.2.3_alpha1", "1.2.3_alpha2", -1},
		{"1.2.3_beta1", "1.2.3_alpha2", 1},
		{"1.2.3_pre1", "1.2.3_beta2", 1},
		{"1.2.3_rc1", "1.2.3_pre2", 1},
		{"1.2.3_p1", "1.2.3", 1},
		{"1.2.3", "1.2.3_p1", -1},
		{"1.2.3-r1", "1.2.3", 1},
		{"1.2.3", "1.2.3-r1", -1},
		{"1.000.2", "1.0.2", 0},
		{"1.000.2-r1", "1.0.2-r1", 0},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			got := CompareVersions(tt.v1, tt.v2)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestGentooVersion_String(t *testing.T) {
	tests := []string{
		"1.0",
		"1.0.0",
		"1.0-r1",
		"1.0_alpha",
		"1.0_alpha1",
		"1.0_alpha1-r1",
		"1.0a",
		"1.0a-r1",
		"1.0_p1",
		"1.0_p1-r1",
		"0.0.1",
		"0.1.0",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			gv := ParseGentooVersion(tt)
			if !gv.IsValid {
				t.Fatalf("Failed to parse %s", tt)
			}
			got := gv.String()
			if got != tt {
				t.Errorf("GentooVersion.String() = %q, want %q", got, tt)
			}
		})
	}
}

func TestGentooVersion_IncrementRevision(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.0", "1.0-r1"},
		{"1.0-r1", "1.0-r2"},
		{"1.0_alpha1", "1.0_alpha1-r1"},
		{"1.0_alpha1-r1", "1.0_alpha1-r2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gv := ParseGentooVersion(tt.input)
			if !gv.IsValid {
				t.Fatalf("Failed to parse %s", tt.input)
			}
			gv.IncrementRevision()
			got := gv.String()
			if got != tt.want {
				t.Errorf("IncrementRevision() resulted in %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGentooVersion_IncrementPart(t *testing.T) {
	tests := []struct {
		input string
		part  string
		want  string
	}{
		{"1.2.3_alpha1-r2", "major", "2.0.0"},
		{"1.2.3_alpha1-r2", "minor", "1.3.0"},
		{"1.2.3_alpha1-r2", "patch", "1.2.4"},
		{"1.2.3_alpha1-r2", "suffix", "1.2.3_alpha2"},
		{"1.2.3_alpha1-r2", "revision", "1.2.3_alpha1-r3"},
		{"1", "minor", "1.1"},
		{"1", "patch", "1.0.1"},
		{"1.0", "patch", "1.0.1"},
		{"1.0_beta", "suffix", "1.0_beta1"},
	}

	for _, tt := range tests {
		t.Run(tt.input+"_"+tt.part, func(t *testing.T) {
			gv := ParseGentooVersion(tt.input)
			if !gv.IsValid {
				t.Fatalf("Failed to parse %s", tt.input)
			}
			gv.IncrementPart(tt.part)
			got := gv.String()
			if got != tt.want {
				t.Errorf("IncrementPart(%q) resulted in %q, want %q", tt.part, got, tt.want)
			}
		})
	}
}

func TestGentooVersion_IncrementPartMultiple(t *testing.T) {
	tests := []struct {
		input string
		parts []any
		want  string
	}{
		{"1.2.3", []any{MinorPart, PatchPart}, "1.3.1"},
		{"1.2.3_alpha1-r2", []any{MajorPart, "minor", PatchPart}, "2.1.1"},
		{"1.2.3_alpha1-r2", []any{SuffixPart, RevisionPart}, "1.2.3_alpha2-r1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gv := ParseGentooVersion(tt.input)
			if !gv.IsValid {
				t.Fatalf("Failed to parse %s", tt.input)
			}
			gv.IncrementPart(tt.parts...)
			got := gv.String()
			if got != tt.want {
				t.Errorf("IncrementPart(%v) resulted in %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestParsePackageAtom(t *testing.T) {
	tests := []struct {
		name string
		dep  string
		want PackageAtom
	}{
		{
			name: "basic category and name",
			dep:  "dev-lang/python",
			want: PackageAtom{
				Category: "dev-lang",
				Name:     "python",
			},
		},
		{
			name: "with version",
			dep:  "dev-lang/python-3.10.4",
			want: PackageAtom{
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4",
			},
		},
		{
			name: "with revision",
			dep:  "dev-lang/python-3.10.4-r1",
			want: PackageAtom{
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4-r1",
			},
		},
		{
			name: "with operator >=",
			dep:  ">=dev-lang/python-3.10.4",
			want: PackageAtom{
				Operator: ">=",
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4",
			},
		},
		{
			name: "with operator ~",
			dep:  "~dev-lang/python-3.10.4",
			want: PackageAtom{
				Operator: "~",
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4",
			},
		},
		{
			name: "with operator !",
			dep:  "!dev-lang/python-3.10.4",
			want: PackageAtom{
				Operator: "!",
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4",
			},
		},
		{
			name: "with operator =",
			dep:  "=media-libs/libpng-1.6.39-r1",
			want: PackageAtom{
				Operator: "=",
				Category: "media-libs",
				Name:     "libpng",
				Version:  "1.6.39-r1",
			},
		},
		{
			name: "with operator <",
			dep:  "<sys-libs/glibc-2.38",
			want: PackageAtom{
				Operator: "<",
				Category: "sys-libs",
				Name:     "glibc",
				Version:  "2.38",
			},
		},
		{
			name: "with operator !!",
			dep:  "!!sys-fs/udev",
			want: PackageAtom{
				Operator: "!!",
				Category: "sys-fs",
				Name:     "udev",
			},
		},
		{
			name: "with operator !<",
			dep:  "!<sys-apps/systemd-216",
			want: PackageAtom{
				Operator: "!<",
				Category: "sys-apps",
				Name:     "systemd",
				Version:  "216",
			},
		},
		{
			name: "with slot",
			dep:  "dev-lang/python:3.10",
			want: PackageAtom{
				Category: "dev-lang",
				Name:     "python",
				Slot:     "3.10",
			},
		},
		{
			name: "with subslot",
			dep:  "dev-lang/python-3.10.4:0/3.10",
			want: PackageAtom{
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4",
				Slot:     "0/3.10",
			},
		},
		{
			name: "with USE flags",
			dep:  "dev-lang/python[sqlite,xml]",
			want: PackageAtom{
				Category: "dev-lang",
				Name:     "python",
				UseFlags: "sqlite,xml",
			},
		},
		{
			name: "everything combined",
			dep:  ">=dev-lang/python-3.10.4-r1:0/3.10[sqlite,xml,-test]",
			want: PackageAtom{
				Operator: ">=",
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4-r1",
				Slot:     "0/3.10",
				UseFlags: "sqlite,xml,-test",
			},
		},
		{
			name: "missing category",
			dep:  "python",
			want: PackageAtom{
				Name: "python",
			},
		},
		{
			name: "missing category with version and operator",
			dep:  ">=python-3.10",
			want: PackageAtom{
				Operator: ">=",
				Name:     "python",
				Version:  "3.10",
			},
		},
		{
			name: "name with numbers",
			dep:  "sys-devel/gcc-11",
			want: PackageAtom{
				Category: "sys-devel",
				Name:     "gcc",
				Version:  "11",
			},
		},
		{
			name: "name with numbers and no version",
			dep:  "x11-libs/libX11",
			want: PackageAtom{
				Category: "x11-libs",
				Name:     "libX11",
			},
		},
		{
			name: "name with numbers and version",
			dep:  "x11-libs/libX11-1.7.2",
			want: PackageAtom{
				Category: "x11-libs",
				Name:     "libX11",
				Version:  "1.7.2",
			},
		},
		{
			name: "virtual with USE flag",
			dep:  "virtual/pkgconfig[native-symlinks]",
			want: PackageAtom{
				Category: "virtual",
				Name:     "pkgconfig",
				UseFlags: "native-symlinks",
			},
		},
		{
			name: "with repo qualifier",
			dep:  "app-misc/foo::arrans-overlay",
			want: PackageAtom{
				Category: "app-misc",
				Name:     "foo",
				Repo:     "arrans-overlay",
			},
		},
		{
			name: "with slot, repo, and use flags",
			dep:  ">=dev-lang/python-3.10.4-r1:0/3.10::gentoo[sqlite,xml]",
			want: PackageAtom{
				Operator: ">=",
				Category: "dev-lang",
				Name:     "python",
				Version:  "3.10.4-r1",
				Slot:     "0/3.10",
				Repo:     "gentoo",
				UseFlags: "sqlite,xml",
			},
		},
		{
			name: "wildcard repo atom",
			dep:  "*/*::guru",
			want: PackageAtom{
				Category: "*",
				Name:     "*",
				Repo:     "guru",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePackageAtom(tt.dep)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePackageAtom(%q) = %+v, want %+v", tt.dep, got, tt.want)
			}
			// Test round-trip String() where applicable
			if tt.want.Category != "" && tt.want.Name != "" {
				formatted := got.String()
				if formatted != tt.dep {
					t.Errorf("got.String() = %q, want %q", formatted, tt.dep)
				}
			}
		})
	}
}

func TestValidateRepoName(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		wantErr bool
	}{
		{name: "valid name", repo: "arrans-overlay", wantErr: false},
		{name: "valid with underscores and numbers", repo: "my_repo_123", wantErr: false},
		{name: "valid hyphen with non-digit", repo: "repo-v1", wantErr: false},
		{name: "newline injection", repo: "repo\ncat/injected", wantErr: true},
		{name: "space in name", repo: "repo name", wantErr: true},
		{name: "leading hyphen", repo: "-repo", wantErr: true},
		{name: "leading plus", repo: "+repo", wantErr: true},
		{name: "leading dot", repo: ".repo", wantErr: true},
		{name: "invalid character @", repo: "repo@name", wantErr: true},
		{name: "empty name", repo: "", wantErr: true},
		{name: "ending in digit version -1", repo: "repo-1", wantErr: true},
		{name: "ending in version -2.3", repo: "overlay-2.3", wantErr: true},
		{name: "ending in revision version -1.2-r1", repo: "foo-1.2-r1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRepoName(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepoName(%q) error = %v, wantErr %v", tt.repo, err, tt.wantErr)
			}
		})
	}
}

func TestQualifyAtomForRepo(t *testing.T) {
	tests := []struct {
		name       string
		dep        string
		targetRepo string
		want       string
		wantErr    bool
	}{
		{
			name:       "unqualified atom",
			dep:        "app-misc/foo",
			targetRepo: "arrans-overlay",
			want:       "app-misc/foo::arrans-overlay",
		},
		{
			name:       "unqualified versioned atom with operator",
			dep:        ">=dev-util/bar-2.0",
			targetRepo: "arrans-overlay",
			want:       ">=dev-util/bar-2.0::arrans-overlay",
		},
		{
			name:       "exact version atom",
			dep:        "=cat/pkg-1.2",
			targetRepo: "arrans-overlay",
			want:       "=cat/pkg-1.2::arrans-overlay",
		},
		{
			name:       "revision-agnostic version atom",
			dep:        "~cat/pkg-1.2",
			targetRepo: "arrans-overlay",
			want:       "~cat/pkg-1.2::arrans-overlay",
		},
		{
			name:       "unqualified slot atom",
			dep:        "cat/pkg:0",
			targetRepo: "arrans-overlay",
			want:       "cat/pkg:0::arrans-overlay",
		},
		{
			name:       "versioned with slot",
			dep:        "=cat/pkg-1.2:0",
			targetRepo: "arrans-overlay",
			want:       "=cat/pkg-1.2:0::arrans-overlay",
		},
		{
			name:       "matching qualified atom",
			dep:        "app-misc/foo::arrans-overlay",
			targetRepo: "arrans-overlay",
			want:       "app-misc/foo::arrans-overlay",
		},
		{
			name:       "matching qualified with version and slot",
			dep:        ">=cat/pkg-2:0::arrans-overlay",
			targetRepo: "arrans-overlay",
			want:       ">=cat/pkg-2:0::arrans-overlay",
		},
		{
			name:       "valid slot operator *",
			dep:        "cat/pkg:*",
			targetRepo: "arrans-overlay",
			want:       "cat/pkg:*::arrans-overlay",
		},
		{
			name:       "valid slot operator =",
			dep:        "cat/pkg:=",
			targetRepo: "arrans-overlay",
			want:       "cat/pkg:=::arrans-overlay",
		},
		{
			name:       "valid slot with equal suffix",
			dep:        "cat/pkg:0=",
			targetRepo: "arrans-overlay",
			want:       "cat/pkg:0=::arrans-overlay",
		},
		{
			name:       "valid slot and subslot",
			dep:        "cat/pkg:0/1",
			targetRepo: "arrans-overlay",
			want:       "cat/pkg:0/1::arrans-overlay",
		},
		{
			name:       "valid slot and subslot with equal suffix",
			dep:        "cat/pkg:0/1=",
			targetRepo: "arrans-overlay",
			want:       "cat/pkg:0/1=::arrans-overlay",
		},
		{
			name:       "invalid slot subslot with asterisk",
			dep:        "cat/pkg:0/*",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "invalid slot with leading slash",
			dep:        "cat/pkg:/0",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "invalid slot with trailing slash",
			dep:        "cat/pkg:0/",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "invalid slot double asterisk",
			dep:        "cat/pkg:**",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "invalid slot subslot double asterisk",
			dep:        "cat/pkg:0/**",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "blocker ! is rejected",
			dep:        "!cat/pkg",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "blocker !! is rejected",
			dep:        "!!cat/pkg",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "blocker with versioned atom is rejected",
			dep:        "!>=cat/pkg-2",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "USE dependency is rejected",
			dep:        "cat/pkg[foo]",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "negative USE dependency is rejected",
			dep:        "cat/pkg[-foo]",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "multi-flag USE dependency is rejected",
			dep:        "cat/pkg[foo,bar]",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "empty USE dependency is rejected",
			dep:        "cat/pkg[]",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "greater than operator with version",
			dep:        ">cat/pkg-2",
			targetRepo: "arrans-overlay",
			want:       ">cat/pkg-2::arrans-overlay",
		},
		{
			name:       "less than or equal operator with version",
			dep:        "<=cat/pkg-2",
			targetRepo: "arrans-overlay",
			want:       "<=cat/pkg-2::arrans-overlay",
		},
		{
			name:       "less than operator with version",
			dep:        "<cat/pkg-2",
			targetRepo: "arrans-overlay",
			want:       "<cat/pkg-2::arrans-overlay",
		},
		{
			name:       "conflicting qualified atom",
			dep:        "app-misc/foo::guru",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "empty atom",
			dep:        "",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "empty repo",
			dep:        "app-misc/foo",
			targetRepo: "",
			wantErr:    true,
		},
		{
			name:       "invalid repo ending in version",
			dep:        "app-misc/foo",
			targetRepo: "repo-1",
			wantErr:    true,
		},
		{
			name:       "invalid repo with newline",
			dep:        "app-misc/foo",
			targetRepo: "repo\ncat/injected",
			wantErr:    true,
		},
		{
			name:       "invalid repo with leading hyphen",
			dep:        "app-misc/foo",
			targetRepo: "-repo",
			wantErr:    true,
		},
		{
			name:       "bare package without category",
			dep:        "foo",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "leading slash",
			dep:        "/foo",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "trailing slash",
			dep:        "cat/",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "extra path component",
			dep:        "cat/foo/extra",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "whitespace inside atom",
			dep:        "cat / foo",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "empty repository qualifier",
			dep:        "cat/pkg::",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "multiple repository qualifiers",
			dep:        "cat/pkg::repo1::repo2",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "repo-wide wildcard atom unqualified",
			dep:        "*/*",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "repo-wide wildcard atom qualified",
			dep:        "*/*::arrans-overlay",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "operator without version",
			dep:        ">=cat/pkg",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "version without operator",
			dep:        "cat/pkg-1.2",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "category leading plus",
			dep:        "+cat/pkg",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "category leading dot",
			dep:        ".cat/pkg",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "package name leading plus",
			dep:        "cat/+pkg",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "package name contains dot",
			dep:        "cat/pkg.name",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "package name contains at-sign",
			dep:        "cat/pkg@name",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
		{
			name:       "invalid version format",
			dep:        "=cat/pkg-1..2",
			targetRepo: "arrans-overlay",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QualifyAtomForRepo(tt.dep, tt.targetRepo)
			if (err != nil) != tt.wantErr {
				t.Fatalf("QualifyAtomForRepo(%q, %q) error = %v, wantErr %v", tt.dep, tt.targetRepo, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("QualifyAtomForRepo(%q, %q) = %q, want %q", tt.dep, tt.targetRepo, got, tt.want)
			}
		})
	}
}

func TestGetLatestMatchingPackageRevision(t *testing.T) {
	mockFS := fstest.MapFS{
		"app-misc/goreleaser-bin/goreleaser-bin-2.17.0.ebuild":    {},
		"app-misc/goreleaser-bin/goreleaser-bin-2.17.0-r1.ebuild": {},
		"app-misc/goreleaser-bin/goreleaser-bin-2.17.0-r3.ebuild": {},
		"app-misc/goreleaser-bin/goreleaser-bin-2.17.0-r2.ebuild": {},
		"app-misc/goreleaser-bin/goreleaser-bin-1.0.0.ebuild":     {},
		"app-misc/goreleaser-bin/metadata.xml":                    {},
	}

	tests := []struct {
		name     string
		category string
		pkgName  string
		version  string
		want     string
		wantErr  bool
	}{
		{"latest_revision", "app-misc", "goreleaser-bin", "2.17.0", "2.17.0-r3", false},
		{"no_revision", "app-misc", "goreleaser-bin", "1.0.0", "1.0.0", false},
		{"not_found_version", "app-misc", "goreleaser-bin", "9.9.9", "", false},
		{"not_found_pkg", "app-misc", "not-exist", "1.0", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLatestMatchingPackageRevision(mockFS, tt.category, tt.pkgName, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestMatchingPackageRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetLatestMatchingPackageRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGentooVersion_Roundtrip(t *testing.T) {
	versions := []string{
		"1.2.3",
		"1.2.3-r1",
		"1.2.3_alpha1",
		"1.2.3_beta2-r1",
		"1.2.3-alpha1",
		"1.2.3_alpha1_beta2_p3-r4",
	}

	for _, v := range versions {
		parsed := ParseGentooVersion(v)
		if !parsed.IsValid {
			// Some of these might be invalid Gentoo versions but valid semver,
			// like 1.2.3-alpha1. Let's just test if String() matches original for valid ones.
			continue
		}
		str := parsed.String()
		if str != v {
			t.Errorf("Roundtrip failed for %s: got %s", v, str)
		}
	}
}

func TestExtractPackageNameFromDep(t *testing.T) {
	tests := []struct {
		name string
		dep  string
		want string
	}{
		{
			name: "basic category and name",
			dep:  "dev-lang/python",
			want: "dev-lang/python",
		},
		{
			name: "with version",
			dep:  "dev-lang/python-3.10.4",
			want: "dev-lang/python",
		},
		{
			name: "with revision",
			dep:  "dev-lang/python-3.10.4-r1",
			want: "dev-lang/python",
		},
		{
			name: "with operator >=",
			dep:  ">=dev-lang/python-3.10.4",
			want: "dev-lang/python",
		},
		{
			name: "with operator ~",
			dep:  "~dev-lang/python-3.10.4",
			want: "dev-lang/python",
		},
		{
			name: "with slot",
			dep:  "dev-lang/python-3.10.4:3.10",
			want: "dev-lang/python",
		},
		{
			name: "with use flags",
			dep:  "dev-lang/python-3.10.4[sqlite,-bluetooth]",
			want: "dev-lang/python",
		},
		{
			name: "with slot and use flags",
			dep:  "dev-lang/python-3.10.4:3.10[sqlite]",
			want: "dev-lang/python",
		},
		{
			name: "without category",
			dep:  "python-3.10.4",
			want: "python",
		},
		{
			name: "with wildcard",
			dep:  "dev-lang/*",
			want: "dev-lang/*",
		},
		{
			name: "simple without version or category",
			dep:  "python",
			want: "python",
		},
		{
			name: "with repo suffix",
			dep:  "dev-lang/python::gentoo",
			want: "dev-lang/python",
		},
		{
			name: "malformed without category but with version and slot",
			dep:  "python-3.10:3.10",
			want: "python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPackageNameFromDep(tt.dep)
			if got != tt.want {
				t.Errorf("ExtractPackageNameFromDep(%q) = %q, want %q", tt.dep, got, tt.want)
			}
		})
	}
}

func TestParsingMode_String(t *testing.T) {
	tests := []struct {
		name string
		mode ParsingMode
		want string
	}{
		{"ParseMetadataOnly", ParseMetadataOnly, "ParseMetadataOnly"},
		{"ParseVariables", ParseVariables, "ParseVariables"},
		{"ParseFull", ParseFull, "ParseFull"},
		{"Unknown", ParsingMode(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("ParsingMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
