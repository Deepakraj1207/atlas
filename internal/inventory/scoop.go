package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	return "Reads <scoop>\\apps\\*\\current\\scoop-manifest.json and scoop-install.json (manifest.json and install.json in older Scoop)"
}

// ScoopFile returns the path of one of Scoop's metadata files in an app's
// "current" folder. Newer Scoop names them scoop-manifest.json and
// scoop-install.json; older Scoop used manifest.json and install.json, and
// apps installed back then keep the old names. name is the old name.
func ScoopFile(currentDir, name string) string {
	p := filepath.Join(currentDir, "scoop-"+name)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return filepath.Join(currentDir, name)
}

// ReadScoopJSON reads a Scoop metadata file, accepting a leading UTF-8
// byte-order mark (Windows PowerShell 5.1 can write one).
func ReadScoopJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	data = []byte(strings.TrimPrefix(string(data), "\uFEFF"))
	return json.Unmarshal(data, v) == nil
}

// scoopLinkedVersion is the version folder "current" points to
// (apps\7zip\current -> apps\7zip\26.06), or "" if it isn't a link.
func scoopLinkedVersion(currentDir string) string {
	target, err := os.Readlink(currentDir)
	if err != nil {
		return ""
	}
	v := filepath.Base(target)
	if v == "current" || v == "." {
		return ""
	}
	return v
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
			if _, err := os.Stat(current); err != nil {
				continue // not installed (no current version)
			}
			var m scoopManifest
			if !ReadScoopJSON(ScoopFile(current, "manifest.json"), &m) {
				// Still list it: the app is installed even if its metadata
				// can't be read. The version folder names the version.
				m.Version = scoopLinkedVersion(current)
			}
			var inst scoopInstall
			ReadScoopJSON(ScoopFile(current, "install.json"), &inst)
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
