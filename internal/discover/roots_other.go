//go:build !windows

package discover

import "github.com/deepakraj1207/atlas/internal/inventory"

func scoopRoots() []inventory.ScoopRoot { return nil }
