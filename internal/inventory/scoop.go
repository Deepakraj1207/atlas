package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/deepakraj1207/atlas/internal/core"
)

// ScoopRoot is one Scoop installation directory.
type ScoopRoot struct {
	Dir   string
	Scope string // "user" or "machine"
}

type scoopScanner struct{ roots []ScoopRoot }

// NewScoopScanner reads Scoop's app folders directly instead of running the
// PowerShell-based "scoop list", which is slower and its output varies.
func NewScoopScanner(roots []ScoopRoot) core.Scanner { return scoopScanner{roots: roots} }

func (scoopScanner) ID() string    { return SourceScoop }
func (scoopScanner) Label() string { return "Scoop" }
func (s scoopScanner) Detail() string {
	return "Reads <scoop>\\apps\\*\\current\\manifest.json and install.json"
}

type scoopManifest struct {
	Version     string `json:"version"`
	Description string `json:"description"`
}

type scoopInstall struct {
	Bucket string `json:"bucket"`
}

func (s scoopScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	var items []core.SoftwareItem
	found := false
	for _, root := range s.roots {
		appsDir := filepath.Join(root.Dir, "apps")
		entries, err := os.ReadDir(appsDir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		found = true
		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !e.IsDir() || e.Name() == "scoop" {
				continue
			}
			current := filepath.Join(appsDir, e.Name(), "current")
			var m scoopManifest
			if !readJSON(filepath.Join(current, "manifest.json"), &m) {
				continue // incomplete or broken install
			}
			var inst scoopInstall
			readJSON(filepath.Join(current, "install.json"), &inst)
			id := e.Name()
			if inst.Bucket != "" {
				id = inst.Bucket + "/" + e.Name()
			}
			items = append(items, core.SoftwareItem{
				Name:            e.Name(),
				Version:         m.Version,
				Description:     m.Description,
				PackageID:       id,
				InstallLocation: current,
				Source:          SourceScoop,
				Kind:            core.KindApp,
				Scope:           root.Scope,
			})
		}
	}
	if !found {
		return nil, core.ErrSourceUnavailable
	}
	return items, nil
}

func readJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}
