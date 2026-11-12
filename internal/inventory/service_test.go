package inventory

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
)

func isUnavailable(err error) bool { return errors.Is(err, core.ErrSourceUnavailable) }

// fakeScanner is a test double. Its items are test data only.
type fakeScanner struct {
	id    string
	items []core.SoftwareItem
	err   error
	delay time.Duration
	panic bool
}

func (f fakeScanner) ID() string     { return f.id }
func (f fakeScanner) Label() string  { return "Label " + f.id }
func (f fakeScanner) Detail() string { return "detail " + f.id }
func (f fakeScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	if f.panic {
		panic("boom")
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.items, f.err
}

func item(name, version, source string) core.SoftwareItem {
	return core.SoftwareItem{Name: name, Version: version, Source: source, Kind: core.KindApp}
}

func fixedTimeout(d time.Duration) func(string) time.Duration {
	return func(string) time.Duration { return d }
}

func TestRunScannersStatuses(t *testing.T) {
	scanners := []core.Scanner{
		fakeScanner{id: "ok", items: []core.SoftwareItem{item("A", "1", "ok")}},
		fakeScanner{id: "missing", err: core.ErrSourceUnavailable},
		fakeScanner{id: "broken", err: errors.New("exit status 3")},
		fakeScanner{id: "slow", delay: time.Second},
		fakeScanner{id: "panics", panic: true},
	}
	var mu sync.Mutex
	var events []core.SourceEvent
	r := RunScanners(context.Background(), scanners, fixedTimeout(50*time.Millisecond),
		func(ev core.SourceEvent) { mu.Lock(); events = append(events, ev); mu.Unlock() }, time.Now)

	want := []core.SourceStatus{core.SourceOK, core.SourceUnavailable, core.SourceFailed, core.SourceTimeout, core.SourceFailed}
	for i, s := range r.Sources {
		if s.Status != want[i] {
			t.Errorf("source %s: status %s, want %s", s.Source, s.Status, want[i])
		}
		if s.Source != scanners[i].ID() || s.Label == "" {
			t.Errorf("source order/labels wrong: %+v", s)
		}
	}
	if r.Sources[2].Details != "exit status 3" || !strings.Contains(r.Sources[4].Details, "boom") {
		t.Errorf("failure details missing: %+v / %+v", r.Sources[2], r.Sources[4])
	}
	if r.Status != core.ScanPartial || len(r.Items) != 1 {
		t.Errorf("status = %s, items = %d", r.Status, len(r.Items))
	}
	if len(events) != 2*len(scanners) {
		t.Errorf("want start+finish events per source, got %d", len(events))
	}
}

func TestOverallStatus(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		scanners []core.Scanner
		want     core.ScanStatus
	}{
		{[]core.Scanner{fakeScanner{id: "a"}, fakeScanner{id: "b", err: core.ErrSourceUnavailable}}, core.ScanComplete},
		{[]core.Scanner{fakeScanner{id: "a", err: errors.New("x")}}, core.ScanFailed},
		{[]core.Scanner{fakeScanner{id: "a", err: core.ErrSourceUnavailable}}, core.ScanNothingToScan},
		{nil, core.ScanNothingToScan},
	}
	for i, c := range cases {
		if got := RunScanners(ctx, c.scanners, fixedTimeout(time.Second), func(core.SourceEvent) {}, time.Now).Status; got != c.want {
			t.Errorf("case %d: %s, want %s", i, got, c.want)
		}
	}
}

func TestRunScannersCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	r := RunScanners(ctx, []core.Scanner{fakeScanner{id: "slow", delay: 5 * time.Second, items: []core.SoftwareItem{item("A", "1", "slow")}}},
		fixedTimeout(10*time.Second), func(core.SourceEvent) {}, time.Now)
	if r.Status != core.ScanCancelled || r.Sources[0].Status != core.SourceCancelled || len(r.Items) != 0 {
		t.Errorf("cancelled scan: %+v", r)
	}
}

func TestMerge(t *testing.T) {
	reg := core.SoftwareItem{Name: "Microsoft Visual C++ 2015-2022 Redistributable (x64)", Version: "14.38.33135.0", Publisher: "Microsoft", Source: SourceRegistry, Kind: core.KindComponent}
	wg := core.SoftwareItem{Name: "Microsoft Visual C++ 2015-2022 Redist…", Version: "14.38.33135.0", PackageID: "Microsoft.VCRedist.2015+.x64", Source: SourceWinget, Kind: core.KindComponent}
	git1 := core.SoftwareItem{Name: "Git", Version: "2.44.0", Publisher: "The Git Development Community", Source: SourceRegistry, Kind: core.KindApp}
	git2 := core.SoftwareItem{Name: "git", Version: "2.44.0", PackageID: "Git.Git", Source: SourceWinget, Kind: core.KindApp}
	py311 := core.SoftwareItem{Name: "Python", Version: "3.11.9", Source: SourceRegistry, Kind: core.KindApp}
	py312 := core.SoftwareItem{Name: "Python", Version: "3.12.3", Source: SourceRegistry, Kind: core.KindApp}
	noVer := core.SoftwareItem{Name: "Python", Source: SourceScoop, Kind: core.KindApp}

	out := Merge([]core.SoftwareItem{reg, git1, py311, py312, wg, git2, noVer})
	if len(out) != 4 {
		t.Fatalf("want 4 merged items, got %d: %+v", len(out), out)
	}
	if out[0].Name != "Git" || out[0].PackageID != "Git.Git" || out[0].Source != SourceRegistry || len(out[0].AlsoFoundIn) != 1 {
		t.Errorf("git merge: %+v", out[0])
	}
	vc := out[1]
	if !strings.HasSuffix(vc.Name, "(x64)") || vc.PackageID == "" || vc.Publisher != "Microsoft" {
		t.Errorf("truncated name should merge into full one: %+v", vc)
	}
	if out[2].Version != "3.11.9" || out[3].Version != "3.12.3" {
		t.Errorf("different versions must stay separate: %+v %+v", out[2], out[3])
	}
	if len(out[2].AlsoFoundIn) != 1 || out[2].AlsoFoundIn[0] != SourceScoop {
		t.Errorf("unversioned item should merge into the first match: %+v", out[2])
	}
}

func newTestService(t *testing.T, scanners ...core.Scanner) (*Service, *database.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewService(scanners, db, nil, nil), db
}

func TestServiceSavesCompleteScanAndRecordsActivity(t *testing.T) {
	svc, _ := newTestService(t, fakeScanner{id: "a", items: []core.SoftwareItem{item("A", "1", "a"), {Name: "libz", Source: "a", Kind: core.KindComponent}}})
	ctx := context.Background()
	r, err := svc.Scan(ctx, nil)
	if err != nil || r.Status != core.ScanComplete {
		t.Fatalf("scan: %v %v", r.Status, err)
	}
	sum, ok, _ := svc.LatestScan(ctx)
	if !ok || sum.AppCount != 1 || sum.ComponentCount != 1 {
		t.Errorf("summary = %+v", sum)
	}
	acts, _ := svc.RecentActivity(ctx, 5)
	if len(acts) != 1 || acts[0].Outcome != core.OutcomeSuccess || !strings.Contains(acts[0].Summary, "1 app and 1 system component") {
		t.Errorf("activity = %+v", acts)
	}
	if !strings.Contains(acts[0].Details, "Label a: 2 found") {
		t.Errorf("details = %q", acts[0].Details)
	}
}

func TestServiceKeepsPreviousListWhenScanFails(t *testing.T) {
	good, db := newTestService(t, fakeScanner{id: "a", items: []core.SoftwareItem{item("Kept", "1", "a")}})
	ctx := context.Background()
	if _, err := good.Scan(ctx, nil); err != nil {
		t.Fatal(err)
	}
	bad := NewService([]core.Scanner{fakeScanner{id: "a", err: errors.New("nope")}}, db, nil, nil)
	r, err := bad.Scan(ctx, nil)
	if err != nil || r.Status != core.ScanFailed {
		t.Fatalf("got %v %v", r.Status, err)
	}
	items, _ := bad.Software(ctx)
	if len(items) != 1 || items[0].Name != "Kept" {
		t.Errorf("failed scan must not replace the list: %+v", items)
	}
	acts, _ := bad.RecentActivity(ctx, 1)
	if acts[0].Outcome != core.OutcomeFailed || !strings.Contains(acts[0].Summary, "previous list was kept") {
		t.Errorf("activity = %+v", acts[0])
	}
}

func TestServiceClearInventory(t *testing.T) {
	svc, _ := newTestService(t, fakeScanner{id: "a", items: []core.SoftwareItem{item("A", "1", "a")}})
	ctx := context.Background()
	svc.Scan(ctx, nil)
	if err := svc.ClearInventory(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := svc.LatestScan(ctx); ok {
		t.Error("list should be cleared")
	}
	acts, _ := svc.RecentActivity(ctx, 5)
	if len(acts) != 2 || acts[0].Kind != "inventory.clear" || !strings.Contains(acts[0].Details, "Removed 1 apps") {
		t.Errorf("activity = %+v", acts)
	}
}

func TestScanActivityTexts(t *testing.T) {
	src := []core.SourceResult{
		{Label: "Winget", Status: core.SourceTimeout, Message: "Took longer than 1m30s and was stopped"},
		{Label: "Registry", Status: core.SourceOK, Count: 3, Duration: 1234 * time.Millisecond},
	}
	a := ScanActivity(core.ScanReport{Status: core.ScanPartial, Sources: src, Items: []core.SoftwareItem{item("x", "", "")}}, nil)
	if a.Outcome != core.OutcomePartial || !strings.Contains(a.Summary, "couldn't read: Winget") {
		t.Errorf("partial: %+v", a)
	}
	if !strings.Contains(a.Details, "Winget: took longer than") || !strings.Contains(a.Details, "Registry: 3 found in 1.23s") {
		t.Errorf("details: %q", a.Details)
	}
	a = ScanActivity(core.ScanReport{Status: core.ScanComplete}, errors.New("disk full"))
	if a.Outcome != core.OutcomeFailed || !strings.Contains(a.Details, "disk full") {
		t.Errorf("save error: %+v", a)
	}
	if SourceLine(core.SourceResult{Label: "X", Status: core.SourceCancelled}) != "X: cancelled" {
		t.Error("empty message should fall back to status")
	}
}
