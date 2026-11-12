package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

func openTemp(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", FileName)
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

// testReport builds a report from test-only items; these never reach a user.
func testReport(started time.Time, names ...string) core.ScanReport {
	r := core.ScanReport{
		StartedAt:  started,
		FinishedAt: started.Add(2 * time.Second),
		Status:     core.ScanComplete,
		Sources: []core.SourceResult{
			{Source: "test-source", Label: "Test source", Status: core.SourceOK, Count: len(names), Duration: 1500 * time.Millisecond},
			{Source: "other", Label: "Other", Status: core.SourceUnavailable, Message: "not on this computer"},
		},
	}
	for i, n := range names {
		kind := core.KindApp
		if strings.HasPrefix(n, "lib") {
			kind = core.KindComponent
		}
		r.Items = append(r.Items, core.SoftwareItem{
			Name: n, Version: "1." + string(rune('0'+i)), Source: "test-source",
			AlsoFoundIn: []string{"a", "b"}, PackageID: "id." + n, Kind: kind, Scope: "user",
		})
	}
	return r
}

func TestOpenMigratesAndIsIdempotent(t *testing.T) {
	db, path := openTemp(t)
	var v int
	if err := db.sql.QueryRow("PRAGMA user_version").Scan(&v); err != nil || v != SchemaVersion() {
		t.Fatalf("user_version = %d (%v), want %d", v, err, SchemaVersion())
	}
	db.Close()
	db2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	db2.Close()
}

func TestOpenRefusesNewerSchema(t *testing.T) {
	db, path := openTemp(t)
	if _, err := db.sql.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(context.Background(), path); err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Errorf("want newer-version error, got %v", err)
	}
}

func TestEmptyDatabase(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	if _, ok, err := db.LatestScan(ctx); ok || err != nil {
		t.Errorf("LatestScan on empty db: ok=%v err=%v", ok, err)
	}
	if items, err := db.Software(ctx); len(items) != 0 || err != nil {
		t.Errorf("Software on empty db: %v %v", items, err)
	}
	if acts, err := db.RecentActivity(ctx, 10); len(acts) != 0 || err != nil {
		t.Errorf("RecentActivity on empty db: %v %v", acts, err)
	}
}

func TestSaveScanRoundTrip(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	start := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	id, err := db.SaveScan(ctx, testReport(start, "zeta", "Alpha", "libthing"))
	if err != nil || id == 0 {
		t.Fatalf("SaveScan: %d %v", id, err)
	}
	sum, ok, err := db.LatestScan(ctx)
	if err != nil || !ok {
		t.Fatalf("LatestScan: %v %v", ok, err)
	}
	if sum.AppCount != 2 || sum.ComponentCount != 1 || sum.Status != core.ScanComplete {
		t.Errorf("summary = %+v", sum)
	}
	if !sum.StartedAt.Equal(start) {
		t.Errorf("started = %v, want %v", sum.StartedAt, start)
	}
	if len(sum.Sources) != 2 || sum.Sources[0].Duration != 1500*time.Millisecond || sum.Sources[1].Status != core.SourceUnavailable {
		t.Errorf("sources = %+v", sum.Sources)
	}

	items, err := db.Software(ctx)
	if err != nil || len(items) != 3 {
		t.Fatalf("Software: %v %v", items, err)
	}
	if items[0].Name != "Alpha" || items[2].Name != "zeta" {
		t.Errorf("want case-insensitive name order, got %q..%q", items[0].Name, items[2].Name)
	}
	if len(items[0].AlsoFoundIn) != 2 || items[0].PackageID != "id.Alpha" || items[0].Scope != "user" {
		t.Errorf("fields not round-tripped: %+v", items[0])
	}
}

func TestSaveScanReplacesPreviousList(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := db.SaveScan(ctx, testReport(now, "one", "two")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SaveScan(ctx, testReport(now.Add(time.Hour), "three")); err != nil {
		t.Fatal(err)
	}
	items, _ := db.Software(ctx)
	if len(items) != 1 || items[0].Name != "three" {
		t.Errorf("latest scan should replace the list, got %+v", items)
	}
	sum, _, _ := db.LatestScan(ctx)
	if sum.AppCount != 1 {
		t.Errorf("latest summary app count = %d", sum.AppCount)
	}
}

func TestClearInventoryKeepsActivity(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	if _, err := db.SaveScan(ctx, testReport(time.Now(), "one")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddActivity(ctx, core.Activity{Kind: "scan", Title: "Scan", Outcome: core.OutcomeSuccess}); err != nil {
		t.Fatal(err)
	}
	if err := db.ClearInventory(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.LatestScan(ctx); ok {
		t.Error("scan summary should be gone")
	}
	if items, _ := db.Software(ctx); len(items) != 0 {
		t.Error("software should be gone")
	}
	if acts, _ := db.RecentActivity(ctx, 10); len(acts) != 1 {
		t.Error("activity must be kept")
	}
}

func TestActivityNewestFirstWithLimit(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		_, err := db.AddActivity(ctx, core.Activity{
			At: base.Add(time.Duration(i) * time.Minute), Kind: "scan",
			Title: "entry " + string(rune('0'+i)), Outcome: core.OutcomeSuccess,
			Summary: "s", Details: "line1\nline2",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	acts, err := db.RecentActivity(ctx, 3)
	if err != nil || len(acts) != 3 {
		t.Fatalf("RecentActivity: %v %v", acts, err)
	}
	if acts[0].Title != "entry 4" || acts[2].Title != "entry 2" {
		t.Errorf("order wrong: %q, %q", acts[0].Title, acts[2].Title)
	}
	if !acts[0].At.Equal(base.Add(4*time.Minute)) || acts[0].Details != "line1\nline2" {
		t.Errorf("fields not round-tripped: %+v", acts[0])
	}
}

func TestDataPersistsAcrossReopen(t *testing.T) {
	db, path := openTemp(t)
	ctx := context.Background()
	if _, err := db.SaveScan(ctx, testReport(time.Now(), "kept")); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if items, _ := db2.Software(ctx); len(items) != 1 {
		t.Errorf("data not persisted: %+v", items)
	}
}

func TestUpdateCheckRoundTripAndRemoval(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	if _, _, ok, err := db.LatestUpdateCheck(ctx); ok || err != nil {
		t.Fatalf("empty db: ok=%v err=%v", ok, err)
	}
	start := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	check := core.UpdateCheck{
		StartedAt: start, FinishedAt: start.Add(time.Second), Status: core.ScanPartial,
		Sources: []core.SourceResult{{Source: "apt", Label: "APT", Status: core.SourceOK, Count: 2}, {Source: "snap", Label: "Snap", Status: core.SourceFailed, Message: "x"}},
		Updates: []core.Update{
			{Manager: "apt", PackageID: "zlib1g", Name: "zlib1g", CurrentVersion: "1", AvailableVersion: "2", Origin: "noble-updates", Scope: "machine", Component: true},
			{Manager: "apt", PackageID: "curl", Name: "curl", AvailableVersion: "8.5", Held: true, Note: "held"},
		},
	}
	if _, err := db.SaveUpdateCheck(ctx, check); err != nil {
		t.Fatal(err)
	}
	sum, ups, ok, err := db.LatestUpdateCheck(ctx)
	if err != nil || !ok || sum.Status != core.ScanPartial || len(sum.Sources) != 2 || len(ups) != 2 {
		t.Fatalf("got %+v %+v %v %v", sum, ups, ok, err)
	}
	if ups[0].Name != "curl" || !ups[0].Held || ups[0].ID == 0 || ups[1].Origin != "noble-updates" || !ups[1].Component || ups[1].Scope != "machine" {
		t.Errorf("updates not round-tripped: %+v", ups)
	}
	if err := db.RemoveUpdates(ctx, []int64{ups[1].ID}); err != nil {
		t.Fatal(err)
	}
	_, ups, _, _ = db.LatestUpdateCheck(ctx)
	if len(ups) != 1 || ups[0].Name != "curl" {
		t.Errorf("removal failed: %+v", ups)
	}

	// A new check replaces the list and prunes old summaries.
	if _, err := db.SaveUpdateCheck(ctx, core.UpdateCheck{StartedAt: start.Add(time.Hour), FinishedAt: start.Add(time.Hour), Status: core.ScanComplete}); err != nil {
		t.Fatal(err)
	}
	sum, ups, _, _ = db.LatestUpdateCheck(ctx)
	if sum.Status != core.ScanComplete || len(ups) != 0 {
		t.Errorf("new check should replace list: %+v %+v", sum, ups)
	}
	var n int
	db.sql.QueryRow("SELECT COUNT(*) FROM update_checks").Scan(&n)
	if n != 1 {
		t.Errorf("old checks should be pruned, have %d", n)
	}
}

func TestMigratesVersion1DatabaseForward(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a Phase 2 database: drop v2+ tables and reset the version.
	for _, q := range []string{"DROP TABLE analyses", "DROP TABLE updates", "DROP TABLE update_check_sources", "DROP TABLE update_checks", "PRAGMA user_version = 1"} {
		if _, err := db.sql.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.SaveScan(context.Background(), testReport(time.Now(), "kept")); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("migrate v1 -> v%d: %v", SchemaVersion(), err)
	}
	defer db2.Close()
	if items, _ := db2.Software(context.Background()); len(items) != 1 {
		t.Error("existing data must survive the migration")
	}
	if _, _, _, err := db2.LatestUpdateCheck(context.Background()); err != nil {
		t.Errorf("v2 tables missing after migration: %v", err)
	}
}

func TestAnalysisRoundTripAndClearedWithInventory(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	if _, ok, err := db.LatestAnalysis(ctx); ok || err != nil {
		t.Fatalf("empty: %v %v", ok, err)
	}
	r := core.AnalysisResult{
		StartedAt: time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 11, 12, 9, 0, 1, 0, time.UTC),
		Status:   core.ScanComplete,
		Findings: []core.Finding{{Key: "k", Severity: core.SeverityAttention, Title: "T", Items: []core.ItemRef{{Name: "a"}}}},
		Runtimes: []core.Runtime{{Family: "Python", Branch: "3.12", Dependents: []string{"x", "y"}, DepsKnown: true}},
		Checks:   []core.CheckSummary{{Name: "c", Found: 1, Available: true}},
	}
	if err := db.SaveAnalysis(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveAnalysis(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.LatestAnalysis(ctx)
	if err != nil || !ok || got.Findings[0].Title != "T" || len(got.Runtimes[0].Dependents) != 2 || !got.FinishedAt.Equal(r.FinishedAt) {
		t.Fatalf("round trip: %+v %v %v", got, ok, err)
	}
	var n int
	db.sql.QueryRow("SELECT COUNT(*) FROM analyses").Scan(&n)
	if n != 1 {
		t.Errorf("only the latest analysis is kept, have %d", n)
	}
	if err := db.ClearInventory(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.LatestAnalysis(ctx); ok {
		t.Error("clearing the software list must clear the analysis based on it")
	}
}

func TestDeleteSoftware(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()
	if _, err := db.SaveScan(ctx, testReport(time.Now(), "keep", "gone")); err != nil {
		t.Fatal(err)
	}
	items, _ := db.Software(ctx)
	var gone core.SoftwareItem
	for _, it := range items {
		if it.Name == "gone" {
			gone = it
		}
	}
	if err := db.DeleteSoftware(ctx, []core.ItemRef{{Name: gone.Name, Version: gone.Version, Source: gone.Source}, {Name: "keep", Version: "wrong", Source: gone.Source}}); err != nil {
		t.Fatal(err)
	}
	items, _ = db.Software(ctx)
	if len(items) != 1 || items[0].Name != "keep" {
		t.Errorf("only the exact match should go: %+v", items)
	}
}
