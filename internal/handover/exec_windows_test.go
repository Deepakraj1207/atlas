//go:build windows

package handover

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
)

func TestExecStarterExitCodesOnWindows(t *testing.T) {
	code, err := ExecStarter(core.Command{Program: "cmd", Args: []string{"/c", "exit 7"}}, strings.NewReader(""), io.Discard, io.Discard)
	if err != nil || code != 7 {
		t.Errorf("exit 7: code %d, err %v", code, err)
	}
}

// Scoop and Chocolatey install .cmd shims; ATLAS must find and run them
// through PATH the same way a terminal does.
func TestExecStarterRunsCmdShimsFromPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "atlasshim.cmd"), []byte("@echo off\r\necho shim %1\r\nexit /b 4\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out strings.Builder
	code, err := ExecStarter(core.Command{Program: "atlasshim", Args: []string{"hello"}}, strings.NewReader(""), &out, io.Discard)
	if err != nil || code != 4 || !strings.Contains(out.String(), "shim hello") {
		t.Errorf("shim: code %d, err %v, out %q", code, err, out.String())
	}
}

func TestExecStarterMissingProgram(t *testing.T) {
	if _, err := ExecStarter(core.Command{Program: "atlas-no-such-program"}, nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "isn't installed") {
		t.Errorf("missing program: %v", err)
	}
}
