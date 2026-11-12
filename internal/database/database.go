// Package database is the SQLite adapter for core.Store. It uses the
// pure-Go modernc.org/sqlite driver so ATLAS cross-compiles for Windows
// with CGO disabled.
//
// The database holds only ATLAS's own records: the latest software list,
// scan summaries and the activity history. It never stores anything outside
// its own file.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/deepakraj1207/atlas/internal/core"
)

// FileName is the database file name inside the data directory.
const FileName = "atlas.db"

// DB implements core.Store.
type DB struct {
	sql  *sql.DB
	path string
}

var _ core.Store = (*DB)(nil)

// Open opens (creating if needed) the database at path and applies any
// pending migrations.
func Open(ctx context.Context, path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data folder: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite allows one writer; a single connection avoids lock contention
	// and keeps pragmas consistent.
	sqlDB.SetMaxOpenConns(1)
	db := &DB{sql: sqlDB, path: path}
	if err := db.migrate(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Path returns the database file path.
func (d *DB) Path() string { return d.path }

// Close closes the database.
func (d *DB) Close() error { return d.sql.Close() }

// migrations are applied in order; the index+1 is the schema version.
var migrations = []string{
	`CREATE TABLE scans (
		id              INTEGER PRIMARY KEY,
		started_at      TEXT    NOT NULL,
		finished_at     TEXT    NOT NULL,
		status          TEXT    NOT NULL,
		app_count       INTEGER NOT NULL,
		component_count INTEGER NOT NULL
	);
	CREATE TABLE scan_sources (
		scan_id     INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
		source      TEXT    NOT NULL,
		label       TEXT    NOT NULL,
		status      TEXT    NOT NULL,
		item_count  INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL,
		message     TEXT    NOT NULL DEFAULT '',
		details     TEXT    NOT NULL DEFAULT ''
	);
	CREATE INDEX scan_sources_scan ON scan_sources(scan_id);
	CREATE TABLE software (
		id               INTEGER PRIMARY KEY,
		scan_id          INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
		name             TEXT    NOT NULL,
		version          TEXT    NOT NULL DEFAULT '',
		publisher        TEXT    NOT NULL DEFAULT '',
		source           TEXT    NOT NULL,
		also_found_in    TEXT    NOT NULL DEFAULT '',
		package_id       TEXT    NOT NULL DEFAULT '',
		install_location TEXT    NOT NULL DEFAULT '',
		install_date     TEXT    NOT NULL DEFAULT '',
		kind             TEXT    NOT NULL,
		scope            TEXT    NOT NULL DEFAULT '',
		description      TEXT    NOT NULL DEFAULT ''
	);
	CREATE INDEX software_scan ON software(scan_id);
	CREATE TABLE activity (
		id      INTEGER PRIMARY KEY,
		at      TEXT NOT NULL,
		kind    TEXT NOT NULL,
		title   TEXT NOT NULL,
		outcome TEXT NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		details TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX activity_at ON activity(at);`,
}

// SchemaVersion is the version this build migrates to.
func SchemaVersion() int { return len(migrations) }

func (d *DB) migrate(ctx context.Context) error {
	var current int
	if err := d.sql.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > len(migrations) {
		return fmt.Errorf("the database was created by a newer version of ATLAS (schema %d, this build understands %d)", current, len(migrations))
	}
	for v := current; v < len(migrations); v++ {
		tx, err := d.sql.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("migrate to schema %d: %w", v+1, err)
		}
		if _, err := tx.ExecContext(ctx, migrations[v]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate to schema %d: %w", v+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", v+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate to schema %d: %w", v+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate to schema %d: %w", v+1, err)
		}
	}
	return nil
}

const timeFormat = time.RFC3339Nano

func fmtTime(t time.Time) string { return t.UTC().Format(timeFormat) }

func parseTime(s string) time.Time {
	t, err := time.Parse(timeFormat, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SaveScan stores a scan and replaces the saved software list with its items.
// Only the latest scan's software is kept; older scan summaries remain.
func (d *DB) SaveScan(ctx context.Context, r core.ScanReport) (int64, error) {
	apps, comps := r.Counts()
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO scans (started_at, finished_at, status, app_count, component_count) VALUES (?, ?, ?, ?, ?)`,
		fmtTime(r.StartedAt), fmtTime(r.FinishedAt), string(r.Status), apps, comps)
	if err != nil {
		return 0, fmt.Errorf("save scan: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("save scan: %w", err)
	}

	for _, s := range r.Sources {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO scan_sources (scan_id, source, label, status, item_count, duration_ms, message, details) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, s.Source, s.Label, string(s.Status), s.Count, s.Duration.Milliseconds(), s.Message, s.Details); err != nil {
			return 0, fmt.Errorf("save scan sources: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM software`); err != nil {
		return 0, fmt.Errorf("replace software list: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO software (scan_id, name, version, publisher, source, also_found_in, package_id, install_location, install_date, kind, scope, description)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("save software: %w", err)
	}
	defer stmt.Close()
	for _, it := range r.Items {
		kind := it.Kind
		if kind == "" {
			kind = core.KindApp
		}
		if _, err := stmt.ExecContext(ctx, id, it.Name, it.Version, it.Publisher, it.Source,
			strings.Join(it.AlsoFoundIn, ","), it.PackageID, it.InstallLocation, it.InstallDate,
			string(kind), it.Scope, it.Description); err != nil {
			return 0, fmt.Errorf("save software: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("save scan: %w", err)
	}
	return id, nil
}

// LatestScan returns the most recent saved scan. ok is false if none exists.
func (d *DB) LatestScan(ctx context.Context) (core.ScanSummary, bool, error) {
	var s core.ScanSummary
	var started, finished, status string
	err := d.sql.QueryRowContext(ctx,
		`SELECT id, started_at, finished_at, status, app_count, component_count FROM scans ORDER BY id DESC LIMIT 1`).
		Scan(&s.ID, &started, &finished, &status, &s.AppCount, &s.ComponentCount)
	if errors.Is(err, sql.ErrNoRows) {
		return core.ScanSummary{}, false, nil
	}
	if err != nil {
		return core.ScanSummary{}, false, fmt.Errorf("read latest scan: %w", err)
	}
	s.StartedAt, s.FinishedAt, s.Status = parseTime(started), parseTime(finished), core.ScanStatus(status)

	rows, err := d.sql.QueryContext(ctx,
		`SELECT source, label, status, item_count, duration_ms, message, details FROM scan_sources WHERE scan_id = ? ORDER BY rowid`, s.ID)
	if err != nil {
		return core.ScanSummary{}, false, fmt.Errorf("read scan sources: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r core.SourceResult
		var st string
		var ms int64
		if err := rows.Scan(&r.Source, &r.Label, &st, &r.Count, &ms, &r.Message, &r.Details); err != nil {
			return core.ScanSummary{}, false, fmt.Errorf("read scan sources: %w", err)
		}
		r.Status = core.SourceStatus(st)
		r.Duration = time.Duration(ms) * time.Millisecond
		s.Sources = append(s.Sources, r)
	}
	return s, true, rows.Err()
}

// Software returns the saved software list ordered by name.
func (d *DB) Software(ctx context.Context) ([]core.SoftwareItem, error) {
	rows, err := d.sql.QueryContext(ctx,
		`SELECT name, version, publisher, source, also_found_in, package_id, install_location, install_date, kind, scope, description
		 FROM software ORDER BY name COLLATE NOCASE, version`)
	if err != nil {
		return nil, fmt.Errorf("read software list: %w", err)
	}
	defer rows.Close()
	var out []core.SoftwareItem
	for rows.Next() {
		var it core.SoftwareItem
		var also, kind string
		if err := rows.Scan(&it.Name, &it.Version, &it.Publisher, &it.Source, &also, &it.PackageID,
			&it.InstallLocation, &it.InstallDate, &kind, &it.Scope, &it.Description); err != nil {
			return nil, fmt.Errorf("read software list: %w", err)
		}
		if also != "" {
			it.AlsoFoundIn = strings.Split(also, ",")
		}
		it.Kind = core.Kind(kind)
		out = append(out, it)
	}
	return out, rows.Err()
}

// ClearInventory deletes the saved software list and all scan summaries.
// Activity history is kept.
func (d *DB) ClearInventory(ctx context.Context) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{`DELETE FROM software`, `DELETE FROM scan_sources`, `DELETE FROM scans`} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("clear saved list: %w", err)
		}
	}
	return tx.Commit()
}

// AddActivity records an activity entry.
func (d *DB) AddActivity(ctx context.Context, a core.Activity) (int64, error) {
	if a.At.IsZero() {
		a.At = time.Now()
	}
	res, err := d.sql.ExecContext(ctx,
		`INSERT INTO activity (at, kind, title, outcome, summary, details) VALUES (?, ?, ?, ?, ?, ?)`,
		fmtTime(a.At), a.Kind, a.Title, string(a.Outcome), a.Summary, a.Details)
	if err != nil {
		return 0, fmt.Errorf("record activity: %w", err)
	}
	return res.LastInsertId()
}

// RecentActivity returns up to limit entries, newest first.
func (d *DB) RecentActivity(ctx context.Context, limit int) ([]core.Activity, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.sql.QueryContext(ctx,
		`SELECT id, at, kind, title, outcome, summary, details FROM activity ORDER BY at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read activity: %w", err)
	}
	defer rows.Close()
	var out []core.Activity
	for rows.Next() {
		var a core.Activity
		var at, outcome string
		if err := rows.Scan(&a.ID, &at, &a.Kind, &a.Title, &outcome, &a.Summary, &a.Details); err != nil {
			return nil, fmt.Errorf("read activity: %w", err)
		}
		a.At = parseTime(at)
		a.Outcome = core.Outcome(outcome)
		out = append(out, a)
	}
	return out, rows.Err()
}
