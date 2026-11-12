//go:build !windows

package inventory

import (
	"context"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Scan reports the registry as unavailable outside Windows.
func (registryScanner) Scan(context.Context) ([]core.SoftwareItem, error) {
	return nil, core.ErrSourceUnavailable
}
