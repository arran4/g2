package pipeline_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`["a", "", "c"]`))
	}))
	defer ts.Close()

	cmd := exec.Command("go", "build", "-o", "g2-test-bin", "../cmd/g2")
	err := cmd.Run()
	assert.NoError(t, err, "failed to build g2")
	defer exec.Command("rm", "g2-test-bin").Run()

	tests := []struct {
		name        string
		args        []string
		expected    string
		expectError bool
	}{
		{
			"simple list stdout",
			[]string{"pipeline", `get(http://localhost:0/fail) | exactly_one`},
			"",
			true,
		},
		{
			"substitution missing",
			[]string{"pipeline", `replace('a', '${VAR}') | trim`},
			"",
			true,
		},
		{
			"list printing empty strings as empty lines",
			[]string{"pipeline", "get(" + ts.URL + ") | json()"},
			"a\n\nc", // TrimSpace removes the final newline in the test assertion
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := exec.Command("./g2-test-bin", tt.args...)
			var out bytes.Buffer
			var stderr bytes.Buffer
			c.Stdout = &out
			c.Stderr = &stderr
			err := c.Run()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, strings.TrimSpace(out.String()))
			}
		})
	}
}
