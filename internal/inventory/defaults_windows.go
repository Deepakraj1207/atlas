//go:build windows

package inventory

import (
	"os"
	"path/filepath"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// DefaultScanners returns the Windows sources, highest priority first:
// the registry provides the richest metadata, package managers add IDs.
func DefaultScanners() []core.Scanner {
	var scoopRoots []ScoopRoot
	if dir := os.Getenv("SCOOP"); dir != "" {
		scoopRoots = append(scoopRoots, ScoopRoot{Dir: dir, Scope: "user"})
	} else if home, err := os.UserHomeDir(); err == nil {
		scoopRoots = append(scoopRoots, ScoopRoot{Dir: filepath.Join(home, "scoop"), Scope: "user"})
	}
	if dir := os.Getenv("SCOOP_GLOBAL"); dir != "" {
		scoopRoots = append(scoopRoots, ScoopRoot{Dir: dir, Scope: "machine"})
	} else if pd := os.Getenv("ProgramData"); pd != "" {
		scoopRoots = append(scoopRoots, ScoopRoot{Dir: filepath.Join(pd, "scoop"), Scope: "machine"})
	}

	chocoRoot := os.Getenv("ChocolateyInstall")
	if chocoRoot == "" {
		if pd := os.Getenv("ProgramData"); pd != "" {
			chocoRoot = filepath.Join(pd, "chocolatey")
		}
	}

	return []core.Scanner{
		NewRegistryScanner(),
		NewWingetScanner(ExecRunner),
		NewChocolateyScanner(chocoRoot),
		NewScoopScanner(scoopRoots),
	}
}

// DefaultTimeouts gives slow sources more time. winget can take a while
// the first time it builds its index.
func DefaultTimeouts() map[string]time.Duration {
	return map[string]time.Duration{SourceWinget: 90 * time.Second}
}
