package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// fakeDeps is a dependency source whose outcome a test chooses.
type fakeDeps struct {
	id, covers string
	in         analysis.Input
	err        error
	hang       bool // block until the read is stopped (a timeout)
}

func (f fakeDeps) ID() string     { return f.id }
func (f fakeDeps) Label() string  { return "Package dependencies (" + f.covers + ")" }
func (f fakeDeps) Detail() string { return "test" }
func (f fakeDeps) Covers() string { return f.covers }
func (f fakeDeps) Read(ctx context.Context) (analysis.Input, error) {
	if f.hang {
		<-ctx.Done()
		return analysis.Input{}, ctx.Err()
	}
	return f.in, f.err
}

func graphOf(source string, nodes []string, deps map[string][]string) analysis.Input {
	g := analysis.Graph{Deps: map[string][]string{}, Nodes: map[string]bool{}, Protected: map[string]string{}}
	for _, n := range nodes {
		g.Nodes[n] = true
	}
	for p, ds := range deps {
		g.Deps[p] = ds
	}
	return analysis.Input{GraphFor: source, Graph: g}
}

func gateSvc(t *testing.T, deps ...analysis.Source) *Service {
	t.Helper()
	s, _ := newSvc(t, Config{
		Removers: []Remover{NewAptRemover(false), NewScoopRemover(false), NewWingetRemover(false)},
		Deps:     deps,
	})
	s.depTimeout = 100 * time.Millisecond
	return s
}

func onlyStep(t *testing.T, s *Service, it core.SoftwareItem) core.UpdateStep {
	t.Helper()
	steps, err := s.PlanRemoval(context.Background(), []core.SoftwareItem{it})
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps=%+v err=%v", steps, err)
	}
	return steps[0]
}

// The audit's P0: a dependency source that failed, timed out or wasn't there
// must not be read as "no dependency records". The removal is refused.
func TestGateFailsClosedWhenDependenciesCantBeVerified(t *testing.T) {
	app := deb("meld", core.KindApp)
	for name, c := range map[string]struct {
		src  fakeDeps
		want string
	}{
		"failed":      {fakeDeps{id: "deps-dpkg", covers: inventory.SourceDpkg, err: errors.New("permission denied")}, "couldn't read"},
		"timed out":   {fakeDeps{id: "deps-dpkg", covers: inventory.SourceDpkg, hang: true}, "took too long"},
		"unavailable": {fakeDeps{id: "deps-dpkg", covers: inventory.SourceDpkg, err: core.ErrSourceUnavailable}, "isn't available"},
		"empty":       {fakeDeps{id: "deps-dpkg", covers: inventory.SourceDpkg, in: graphOf(inventory.SourceDpkg, nil, nil)}, "came back empty"},
		"not listed":  {fakeDeps{id: "deps-dpkg", covers: inventory.SourceDpkg, in: graphOf(inventory.SourceDpkg, []string{"python3"}, nil)}, "didn't list this package"},
	} {
		st := onlyStep(t, gateSvc(t, c.src), app)
		if st.Blocked == "" || !strings.Contains(st.Blocked, c.want) {
			t.Errorf("%s: an app must be blocked when its dependencies can't be verified; got %q", name, st.Blocked)
		}
		if len(st.Commands) != 0 {
			t.Errorf("%s: a blocked step must have no commands", name)
		}
	}
}

func TestGateStillAllowsAppsWhenEvidenceIsComplete(t *testing.T) {
	dpkg := fakeDeps{id: "deps-dpkg", covers: inventory.SourceDpkg,
		in: graphOf(inventory.SourceDpkg, []string{"meld", "python3"}, map[string][]string{"meld": {"python3"}})}
	if st := onlyStep(t, gateSvc(t, dpkg), deb("meld", core.KindApp)); st.Blocked != "" {
		t.Errorf("nothing needs meld: it may be removed; got %q", st.Blocked)
	}
	if st := onlyStep(t, gateSvc(t, dpkg), deb("python3", core.KindApp)); !strings.Contains(st.Blocked, "Needed by meld") {
		t.Errorf("python3 is needed by meld; got %q", st.Blocked)
	}
	// A failure in a source this item doesn't come from doesn't block it.
	brokenScoop := fakeDeps{id: "deps-scoop", covers: inventory.SourceScoop, err: errors.New("disk error")}
	if st := onlyStep(t, gateSvc(t, dpkg, brokenScoop), deb("meld", core.KindApp)); st.Blocked != "" {
		t.Errorf("an unrelated failed source must not block; got %q", st.Blocked)
	}
	// Software from a source that keeps no dependency records (winget) is
	// unchanged: apps allowed, runtimes and components refused.
	winget := core.SoftwareItem{Name: "Obsidian", PackageID: "Obsidian.Obsidian", Source: inventory.SourceWinget, Kind: core.KindApp}
	if st := onlyStep(t, gateSvc(t, dpkg, brokenScoop), winget); st.Blocked != "" {
		t.Errorf("winget app: got %q", st.Blocked)
	}
}

// Scoop's graph uses bare app names; inventory uses bucket/app. Before the
// fix Scoop items never matched their graph.
func TestGateMatchesScoopAppsByBareName(t *testing.T) {
	scoop := fakeDeps{id: "deps-scoop", covers: inventory.SourceScoop,
		in: graphOf(inventory.SourceScoop, []string{"7zip", "innounp", "aria2"}, map[string][]string{"innounp": {"7zip"}})}
	it := func(id string) core.SoftwareItem {
		return core.SoftwareItem{Name: id[strings.Index(id, "/")+1:], PackageID: id, Source: inventory.SourceScoop, Kind: core.KindApp}
	}
	if st := onlyStep(t, gateSvc(t, scoop), it("main/7zip")); !strings.Contains(st.Blocked, "Needed by innounp") {
		t.Errorf("7zip is needed by innounp; got %q", st.Blocked)
	}
	if st := onlyStep(t, gateSvc(t, scoop), it("main/aria2")); st.Blocked != "" {
		t.Errorf("nothing needs aria2; got %q", st.Blocked)
	}
	// Removing 7zip together with innounp, the app that needs it, is fine.
	steps, err := gateSvc(t, scoop).PlanRemoval(context.Background(), []core.SoftwareItem{it("main/7zip"), it("extras/innounp")})
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range steps {
		if st.Blocked != "" {
			t.Errorf("both removed together: %s blocked: %q", st.Updates[0].Name, st.Blocked)
		}
	}
}

func TestUnverifiedLabelWithoutName(t *testing.T) {
	r := depRecords{result: core.SourceResult{Source: "deps-x", Status: core.SourceFailed}}
	if why := r.unverified(); !strings.Contains(why, "dependency records for deps-x") {
		t.Errorf("got %q", why)
	}
}
