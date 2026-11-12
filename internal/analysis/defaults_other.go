//go:build !windows

package analysis

import "github.com/deepakraj1207/atlas/internal/inventory"

// DefaultSources returns the Linux analysis sources.
func DefaultSources() []Source {
	return []Source{
		NewDpkgDeps("/var/lib/dpkg/status"),
		NewRPMDeps(inventory.ExecRunner),
	}
}

// UsageSupported reports whether this platform records program use.
const UsageSupported = false
