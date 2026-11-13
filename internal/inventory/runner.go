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
	msg := fmt.Sprintf("%s exited with code %s", e.Command, FormatExitCode(e.ExitCode))
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// FormatExitCode shows Windows HRESULT-style codes (such as winget's
// 0x8A15xxxx errors) in hexadecimal, the form their documentation uses.
func FormatExitCode(code int) string {
	if code > 0xFFFF || code < -0xFFFF {
		return fmt.Sprintf("0x%08X", uint32(code))
	}
	return fmt.Sprint(code)
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
	// Some tools (winget among them) print their error to stdout.
	msg := lastLine(stderr.String())
	if msg == "" {
		msg = lastLine(stdout.String())
	}
	ce := &CommandError{Command: name, ExitCode: -1, Stderr: msg, Err: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		ce.ExitCode = exitErr.ExitCode()
	}
	return stdout.Bytes(), ce
}

// lastLine returns the last non-empty line, trimmed to a reasonable length.
func lastLine(s string) string {
	// Drop spinner frames drawn with carriage returns.
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if i := strings.LastIndex(strings.TrimRight(l, "\r "), "\r"); i >= 0 {
			l = l[i+1:]
		}
		if t := strings.Trim(strings.TrimSpace(l), `-\|/`); t != "" {
			lines = append(lines, strings.TrimSpace(l))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	l := lines[len(lines)-1]
	if len(l) > 200 {
		l = l[:200] + "…"
	}
	return l
}
