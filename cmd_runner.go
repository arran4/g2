package g2

import (
	"bytes"
	"errors"
	"os/exec"
)

// ErrPortageUnavailable is returned when portageq cannot be found or run.
var ErrPortageUnavailable = errors.New("portage capability unavailable")

// CmdRunner encapsulates os/exec calls for testing.
type CmdRunner interface {
	Run(cmdName string, args ...string) ([]byte, []byte, error)
}

// OSExecRunner implements CmdRunner using real os/exec.
type OSExecRunner struct{}

func (r *OSExecRunner) Run(cmdName string, args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(cmdName, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return stdout.Bytes(), stderr.Bytes(), ErrPortageUnavailable
		}
	}
	return stdout.Bytes(), stderr.Bytes(), err
}
