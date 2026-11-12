//go:build windows

package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/deepakraj1207/atlas/internal/core"
)

type keyValues struct{ k registry.Key }

func (kv keyValues) String(name string) (string, bool) {
	s, _, err := kv.k.GetStringValue(name)
	return s, err == nil
}

func (kv keyValues) Int(name string) (uint64, bool) {
	n, _, err := kv.k.GetIntegerValue(name)
	return n, err == nil
}

// Scan opens every Uninstall key read-only. It fails only if none of the
// locations could be opened.
func (registryScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	const path = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	locations := []struct {
		root  registry.Key
		path  string
		scope string
	}{
		{registry.LOCAL_MACHINE, path, "machine"},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, "machine"},
		{registry.CURRENT_USER, path, "user"},
	}
	var items []core.SoftwareItem
	var errs []string
	opened := 0
	for _, loc := range locations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		k, err := registry.OpenKey(loc.root, loc.path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
		if err != nil {
			if !errors.Is(err, registry.ErrNotExist) {
				errs = append(errs, fmt.Sprintf("%s: %v", loc.path, err))
			}
			continue
		}
		opened++
		names, err := k.ReadSubKeyNames(-1)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", loc.path, err))
		}
		for _, n := range names {
			if err := ctx.Err(); err != nil {
				k.Close()
				return nil, err
			}
			sk, err := registry.OpenKey(k, n, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			if it, ok := registryItem(keyValues{sk}, loc.scope); ok {
				items = append(items, it)
			}
			sk.Close()
		}
		k.Close()
	}
	if opened == 0 {
		return nil, fmt.Errorf("could not open any Uninstall registry key: %s", strings.Join(errs, "; "))
	}
	return items, nil
}
