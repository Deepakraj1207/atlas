package inventory

import (
	"context"
	"encoding/xml"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

type chocoScanner struct{ root string }

// NewChocolateyScanner reads package metadata (.nuspec) from Chocolatey's
// lib folder. This avoids "choco list", whose meaning changed between
// Chocolatey v1 (remote) and v2 (local).
func NewChocolateyScanner(root string) core.Scanner { return chocoScanner{root: root} }

func (chocoScanner) ID() string     { return SourceChocolatey }
func (chocoScanner) Label() string  { return "Chocolatey" }
func (chocoScanner) Detail() string { return "Reads <ChocolateyInstall>\\lib\\*\\*.nuspec" }

type nuspec struct {
	Metadata struct {
		ID          string `xml:"id"`
		Version     string `xml:"version"`
		Title       string `xml:"title"`
		Authors     string `xml:"authors"`
		Summary     string `xml:"summary"`
		Description string `xml:"description"`
	} `xml:"metadata"`
}

func (s chocoScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	if s.root == "" {
		return nil, core.ErrSourceUnavailable
	}
	libDir := filepath.Join(s.root, "lib")
	entries, err := os.ReadDir(libDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, core.ErrSourceUnavailable
	}
	if err != nil {
		return nil, err
	}
	var items []core.SoftwareItem
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(libDir, e.Name(), e.Name()+".nuspec"))
		if err != nil {
			continue
		}
		var n nuspec
		if xml.Unmarshal(data, &n) != nil || n.Metadata.ID == "" {
			continue
		}
		md := n.Metadata
		name := strings.TrimSpace(md.Title)
		if name == "" {
			name = md.ID
		}
		desc := strings.TrimSpace(md.Summary)
		if desc == "" {
			desc = firstLine(md.Description)
		}
		items = append(items, core.SoftwareItem{
			Name:            name,
			Version:         md.Version,
			Publisher:       md.Authors,
			PackageID:       md.ID,
			Description:     desc,
			InstallLocation: filepath.Join(libDir, e.Name()),
			Source:          SourceChocolatey,
			Kind:            classifyChoco(md.ID, name),
			Scope:           "machine",
		})
	}
	return items, nil
}

func classifyChoco(id, name string) core.Kind {
	l := strings.ToLower(id)
	if l == "chocolatey" || strings.HasPrefix(l, "chocolatey-") || strings.HasSuffix(l, ".extension") ||
		strings.HasSuffix(l, ".template") || (strings.HasPrefix(l, "kb") && len(l) > 2 && l[2] >= '0' && l[2] <= '9') {
		return core.KindComponent
	}
	return classifyWindowsName(name)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
