//go:build !windows

package inventory

import (
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// DefaultScanners returns the Linux sources, highest priority first.
func DefaultScanners() []core.Scanner {
	return []core.Scanner{
		NewDpkgScanner("/var/lib/dpkg/status", "/var/lib/apt/extended_states"),
		NewRPMScanner(ExecRunner),
		NewSnapScanner(ExecRunner),
		NewFlatpakScanner(ExecRunner),
	}
}

// DefaultTimeouts has no per-source overrides on Linux.
func DefaultTimeouts() map[string]time.Duration { return map[string]time.Duration{} }
