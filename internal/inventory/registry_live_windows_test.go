//go:build windows

package inventory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// These tests read the real Windows registry and run real commands. They
// only run on Windows and change nothing.

func TestRegistryScanReadsThisComputer(t *testing.T) {
	items, err := NewRegistryScanner().Scan(context.Background())
	if err != nil {
		t.Fatalf("registry scan: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("a Windows installation always lists some installed programs")
	}
	machine := 0
	for _, it := range items {
		if strings.TrimSpace(it.Name) == "" || it.Source != SourceRegistry {
			t.Errorf("bad item: %+v", it)
		}
		if it.Scope != "machine" && it.Scope != "user" {
			t.Errorf("scope must be machine or user: %+v", it)
		}
		if it.Scope == "machine" {
			machine++
		}
		if it.InstallDate != "" && len(it.InstallDate) != 10 {
			t.Errorf("install date must be YYYY-MM-DD: %+v", it)
		}
	}
	if machine == 0 {
		t.Error("expected some programs installed for all users")
	}
	t.Logf("%d programs found in the registry", len(items))
}

func TestExecRunnerOnWindows(t *testing.T) {
	out, err := ExecRunner(context.Background(), "cmd", "/c", "echo atlas-ok")
	if err != nil || !strings.Contains(string(out), "atlas-ok") {
		t.Fatalf("cmd /c echo: %q %v", out, err)
	}
	_, err = ExecRunner(context.Background(), "cmd", "/c", "exit 3")
	var ce *CommandError
	if err == nil || !asCommandError(err, &ce) || ce.ExitCode != 3 {
		t.Errorf("exit code 3 should be reported: %v", err)
	}
}

func asCommandError(err error, target **CommandError) bool {
	ce, ok := err.(*CommandError)
	if ok {
		*target = ce
	}
	return ok
}

func TestDefaultScannersRunOnWindows(t *testing.T) {
	r := RunScanners(context.Background(), DefaultScanners(), func(string) time.Duration { return 2 * time.Minute }, func(core.SourceEvent) {}, time.Now)
	for _, s := range r.Sources {
		t.Logf("%s: %s, %d found %s", s.Label, s.Status, s.Count, s.Details)
		if s.Source == SourceRegistry && s.Status != core.SourceOK {
			t.Errorf("the registry must always be readable: %+v", s)
		}
	}
}
