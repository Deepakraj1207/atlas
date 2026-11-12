package analysis

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// Input is what one analysis source contributes.
type Input struct {
	// GraphFor is the inventory source ID the graph describes ("dpkg", ...).
	GraphFor string
	Graph    Graph
	Usage    []UsageRecord
	IsUsage  bool
}

// Source is a read-only data source the analysis uses.
type Source interface {
	ID() string
	Label() string
	Detail() string
	Read(ctx context.Context) (Input, error)
}

// ---- dpkg ----

type dpkgDeps struct{ statusPath string }

// NewDpkgDeps reads Depends and Pre-Depends from dpkg's status file,
// resolving virtual packages through Provides.
func NewDpkgDeps(statusPath string) Source { return dpkgDeps{statusPath: statusPath} }

func (dpkgDeps) ID() string    { return "deps-dpkg" }
func (dpkgDeps) Label() string { return "Package dependencies (dpkg)" }
func (d dpkgDeps) Detail() string {
	return "Reads Depends, Pre-Depends and Provides from " + d.statusPath
}

func (d dpkgDeps) Read(ctx context.Context) (Input, error) {
	f, err := os.Open(d.statusPath)
	if errors.Is(err, fs.ErrNotExist) {
		return Input{}, core.ErrSourceUnavailable
	}
	if err != nil {
		return Input{}, err
	}
	defer f.Close()
	return parseDpkgDeps(ctx, f)
}

// depName strips version constraints and architecture qualifiers.
func depName(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " ("); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i]
	}
	return s
}

func parseDpkgDeps(ctx context.Context, r io.Reader) (Input, error) {
	type pkg struct{ name, deps string }
	var pkgs []pkg
	providers := map[string][]string{}
	g := newGraph()
	err := inventory.ReadStanzas(r, func(f map[string]string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		st := strings.Fields(f["Status"])
		if len(st) < 3 || st[2] != "installed" || f["Package"] == "" {
			return nil
		}
		name := f["Package"]
		g.addNode(name)
		switch {
		case f["Essential"] == "yes":
			g.Protected[key(name)] = "the package system marks it as essential"
		case f["Priority"] == "required":
			g.Protected[key(name)] = "the package system marks it as required"
		}
		for _, p := range strings.Split(f["Provides"], ",") {
			if v := depName(p); v != "" {
				providers[key(v)] = append(providers[key(v)], name)
			}
		}
		pkgs = append(pkgs, pkg{name, f["Pre-Depends"] + "," + f["Depends"]})
		return nil
	})
	if err != nil {
		return Input{}, err
	}
	for _, p := range pkgs {
		for _, group := range strings.Split(p.deps, ",") {
			for _, alt := range strings.Split(group, "|") {
				n := depName(alt)
				if n == "" {
					continue
				}
				if g.Has(n) {
					g.addDep(p.name, n)
				}
				for _, prov := range providers[key(n)] {
					g.addDep(p.name, prov)
				}
			}
		}
	}
	return Input{GraphFor: inventory.SourceDpkg, Graph: g}, nil
}

// ---- rpm ----

type rpmDeps struct{ run inventory.Runner }

// NewRPMDeps reads every package's requirements and capabilities from the
// RPM database in two queries and resolves requirements to packages.
func NewRPMDeps(run inventory.Runner) Source { return rpmDeps{run: run} }

func (rpmDeps) ID() string    { return "deps-rpm" }
func (rpmDeps) Label() string { return "Package dependencies (RPM)" }
func (rpmDeps) Detail() string {
	return "Runs: rpm -qa --qf '[%{NAME}\\t%{REQUIRENAME}\\n]' and the same for PROVIDENAME"
}

func (d rpmDeps) Read(ctx context.Context) (Input, error) {
	req, err := d.run(ctx, "rpm", "-qa", "--qf", "[%{NAME}\t%{REQUIRENAME}\n]")
	if err != nil {
		return Input{}, err
	}
	prov, err := d.run(ctx, "rpm", "-qa", "--qf", "[%{NAME}\t%{PROVIDENAME}\n]")
	if err != nil {
		return Input{}, err
	}
	return Input{GraphFor: inventory.SourceRPM, Graph: parseRPMDeps(string(req), string(prov))}, nil
}

func parseRPMDeps(requires, provides string) Graph {
	g := newGraph()
	providers := map[string][]string{}
	for _, l := range strings.Split(provides, "\n") {
		name, cap, ok := strings.Cut(strings.TrimSpace(l), "\t")
		if !ok || name == "" {
			continue
		}
		g.addNode(name)
		providers[cap] = append(providers[cap], name)
	}
	for _, l := range strings.Split(requires, "\n") {
		name, cap, ok := strings.Cut(strings.TrimSpace(l), "\t")
		if !ok || name == "" || strings.HasPrefix(cap, "rpmlib(") || strings.HasPrefix(cap, "/") {
			continue
		}
		g.addNode(name)
		for _, p := range providers[cap] {
			g.addDep(name, p)
		}
	}
	return g
}

// ---- Chocolatey ----

type chocoDeps struct{ root string }

// NewChocolateyDeps reads <dependency> entries from installed .nuspec files.
func NewChocolateyDeps(root string) Source { return chocoDeps{root: root} }

func (chocoDeps) ID() string    { return "deps-chocolatey" }
func (chocoDeps) Label() string { return "Package dependencies (Chocolatey)" }
func (chocoDeps) Detail() string {
	return "Reads <dependency> entries in <ChocolateyInstall>\\lib\\*\\*.nuspec"
}

func (d chocoDeps) Read(ctx context.Context) (Input, error) {
	if d.root == "" {
		return Input{}, core.ErrSourceUnavailable
	}
	lib := filepath.Join(d.root, "lib")
	entries, err := os.ReadDir(lib)
	if errors.Is(err, fs.ErrNotExist) {
		return Input{}, core.ErrSourceUnavailable
	}
	if err != nil {
		return Input{}, err
	}
	g := newGraph()
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return Input{}, err
		}
		data, err := os.ReadFile(filepath.Join(lib, e.Name(), e.Name()+".nuspec"))
		if err != nil {
			continue
		}
		id, deps := nuspecDeps(data)
		if id == "" {
			continue
		}
		g.addNode(id)
		for _, dep := range deps {
			g.addDep(id, dep)
		}
	}
	return Input{GraphFor: inventory.SourceChocolatey, Graph: g}, nil
}

// nuspecDeps returns a package's ID and dependency IDs, wherever the
// <dependency> elements are nested (with or without <group>).
func nuspecDeps(data []byte) (id string, deps []string) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	inID := false
	for {
		tok, err := dec.Token()
		if err != nil {
			return id, deps
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "id":
				inID = id == ""
			case "dependency":
				for _, a := range t.Attr {
					if a.Name.Local == "id" && a.Value != "" {
						deps = append(deps, a.Value)
					}
				}
			}
		case xml.CharData:
			if inID {
				id = strings.TrimSpace(string(t))
			}
		case xml.EndElement:
			inID = false
		}
	}
}

// ---- Scoop ----

type scoopDeps struct{ roots []inventory.ScoopRoot }

// NewScoopDeps reads "depends" from installed Scoop manifests.
func NewScoopDeps(roots []inventory.ScoopRoot) Source { return scoopDeps{roots: roots} }

func (scoopDeps) ID() string    { return "deps-scoop" }
func (scoopDeps) Label() string { return "Package dependencies (Scoop)" }
func (scoopDeps) Detail() string {
	return "Reads \"depends\" in <scoop>\\apps\\*\\current\\manifest.json"
}

func (d scoopDeps) Read(ctx context.Context) (Input, error) {
	g := newGraph()
	found := false
	for _, root := range d.roots {
		apps := filepath.Join(root.Dir, "apps")
		entries, err := os.ReadDir(apps)
		if err != nil {
			continue
		}
		found = true
		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return Input{}, err
			}
			data, err := os.ReadFile(filepath.Join(apps, e.Name(), "current", "manifest.json"))
			if err != nil {
				continue
			}
			g.addNode(e.Name())
			for _, dep := range scoopDepends(data) {
				if i := strings.LastIndex(dep, "/"); i >= 0 {
					dep = dep[i+1:] // "bucket/app" -> "app"
				}
				g.addDep(e.Name(), dep)
			}
		}
	}
	if !found {
		return Input{}, core.ErrSourceUnavailable
	}
	return Input{GraphFor: inventory.SourceScoop, Graph: g}, nil
}

// scoopDepends accepts "depends" as a string or a list.
func scoopDepends(data []byte) []string {
	var m struct {
		Depends json.RawMessage `json:"depends"`
	}
	if json.Unmarshal(data, &m) != nil || len(m.Depends) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(m.Depends, &one) == nil {
		return []string{one}
	}
	var many []string
	_ = json.Unmarshal(m.Depends, &many)
	return many
}
