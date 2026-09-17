package pipeline_test

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/g2"
	"github.com/arran4/g2/pipeline"
	"github.com/stretchr/testify/assert"
)

func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/legit_false_scalar":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`0`))
		case "/legit_false_string":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`false`))
		case "/json_bool":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"k": false}`))
		case "/json_int":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"k": 0}`))
		case "/empty":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(``))
		case "/rss":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<rss><channel><item><link>http://example.com/v1.0.tar.gz</link></item></channel></rss>`))
		case "/atom":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"><entry><link href="http://example.com/v2.0.tar.gz"/></entry></feed>`))
		case "/json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"releases": [{"tag": "v2.0"}]}`))
		case "/html":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><body><a href="relative.tar.gz">link</a><a href="other.tar.gz">link2</a></body></html>`))
		case "/xml":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<root><link>http://example.com/v3.0.tar.gz</link></root>`))
		case "/malformed_json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"releases": [{"tag": "v2.0"`))
		case "/malformed_xml":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<root><link>http://example.com/v3.0.tar.gz</root>`))
		case "/malformed_rss":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<rss><channel><item><link>http://example.com</item></channel></rss>`))
		case "/malformed_atom":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"><entry><link href="broken></feed>`))
		case "/whitespace":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("  \t\r\n  extracted content  \t\r\n  "))
		case "/whitespaced_list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"items": ["  apple  ", "  banana  "]}`))
		case "/which_browser/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><body><a href="downloads/v0.2.6/which_browser-0.2.6+44-linux.deb">download</a></body></html>`))
		case "/which_browser/downloads/v0.2.6/which_browser-0.2.6+44-linux.deb":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("binary-deb-content"))
		case "/404":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestPythonPipeline(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	tests := []struct {
		name        string
		pipeline    string
		expected    string
		fail        bool
		expectedErr string
	}{
		{"RSS", "get(" + ts.URL + "/rss) | rss | first | link | url.basename | regex(v(.*)\\.tar\\.gz)", "1.0", false, ""},
		{"Atom", "get(" + ts.URL + "/atom) | atom | first | link | url.basename | regex(v(.*)\\.tar\\.gz)", "2.0", false, ""},
		{"JSON", "get(" + ts.URL + "/json) | json(releases.0.tag) | replace('v', '')", "2.0", false, ""},
		{"HTML Links", "get(" + ts.URL + "/html) | html_links | first", ts.URL + "/relative.tar.gz", false, ""},
		{"XML XPath", "get(" + ts.URL + "/xml) | xml | xpath(.//link) | first | regex(v(.*)\\.tar\\.gz)", "3.0", false, ""},
		{"Replace comma", "get(" + ts.URL + "/rss) | replace('tar.gz', 'zip')", "<rss><channel><item><link>http://example.com/v1.0.zip</link></item></channel></rss>", false, ""},
		{"Replace spaces after comma", "get(" + ts.URL + "/rss) | replace('tar.gz',  'zip')", "<rss><channel><item><link>http://example.com/v1.0.zip</link></item></channel></rss>", false, ""},
		{"Replace spaces around args", "get(" + ts.URL + "/rss) | replace(  'tar.gz'  ,  'zip'  )", "<rss><channel><item><link>http://example.com/v1.0.zip</link></item></channel></rss>", false, ""},
		{"Replace double quotes", "get(" + ts.URL + "/rss) | replace(\"tar.gz\", \"zip\")", "<rss><channel><item><link>http://example.com/v1.0.zip</link></item></channel></rss>", false, ""},
		{"Replace commas inside args", "get(" + ts.URL + "/json) | replace('v2.0', 'v3.0,v') | json(releases.0.tag)", "v3.0,v", false, ""},
		{"Replace single and double quotes", "get(" + ts.URL + "/json) | replace('\"tag\": \"v2.0\"', '\"tag\": \"v3.0\"') | json(releases.0.tag)", "v3.0", false, ""},
		{"Replace escaped quotes", "get(" + ts.URL + "/json) | json(releases.0.tag) | replace('v2.0', 'v\\'3.0')", "v'3.0", false, ""},
		{"Replace backslashes", "get(" + ts.URL + "/json) | json(releases.0.tag) | replace('v2', 'v\\\\2')", "v\\2.0", false, ""},
		{"Replace empty string", "get(" + ts.URL + "/json) | replace('v', '') | json(releases.0.tag)", "2.0", false, ""},
		{"Replace empty strings both args", "get(" + ts.URL + "/json) | replace('', '') | json(releases.0.tag)", "v2.0", false, ""},
		{"Fail Replace less args", "get(" + ts.URL + "/rss) | replace('a')", "", true, "exactly 2 arguments"},
		{"Fail Replace more args", "get(" + ts.URL + "/rss) | replace('a', 'b', 'c')", "", true, "exactly 2 arguments"},
		{"Fail Replace unterminated quotes", "get(" + ts.URL + "/rss) | replace('a', 'b)", "", true, "unterminated quote"},
		{"Fail Replace trailing junk after quoted first arg", "get(" + ts.URL + "/rss) | replace('a'junk, 'b')", "", true, "trailing characters after quote"},
		{"Fail Replace leading junk before quoted first arg", "get(" + ts.URL + "/rss) | replace(junk'a', 'b')", "", true, "arguments must be strings"},
		{"Fail Replace trailing junk after quoted second arg", "get(" + ts.URL + "/rss) | replace('a', 'b'junk)", "", true, "trailing characters after quote"},
		{"Fail Replace leading junk before quoted second arg", "get(" + ts.URL + "/rss) | replace('a', junk'b')", "", true, "arguments must be strings"},
		{"Fail Replace unquoted number second arg", "get(" + ts.URL + "/rss) | replace('a', 1)", "", true, "arguments must be strings"},
		{"Fail Replace unquoted number first arg", "get(" + ts.URL + "/rss) | replace(1, 'b')", "", true, "arguments must be strings"},
		{"Fail Replace unquoted identifier second arg", "get(" + ts.URL + "/rss) | replace('a', foo)", "", true, "arguments must be strings"},
		{"Fail Replace unquoted mix with space", "get(" + ts.URL + "/rss) | replace('a' junk, 'b')", "", true, "trailing characters after quote"},
		{"Fail Replace unquoted mix before quote", "get(" + ts.URL + "/rss) | replace(junk 'a', 'b')", "", true, "arguments must be strings"},

		{"Positive trim leading trailing whitespace", "get(" + ts.URL + "/whitespace) | trim", "extracted content", false, ""},
		{"Positive trim with further processing", "get(" + ts.URL + "/whitespace) | trim | replace('content', 'value')", "extracted value", false, ""},

		{"Fail Malformed JSON", "get(" + ts.URL + "/malformed_json) | json(releases.0.tag)", "", true, "error parsing JSON path releases.0.tag"},
		{"Fail Malformed XML", "get(" + ts.URL + "/malformed_xml) | xml", "", true, "error parsing XML"},
		{"Fail Malformed RSS", "get(" + ts.URL + "/malformed_rss) | rss", "", true, "error parsing rss"},
		{"Fail Malformed Atom", "get(" + ts.URL + "/malformed_atom) | atom", "", true, "error parsing atom"},
		{"Fail Invalid XPath", "get(" + ts.URL + "/xml) | xml | xpath(//[invalid)", "", true, "error executing xpath"},
		{"Fail XPath on non-XML", "get(" + ts.URL + "/json) | xpath(//link)", "", true, "error executing xpath: input must be parsed xml"},
		{"Fail Invalid Regex", "get(" + ts.URL + "/json) | regex([a-)", "", true, "invalid regex"},

		{"Regex alternation", "get(" + ts.URL + "/html) | html_links | regex(((?:relative|other)\\.tar\\.gz)) | last", "other.tar.gz", false, ""},
		{"Regex escaped paren", "get(" + ts.URL + "/json) | replace('v2.0', '(v2.0)') | regex(\\((.*?)\\))", "v2.0", false, ""},
		{"Replace spaces and quotes", "get(" + ts.URL + "/json) | replace('\"tag\": \"v2.0\"', '\"tag\": \"v3.0\"') | json(releases.0.tag)", "v3.0", false, ""},
		{"Fail 404", "get(" + ts.URL + "/404) | trim", "", true, "status 404"},
		{"Fail Unknown", "unknown_cmd", "", true, "unknown command"},
		{"Fail Empty", "get(" + ts.URL + "/rss) | ", "", true, "empty pipeline stage"},
		{"Fail Unbalanced", "get(" + ts.URL + "/rss | trim", "", true, "unbalanced parentheses"},
		{"Fail Dangling Escape", "get(" + ts.URL + "/rss) | regex(v\\", "", true, "dangling escape in pipeline"},
		{"Exactly one success string", "get(" + ts.URL + "/rss) | replace('tar.gz', 'zip') | exactly_one", "<rss><channel><item><link>http://example.com/v1.0.zip</link></item></channel></rss>", false, ""},
		{"Exactly one success list", "get(" + ts.URL + "/rss) | rss | first | link | url.basename | exactly_one", "v1.0.tar.gz", false, ""},
		{"Exactly one fail 0 items list", "get(" + ts.URL + "/json) | json(releases) | regex(notfound) | exactly_one", "", true, "exactly_one/single expected 1 item, got 0"},
		{"Exactly one fail 0 items string", "get(" + ts.URL + "/json) | json(releases) | regex(notfound) | first | exactly_one", "", true, "exactly_one/single expected 1 item, got 0"},
		{"Exactly one fail 2 items list", "get(" + ts.URL + "/html) | html_links | exactly_one", "", true, "exactly_one/single expected 1 item, got 2"},
		{"Single success list", "get(" + ts.URL + "/rss) | rss | first | link | url.basename | single", "v1.0.tar.gz", false, ""},
		{"Single fail 0 items list", "get(" + ts.URL + "/json) | json(releases) | regex(notfound) | single", "", true, "exactly_one/single expected 1 item, got 0"},
		{"Single fail 0 items string", "get(" + ts.URL + "/json) | json(releases) | regex(notfound) | first | single", "", true, "exactly_one/single expected 1 item, got 0"},
		{"Single fail 2 items list", "get(" + ts.URL + "/html) | html_links | single", "", true, "exactly_one/single expected 1 item, got 2"},
		{"Single fail scalar empty string", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '') | single", "", true, "exactly_one/single expected 1 item, got 0"},
		{"Single success scalar numeric 0", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '0') | single", "0", false, ""},
		{"Single success scalar string 0", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '\"0\"') | json() | single", "0", false, ""},
		{"Single success scalar string false", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '\"false\"') | json() | single", "false", false, ""},
		{"Single success json int 0", "get(" + ts.URL + "/json_int) | json(k) | single", "0", false, ""},
		{"Single success json bool false", "get(" + ts.URL + "/json_bool) | json(k) | single", "false", false, ""},
		{"Exactly one legitimately false scalar string 0", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '0') | exactly_one", "0", false, ""},
		{"Exactly one legitimately false string false", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', 'false') | exactly_one", "false", false, ""},
		{"Exactly one legitimately false json bool", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '{\"k\": false}') | json(k) | exactly_one", "false", false, ""},
		{"Exactly one legitimately false json int", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '{\"k\": 0}') | json(k) | exactly_one", "0", false, ""},
		{"Exactly one empty string", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '') | exactly_one", "", true, "exactly_one/single expected 1 item, got 0"},
		{"Exactly one success scalar string 0", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '\"0\"') | json() | exactly_one", "0", false, ""},
		{"Exactly one success scalar string false", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '\"false\"') | json() | exactly_one", "false", false, ""},

		{"which_browser regression", "get(" + ts.URL + "/which_browser/) | html_links | regex(which_browser-([^/]+)-linux[.]deb$) | exactly_one", "0.2.6+44", false, ""},
	}

	evaluator := pipeline.NewEvaluator(ts.Client())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := evaluator.Evaluate(tt.pipeline)

			if tt.fail {
				assert.Error(t, err)
				if tt.expectedErr != "" && err != nil {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
				return
			}

			assert.NoError(t, err)
			var outStr string
			if val != nil {
				if val.IsList() {
					var lst []string
					for _, item := range val.List {
						lst = append(lst, fmt.Sprintf("%v", item))
					}
					outStr = strings.Join(lst, "\n")
				} else {
					outStr = val.GetString()
				}
			}
			assert.Equal(t, tt.expected, outStr)
		})
	}
}

func TestSubstitutions(t *testing.T) {
	tests := []struct {
		name        string
		pipeline    string
		subs        map[string]string
		expected    string
		expectError bool
		errContains string
	}{
		{
			"simple subst",
			"get(${URL}) | regex(${PATTERN})",
			map[string]string{"URL": "http://foo", "PATTERN": "foo"},
			"get(http://foo) | regex(foo)",
			false,
			"",
		},
		{
			"required substitutions all present",
			"get(http://example.com/downloads/${TAG}/${RELEASE_FILENAME}) | regex(v${VERSION})",
			map[string]string{
				"VERSION":          "1.2.3",
				"TAG":              "v1.2.3",
				"RELEASE_FILENAME": "app-1.2.3.tar.gz",
			},
			"get(http://example.com/downloads/v1.2.3/app-1.2.3.tar.gz) | regex(v1.2.3)",
			false,
			"",
		},
		{
			"missing VERSION variable",
			"get(http://example.com/${VERSION})",
			map[string]string{"TAG": "v1.2.3", "RELEASE_FILENAME": "app.tar.gz"},
			"",
			true,
			"missing required substitution: ${VERSION}",
		},
		{
			"missing TAG variable",
			"get(http://example.com/${TAG})",
			map[string]string{"VERSION": "1.2.3", "RELEASE_FILENAME": "app.tar.gz"},
			"",
			true,
			"missing required substitution: ${TAG}",
		},
		{
			"missing RELEASE_FILENAME variable",
			"get(http://example.com/${RELEASE_FILENAME})",
			map[string]string{"VERSION": "1.2.3", "TAG": "v1.2.3"},
			"",
			true,
			"missing required substitution: ${RELEASE_FILENAME}",
		},
		{
			"missing var",
			"get(${URL})",
			map[string]string{},
			"",
			true,
			"missing required substitution: ${URL}",
		},
		{
			"unterminated",
			"get(${URL)",
			map[string]string{"URL": "foo"},
			"",
			true,
			"unterminated substitution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.PerformSubstitutions(tt.pipeline, tt.subs)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errContains != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

type errorRoundTripper struct{}

func (e *errorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("simulated network connection reset by peer")
}

func TestTransportFailure(t *testing.T) {
	client := &http.Client{Transport: &errorRoundTripper{}}
	evaluator := pipeline.NewEvaluator(client)
	val, err := evaluator.Evaluate("get(http://test.local/stream)")
	assert.Error(t, err)
	assert.Nil(t, val)
	assert.Contains(t, err.Error(), "error fetching URL http://test.local/stream")
	assert.Contains(t, err.Error(), "simulated network connection reset by peer")
}

func TestWhichBrowser_RegressionHandoff(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	evaluator := pipeline.NewEvaluator(ts.Client())

	// 1. Raw discovered artifact/version from upstream HTML listing
	rawVerVal, err := evaluator.Evaluate("get(" + ts.URL + "/which_browser/) | html_links | regex(which_browser-([^/]+)-linux[.]deb$) | exactly_one")
	assert.NoError(t, err)
	assert.False(t, rawVerVal.IsEmpty)
	rawVersion := rawVerVal.GetString()
	assert.Equal(t, "0.2.6+44", rawVersion)

	// 2. Discovered full URL
	urlVal, err := evaluator.Evaluate("get(" + ts.URL + "/which_browser/) | html_links | regex((.*which_browser-.*-linux[.]deb$)) | exactly_one")
	assert.NoError(t, err)
	discoveredURL := urlVal.GetString()
	assert.Equal(t, ts.URL+"/which_browser/downloads/v0.2.6/which_browser-0.2.6+44-linux.deb", discoveredURL)

	// 3. Artifact filename extracted using exactly_one before url.basename (since url.basename is scalar-only)
	artifactVal, err := evaluator.Evaluate("get(" + ts.URL + "/which_browser/) | html_links | regex((.*which_browser-.*-linux[.]deb$)) | exactly_one | url.basename")
	assert.NoError(t, err)
	artifactFilename := artifactVal.GetString()
	assert.Equal(t, "which_browser-0.2.6+44-linux.deb", artifactFilename)

	// 4. Normalized Gentoo version using replace('+', '_p') after exactly_one (downstream oracle s/\+/_p/g)
	gentooVerVal, err := evaluator.Evaluate("get(" + ts.URL + "/which_browser/) | html_links | regex(which_browser-([^/]+)-linux[.]deb$) | exactly_one | replace('+', '_p')")
	assert.NoError(t, err)
	gentooVersion := gentooVerVal.GetString()
	assert.Equal(t, "0.2.6_p44", gentooVersion)

	// 5. Resulting ebuild parsing and variable expansion handoff:
	// Verify that normalized Gentoo version and discovered artifact filename integrate cleanly
	// with the downstream Gentoo ebuild model used by workflow-builder (www-client/which-browser-bin/which-browser-bin-0.2.6_p44.ebuild).
	ebuildName := fmt.Sprintf("which-browser-bin-%s.ebuild", gentooVersion)
	ebuildContent := fmt.Sprintf(`EAPI=8
DESCRIPTION="Web browser binary package"
HOMEPAGE="https://example.com"
SRC_URI="%s -> ${P}-%s"
`, discoveredURL, artifactFilename)

	mockFS := fstest.MapFS{
		ebuildName: &fstest.MapFile{
			Data: []byte(ebuildContent),
		},
	}

	parsed, err := g2.ParseEbuild(mockFS, ebuildName, g2.ParseFull)
	assert.NoError(t, err)
	assert.Equal(t, "which-browser-bin", parsed.Vars["PN"])
	assert.Equal(t, "0.2.6_p44", parsed.Vars["PV"])
	assert.Equal(t, "which-browser-bin-0.2.6_p44", parsed.Vars["P"])
	assert.NotEmpty(t, parsed.SrcUri)
	expectedDistfileName := fmt.Sprintf("which-browser-bin-%s-%s", gentooVersion, artifactFilename)
	assert.Equal(t, expectedDistfileName, parsed.SrcUri[0].Filename)
	assert.Equal(t, discoveredURL, parsed.SrcUri[0].URL)

	// 6. Generic download verification using pipeline variable substitutions
	downloadPipeline, err := pipeline.PerformSubstitutions(
		"get(${SRC_URL}) | trim",
		map[string]string{"SRC_URL": discoveredURL},
	)
	assert.NoError(t, err)
	debVal, err := evaluator.Evaluate(downloadPipeline)
	assert.NoError(t, err)
	assert.Equal(t, "binary-deb-content", debVal.GetString())

	// 7. Real manifest handoff using g2.UpsertManifest and round-trip verification
	manifestDir := t.TempDir()
	manifestPath := filepath.Join(manifestDir, "Manifest")

	debBytes := []byte(debVal.GetString())
	hash512 := sha512.Sum512(debBytes)
	entry := g2.NewManifestEntry(
		"DIST",
		parsed.SrcUri[0].Filename,
		int64(len(debBytes)),
		g2.Hash{Type: "SHA512", Value: hex.EncodeToString(hash512[:])},
	)

	err = g2.UpsertManifest(manifestPath, entry)
	assert.NoError(t, err)

	manifestData, err := os.ReadFile(manifestPath)
	assert.NoError(t, err)
	expectedLine := fmt.Sprintf("DIST %s %d SHA512 %s\n", parsed.SrcUri[0].Filename, len(debBytes), hex.EncodeToString(hash512[:]))
	assert.Equal(t, expectedLine, string(manifestData))

	parsedManifest, err := g2.ParseManifest(manifestPath)
	assert.NoError(t, err)
	assert.Len(t, parsedManifest.Entries, 1)
	assert.Equal(t, parsed.SrcUri[0].Filename, parsedManifest.Entries[0].Filename)
	assert.Equal(t, int64(len(debBytes)), parsedManifest.Entries[0].Size)
	assert.Equal(t, "SHA512", parsedManifest.Entries[0].Hashes[0].Type)
	assert.Equal(t, hex.EncodeToString(hash512[:]), parsedManifest.Entries[0].Hashes[0].Value)
}

func TestScalarOnlyOperators(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	evaluator := pipeline.NewEvaluator(ts.Client())

	t.Run("trim leaves list unchanged", func(t *testing.T) {
		val, err := evaluator.Evaluate("get(" + ts.URL + "/whitespaced_list) | json(items) | trim")
		assert.NoError(t, err)
		assert.True(t, val.IsList())
		assert.Equal(t, []interface{}{"  apple  ", "  banana  "}, val.List)

		// But when reduced to a scalar, trim strips whitespace
		scalarVal, err := evaluator.Evaluate("get(" + ts.URL + "/whitespaced_list) | json(items) | first | trim")
		assert.NoError(t, err)
		assert.False(t, scalarVal.IsList())
		assert.Equal(t, "apple", scalarVal.GetString())
	})

	t.Run("replace leaves list unchanged", func(t *testing.T) {
		val, err := evaluator.Evaluate("get(" + ts.URL + "/whitespaced_list) | json(items) | replace('apple', 'pear')")
		assert.NoError(t, err)
		assert.True(t, val.IsList())
		assert.Equal(t, []interface{}{"  apple  ", "  banana  "}, val.List)

		// But when reduced to a scalar, replace replaces
		scalarVal, err := evaluator.Evaluate("get(" + ts.URL + "/whitespaced_list) | json(items) | first | replace('apple', 'pear')")
		assert.NoError(t, err)
		assert.False(t, scalarVal.IsList())
		assert.Equal(t, "  pear  ", scalarVal.GetString())
	})

	t.Run("url.basename leaves list unchanged", func(t *testing.T) {
		val, err := evaluator.Evaluate("get(" + ts.URL + "/html) | html_links | url.basename")
		assert.NoError(t, err)
		assert.True(t, val.IsList())
		assert.Equal(t, 2, len(val.List))
		assert.Equal(t, ts.URL+"/relative.tar.gz", val.List[0])
		assert.Equal(t, ts.URL+"/other.tar.gz", val.List[1])

		// But when reduced to a scalar, url.basename extracts basename
		scalarVal, err := evaluator.Evaluate("get(" + ts.URL + "/html) | html_links | first | url.basename")
		assert.NoError(t, err)
		assert.False(t, scalarVal.IsList())
		assert.Equal(t, "relative.tar.gz", scalarVal.GetString())
	})
}

func TestCardinality(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	tests := []struct {
		name        string
		pipeline    string
		expectEmpty bool
		expectValue interface{}
		expectList  bool
	}{
		{"json bool", "get(" + ts.URL + "/json_bool) | json(k)", false, false, false},
		{"json num", "get(" + ts.URL + "/json_int) | json(k)", false, float64(0), false},
		{"empty string", "get(" + ts.URL + "/empty)", false, "", false},
		{"empty scalar after regex match nothing", "get(" + ts.URL + "/legit_false_scalar) | regex(z)", false, "", false},
	}

	evaluator := pipeline.NewEvaluator(ts.Client())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := evaluator.Evaluate(tt.pipeline)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectEmpty, val.IsEmpty)
			assert.Equal(t, tt.expectList, val.IsList())
			if !tt.expectList {
				assert.Equal(t, tt.expectValue, val.Value)
			}
		})
	}
}
