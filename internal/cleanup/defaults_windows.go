//go:build windows

package cleanup

import (
	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// DefaultConfig returns the Windows removers and tidiers.
func DefaultConfig(elevated bool) Config {
	roots := inventory.ScoopRoots()
	return Config{
		Removers: []Remover{NewWingetRemover(elevated), NewChocolateyRemover(elevated), NewScoopRemover(elevated)},
		Tidiers:  []Tidier{NewScoopOldVersions(roots, elevated), NewScoopCache(roots)},
		Deps:     []analysis.Source{analysis.NewChocolateyDeps(inventory.ChocolateyRoot()), analysis.NewScoopDeps(roots)},
	}
}
