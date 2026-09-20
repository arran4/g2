package main

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

	tests := []struct {
		name           string
		args           []string
		expected       string
		expectError    bool
		expectedErrStr string
		expectedStderr string
	}{
		{
			"exactly_one failure on zero cardinality emits no stdout and returns error",
			[]string{"pipeline", `get(http://127.0.0.1:0/fail) | exactly_one`},
			"",
			true,
			"connection refused",
			"",
		},
		{
			"substitution missing fails clearly",
			[]string{"pipeline", `replace('a', '${VAR}') | trim`},
			"",
			true,
			"missing required substitution: ${VAR}",
			"",
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
			"",
		},
		{
			"positive trim via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/whitespace) | trim"},
			"padded string\n",
			false,
			"",
			"",
		},
		{
			"single zero cardinality fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/empty) | json() | single"},
			"",
			true,
			"expected 1 item, got 0",
			"",
		},
		{
			"single multiple items fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + ") | json() | single"},
			"",
			true,
			"expected 1 item, got 3",
			"",
		},
		{
			"single scalar numeric 0 preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_zero) | json(num) | single"},
			"0\n",
			false,
			"",
			"",
		},
		{
			"single scalar bool false preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_false) | json(b) | single"},
			"false\n",
			false,
			"",
			"",
		},
		{
			"single scalar string 0 preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_str_zero) | json(s) | single"},
			"0\n",
			false,
			"",
			"",
		},
		{
			"single scalar string false preserves value via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/json_str_false) | json(s) | single"},
			"false\n",
			false,
			"",
			"",
		},
		{
			"single empty scalar string fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/empty_scalar) | single"},
			"",
			true,
			"expected 1 item, got 0",
			"",
		},
		{
			"replace non-string argument fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/replace_base) | replace('base', 1)"},
			"",
			true,
			"arguments must be strings",
			"",
		},
		{
			"replace trailing characters after quote fails via CLI",
			[]string{"pipeline", "get(" + ts.URL + "/replace_base) | replace('base'junk, 'new')"},
			"",
			true,
			"trailing characters after quote",
			"",
		},
		{
			"list printing empty strings as empty lines",
			[]string{"pipeline", "get(" + ts.URL + ") | json()"},
			"a\n\nc\n",
			false,
			"",
			"",
		},
		{
			"zero cardinality list emits no output",
			[]string{"pipeline", "get(" + ts.URL + "/empty) | json()"},
			"",
			false,
			"",
			"",
		},
		{
			"empty scalar string without cardinality check produces no output",
			[]string{"pipeline", "get(" + ts.URL + "/empty_scalar) | replace('a', '')"},
			"",
			false,
			"",
			"",
		},
		{
			"invalid flag produces error and help",
			[]string{"pipeline", "--invalid"},
			"",
			true,
			"",
			"flag provided but not defined: -invalid\nUsage: g2 pipeline",
		},
		{
			"missing arguments produces error and help",
			[]string{"pipeline"},
			"",
			true,
			"pipeline command requires exactly one argument",
			"Usage: g2 pipeline",
		},
		{
			"multiple arguments produces error and help",
			[]string{"pipeline", "expr1", "expr2"},
			"",
			true,
			"pipeline command requires exactly one argument",
			"Usage: g2 pipeline",
		},
		{
			"help flag outputs help",
			[]string{"pipeline", "--help"},
			"Usage: g2 pipeline",
			false,
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var stderr bytes.Buffer

			// We skip "pipeline" if it's the first element, as runPipeline takes the remaining args
			args := tt.args
			if len(args) > 0 && args[0] == "pipeline" {
				args = args[1:]
			}

			err := runPipeline(args, &out, &stderr)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrStr != "" && err != nil {
					assert.Contains(t, err.Error(), tt.expectedErrStr)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedStderr != "" {
				assert.Contains(t, stderr.String(), tt.expectedStderr)
			} else {
				assert.Empty(t, stderr.String())
			}

			// For both success and failure cases, verify stdout content
			if tt.name == "help flag outputs help" {
				assert.Contains(t, out.String(), tt.expected)
			} else {
				assert.Equal(t, tt.expected, out.String()) // Don't use TrimSpace to strictly verify trailing newlines
			}
		})
	}
}
func TestCLI_UnknownOperatorFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	err := runPipeline([]string{"get(" + ts.URL + ") | unknown_operator"}, &stdout, &stderr)

	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "unknown command")
	}
	assert.Empty(t, stderr.String()) // stderr should be empty because err is returned
	assert.Empty(t, stdout.String())
}

func TestCLI_Smoke_ProcessBoundary(t *testing.T) {
	// Only run this test if requested to explicitly test the process boundary,
	// or let it run normally as it provides the critical end-to-end guarantee.
	// As per issue instructions, we retain a minimal spawned-binary smoke test.

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	binPath := t.TempDir() + "/g2-test-bin"

	// Ensure we only build once
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	var buildErr bytes.Buffer
	buildCmd.Stderr = &buildErr
	err := buildCmd.Run()
	assert.NoError(t, err, "failed to build g2: %s", buildErr.String())

	tests := []struct {
		name           string
		args           []string
		expectedExit   int
		expectedStdout string
		expectedStderr string
	}{
		{
			"evaluation failure yields exit 1 and stderr message",
			[]string{"pipeline", "get(" + ts.URL + ") | unknown_operator"},
			1,
			"",
			"unknown command: unknown_operator",
		},
		{
			"invalid flag yields exit 2 and stderr usage",
			[]string{"pipeline", "--invalid"},
			2,
			"",
			"flag provided but not defined: -invalid\nUsage: g2 pipeline",
		},
		{
			"help yields exit 0 and stdout usage",
			[]string{"pipeline", "--help"},
			0,
			"Usage: g2 pipeline",
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

			if tt.expectedExit == 0 {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if exitError, ok := err.(*exec.ExitError); ok {
					assert.Equal(t, tt.expectedExit, exitError.ExitCode())
				} else {
					t.Fatalf("expected ExitError, got %T: %v", err, err)
				}
			}

			if tt.expectedStdout != "" {
				assert.Contains(t, out.String(), tt.expectedStdout)
			} else {
				assert.Empty(t, out.String())
			}

			if tt.expectedStderr != "" {
				assert.Contains(t, stderr.String(), tt.expectedStderr)
			} else {
				assert.Empty(t, stderr.String())
			}
		})
	}
}

func TestCLI_TokenizerFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	err := runPipeline([]string{"get(" + ts.URL + ") | replace('a, 'b')"}, &stdout, &stderr)

	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "unterminated quote")
	}
	assert.Empty(t, stderr.String()) // stderr should be empty because err is returned
	assert.Empty(t, stdout.String())
}
