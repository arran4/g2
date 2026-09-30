package g2

import (
	"errors"
	"strings"
	"testing"
)

type MockCmdRunner struct {
	stdout []byte
	stderr []byte
	err    error
	calls  []string
}

func (m *MockCmdRunner) Run(cmdName string, args ...string) ([]byte, []byte, error) {
	m.calls = append(m.calls, cmdName+" "+strings.Join(args, " "))
	return m.stdout, m.stderr, m.err
}

func TestOSExecPortageContext_CommandBuilding(t *testing.T) {
	mock := &MockCmdRunner{
		stdout: []byte("a\nb\nc"),
	}
	ctx := &OSExecPortageContext{Runner: mock}

	res, err := ctx.QueryMetadata("my-repo", "sys-apps", "test", "1.0", []string{"K1", "K2", "K3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}

	expectedCall := "portageq metadata / ebuild sys-apps/test-1.0::my-repo K1 K2 K3"
	if mock.calls[0] != expectedCall {
		t.Fatalf("expected call %q, got %q", expectedCall, mock.calls[0])
	}

	if res["K1"] != "a" || res["K2"] != "b" || res["K3"] != "c" {
		t.Fatalf("unexpected result map: %v", res)
	}
}

func TestOSExecPortageContext_Unavailable(t *testing.T) {
	mock := &MockCmdRunner{
		err: ErrPortageUnavailable,
	}
	ctx := &OSExecPortageContext{Runner: mock}

	_, err := ctx.QueryMetadata("", "sys-apps", "test", "1.0", []string{"K1"})
	if !errors.Is(err, ErrPortageUnavailable) {
		t.Fatalf("expected ErrPortageUnavailable, got %v", err)
	}
}

func TestOSExecPortageContext_GenericError(t *testing.T) {
	mock := &MockCmdRunner{
		err:    errors.New("some execution error"),
		stderr: []byte("stderr output"),
	}
	ctx := &OSExecPortageContext{Runner: mock}

	_, err := ctx.QueryMetadata("", "sys-apps", "test", "1.0", []string{"K1"})
	if err == nil || !strings.Contains(err.Error(), "stderr output") {
		t.Fatalf("expected error containing stderr, got: %v", err)
	}
}

func TestOSExecPortageContext_MalformedOutput(t *testing.T) {
	mock := &MockCmdRunner{
		stdout: []byte("only one line"), // requested 2
	}
	ctx := &OSExecPortageContext{Runner: mock}

	_, err := ctx.QueryMetadata("", "sys-apps", "test", "1.0", []string{"K1", "K2"})
	if err == nil || !strings.Contains(err.Error(), "expected 2") {
		t.Fatalf("expected malformed line count error, got: %v", err)
	}
}
