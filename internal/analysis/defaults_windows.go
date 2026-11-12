//go:build windows

package analysis

import "github.com/deepakraj1207/atlas/internal/inventory"

// DefaultSources returns the Windows analysis sources.
func DefaultSources() []Source {
	return []Source{
		NewChocolateyDeps(inventory.ChocolateyRoot()),
		NewScoopDeps(inventory.ScoopRoots()),
		NewUsageSource(),
	}
}

// UsageSupported reports whether this platform records program use.
const UsageSupported = true
