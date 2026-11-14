package discover

import (
	"runtime"

	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
	"github.com/deepakraj1207/atlas/internal/updates"
)

// DefaultConfig returns the configuration for this computer.
func DefaultConfig(elevated bool) Config {
	cfg := Config{Elevated: elevated, Run: inventory.ExecRunner}
	if runtime.GOOS == "windows" {
		cfg.ScoopRoots = scoopRoots()
	} else {
		cfg.AptListsDir = updates.AptListsDir
		cfg.RPMManager = platform.LinuxRPMRoute().Manager
	}
	return cfg
}
