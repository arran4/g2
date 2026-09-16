package pipeline_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		case "/which_browser/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><body><a href="downloads/v0.2.6/which_browser-0.2.6+44-linux.deb">download</a></body></html>`))
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
		{"Replace commas inside args", "get(" + ts.URL + "/json) | replace('v2.0', 'v3.0,v') | json(releases.0.tag)", "v3.0,v", false, ""},
		{"Replace single and double quotes", "get(" + ts.URL + "/json) | replace('\"tag\": \"v2.0\"', '\"tag\": \"v3.0\"') | json(releases.0.tag)", "v3.0", false, ""},
		{"Replace escaped quotes", "get(" + ts.URL + "/json) | json(releases.0.tag) | replace('v2.0', 'v\\'3.0')", "v'3.0", false, ""},
		{"Replace backslashes", "get(" + ts.URL + "/json) | json(releases.0.tag) | replace('v2', 'v\\\\2')", "v\\2.0", false, ""},
		{"Replace empty string", "get(" + ts.URL + "/json) | replace('v', '') | json(releases.0.tag)", "2.0", false, ""},
		{"Fail Replace less args", "get(" + ts.URL + "/rss) | replace('a')", "", true, "exactly 2 arguments"},
		{"Fail Replace more args", "get(" + ts.URL + "/rss) | replace('a', 'b', 'c')", "", true, "exactly 2 arguments"},
		{"Fail Replace unterminated quotes", "get(" + ts.URL + "/rss) | replace('a', 'b)", "", true, "unterminated quote"},

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
		{"Single fail 2 items list", "get(" + ts.URL + "/html) | html_links | single", "", true, "exactly_one/single expected 1 item, got 2"},
		{"Exactly one legitimately false scalar string 0", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '0') | exactly_one", "0", false, ""},
		{"Exactly one legitimately false string false", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', 'false') | exactly_one", "false", false, ""},
		{"Exactly one legitimately false json bool", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '{\"k\": false}') | json(k) | exactly_one", "false", false, ""},
		{"Exactly one legitimately false json int", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '{\"k\": 0}') | json(k) | exactly_one", "0", false, ""},
		{"Exactly one empty string", "get(" + ts.URL + "/json) | replace('{\"releases\": [{\"tag\": \"v2.0\"}]}', '') | exactly_one", "", true, "exactly_one/single expected 1 item, got 0"},

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
	}{
		{
			"simple subst",
			"get(${URL}) | regex(${PATTERN})",
			map[string]string{"URL": "http://foo", "PATTERN": "foo"},
			"get(http://foo) | regex(foo)",
			false,
		},
		{
			"missing var",
			"get(${URL})",
			map[string]string{},
			"",
			true,
		},
		{
			"unterminated",
			"get(${URL)",
			map[string]string{"URL": "foo"},
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.PerformSubstitutions(tt.pipeline, tt.subs)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
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
