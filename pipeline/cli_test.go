package pipeline_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/empty" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.URL.Path == "/empty_scalar" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(``))
			return
		}
		if r.URL.Path == "/replace_base" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`base`))
			return
		}
		if r.URL.Path == "/whitespace" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("   \n\t  padded string  \t\n   "))
			return
		}
		if r.URL.Path == "/json_zero" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"num": 0}`))
			return
		}
		if r.URL.Path == "/json_false" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"b": false}`))
			return
		}
		if r.URL.Path == "/json_str_zero" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"s": "0"}`))
			return
		}
		if r.URL.Path == "/json_str_false" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"s": "false"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`["a", "", "c"]`))
	}))
	defer ts.Close()

	binPath := t.TempDir() + "/g2-test-bin"
	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/g2")
	err := cmd.Run()
	assert.NoError(t, err, "failed to build g2")

	tests := []struct {
		name           string
		args           []string
		expected       string
		expectError    bool
		expectedStderr string
	}{
		{
			"exactly_one failure on zero cardinality emits no stdout and writes to stderr",
			[]string{"pipeline", `get(http://127.0.0.1:0/fail) | exactly_one`},
			"",
			true,
			"connection refused",
		},
		{
			"substitution missing fails clearly",
			[]string{"pipeline", `replace('a', '${VAR}') | trim`},
			"",
			true,
			"missing required substitution: ${VAR}",
		},
		{
			"required substitutions all succeed via CLI flags",
			[]string{
				"pipeline",
				"-s", "VERSION=1.2.3",
				"-s", "TAG=v1.2.3",
				"-s", "RELEASE_FILENAME=pkg-1.2.3.tar.gz",
				"get(" + ts.URL + "/replace_base) | replace('base', '${VERSION}-${TAG}-${RELEASE_FILENAME}')",
			},
			"1.2.3-v1.2.3-pkg-1.2.3.tar.gz\n",
			false,
			"",
		},
		{
			"positive trim via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/whitespace) | trim"},
			"padded string\n",
			false,
			"",
		},
		{
			"single zero cardinality fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/empty) | json() | single"},
			"",
			true,
			"expected 1 item, got 0",
		},
		{
			"single multiple items fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + ") | json() | single"},
			"",
			true,
			"expected 1 item, got 3",
		},
		{
			"single scalar numeric 0 preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_zero) | json(num) | single"},
			"0\n",
			false,
			"",
		},
		{
			"single scalar bool false preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_false) | json(b) | single"},
			"false\n",
			false,
			"",
		},
		{
			"single scalar string 0 preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_str_zero) | json(s) | single"},
			"0\n",
			false,
			"",
		},
		{
			"single scalar string false preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_str_false) | json(s) | single"},
			"false\n",
			false,
			"",
		},
		{
			"single empty scalar string fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/empty_scalar) | single"},
			"",
			true,
			"expected 1 item, got 0",
		},
		{
			"replace non-string argument fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/replace_base) | replace('base', 1)"},
			"",
			true,
			"arguments must be strings",
		},
		{
			"replace trailing characters after quote fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/replace_base) | replace('base'junk, 'new')"},
			"",
			true,
			"trailing characters after quote",
		},
		{
			"list printing empty strings as empty lines",
			[]string{"pipeline", "get(" + ts.URL + ") | json()"},
			"a\n\nc\n",
			false,
			"",
		},
		{
			"zero cardinality list emits no output",
			[]string{"pipeline", "get(" + ts.URL + "/empty) | json()"},
			"",
			false,
			"",
		},
		{
			"empty scalar string without cardinality check produces no output",
			[]string{"pipeline", "get(" + ts.URL + "/empty_scalar) | replace('a', '')"},
			"",
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := exec.Command(binPath, tt.args...)
			var out bytes.Buffer
			var stderr bytes.Buffer
			c.Stdout = &out
			c.Stderr = &stderr
			err := c.Run()

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedStderr != "" {
					assert.Contains(t, stderr.String(), tt.expectedStderr)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, out.String()) // Don't use TrimSpace to strictly verify trailing newlines
			}
		})
	}
}
