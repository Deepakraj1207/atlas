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
