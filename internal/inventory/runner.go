package inventory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Runner runs a read-only command and returns its standard output. stdout
// is returned even when err is non-nil so callers can use partial output.
// A missing program must yield an error wrapping core.ErrSourceUnavailable.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// CommandError describes a command that ran but failed.
type CommandError struct {
	Command  string
	ExitCode int
	Stderr   string
	Err      error
}

func (e *CommandError) Error() string {
	msg := fmt.Sprintf("%s exited with code %d", e.Command, e.ExitCode)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *CommandError) Unwrap() error { return e.Err }

// ExecRunner runs programs found on PATH with no stdin, a C locale on Unix
// (so output is not translated), and a bounded wait after cancellation.
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%s not found: %w", name, core.ErrSourceUnavailable)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = nil
	if runtime.GOOS != "windows" {
		cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	}
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	if ctx.Err() != nil {
		return stdout.Bytes(), ctx.Err()
	}
	ce := &CommandError{Command: name, ExitCode: -1, Stderr: lastLine(stderr.String()), Err: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		ce.ExitCode = exitErr.ExitCode()
	}
	return stdout.Bytes(), ce
}

// lastLine returns the last non-empty line, trimmed to a reasonable length.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	l := strings.TrimSpace(lines[len(lines)-1])
	if len(l) > 200 {
		l = l[:200] + "…"
	}
	return l
}
