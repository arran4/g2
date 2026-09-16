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
			[]string{"pipeline", "get(" + ts.URL + "/empty_scalar) | replace('a', '')"}, // string replacing 'a' on empty gives empty
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
