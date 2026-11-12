package core

import (
	"context"
	"errors"
	"time"
)

// Kind separates things a person installed on purpose from the supporting
// pieces that came along with them.
type Kind string

const (
	// KindApp is an application or tool the user is likely to recognise.
	KindApp Kind = "app"
	// KindComponent is a library, runtime, redistributable or system part.
	KindComponent Kind = "component"
)

// SoftwareItem describes one installed application, tool or component as
// found by one or more scanners.
type SoftwareItem struct {
	Name      string
	Version   string
	Publisher string
	// Source is the ID of the scanner that found it, e.g. "windows-registry".
	Source string
	// AlsoFoundIn lists other scanner IDs that reported the same item.
	AlsoFoundIn []string
	// PackageID is the package manager's identifier when known, e.g. "Git.Git".
	PackageID       string
	InstallLocation string
	// InstallDate is YYYY-MM-DD when known.
	InstallDate string
	Kind        Kind
	// Scope is "machine" or "user" when known.
	Scope       string
	Description string
}

// ErrSourceUnavailable is returned by a Scanner whose source does not exist
// on this computer (for example, winget is not installed). It is not an error
// from the user's point of view.
var ErrSourceUnavailable = errors.New("source not available on this computer")

// Scanner discovers installed software without modifying the system.
// Implementations live in internal/inventory.
type Scanner interface {
	// ID is a stable identifier such as "windows-registry" or "dpkg".
	ID() string
	// Label is the plain-language name shown to users.
	Label() string
	// Detail says technically what the scanner reads or runs (Advanced mode).
	Detail() string
	// Scan returns everything the scanner can see. It must be read-only and
	// return ErrSourceUnavailable when the source does not exist here.
	Scan(ctx context.Context) ([]SoftwareItem, error)
}

// SourceInfo is the static description of a scanner.
type SourceInfo struct {
	ID, Label, Detail string
}

// SourceStatus is the outcome of one scanner.
type SourceStatus string

const (
	SourceOK          SourceStatus = "ok"
	SourceUnavailable SourceStatus = "unavailable"
	SourceFailed      SourceStatus = "failed"
	SourceTimeout     SourceStatus = "timeout"
	SourceCancelled   SourceStatus = "cancelled"
)

// SourceResult reports how one scanner did.
type SourceResult struct {
	Source   string
	Label    string
	Status   SourceStatus
	Count    int
	Duration time.Duration
	// Message is a short plain-language explanation for non-OK results.
	Message string
	// Details is technical error text for Advanced mode.
	Details string
}

// SourceEvent is sent while a scan runs. Result is nil when the scanner has
// just started.
type SourceEvent struct {
	Source string
	Result *SourceResult
}

// ScanStatus is the overall outcome of a scan.
type ScanStatus string

const (
	// ScanComplete: every source that exists here was read.
	ScanComplete ScanStatus = "complete"
	// ScanPartial: at least one source was read and at least one failed.
	ScanPartial ScanStatus = "partial"
	// ScanFailed: no source could be read.
	ScanFailed ScanStatus = "failed"
	// ScanNothingToScan: none of the supported sources exist here.
	ScanNothingToScan ScanStatus = "nothing"
	// ScanCancelled: the user stopped the scan.
	ScanCancelled ScanStatus = "cancelled"
)

// Saved reports whether a scan with this status replaces the saved list.
func (s ScanStatus) Saved() bool { return s == ScanComplete || s == ScanPartial }

// ScanReport is the full result of running all scanners.
type ScanReport struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Status     ScanStatus
	Sources    []SourceResult
	Items      []SoftwareItem
}

// Counts returns the number of apps and components in the report.
func (r ScanReport) Counts() (apps, components int) {
	return CountKinds(r.Items)
}

// CountKinds counts apps and components.
func CountKinds(items []SoftwareItem) (apps, components int) {
	for _, it := range items {
		if it.Kind == KindComponent {
			components++
		} else {
			apps++
		}
	}
	return apps, components
}

// ScanSummary is what the store remembers about the latest saved scan.
type ScanSummary struct {
	ID             int64
	StartedAt      time.Time
	FinishedAt     time.Time
	Status         ScanStatus
	AppCount       int
	ComponentCount int
	Sources        []SourceResult
}

// Outcome classifies an activity record.
type Outcome string

const (
	OutcomeSuccess   Outcome = "success"
	OutcomePartial   Outcome = "partial"
	OutcomeFailed    Outcome = "failed"
	OutcomeCancelled Outcome = "cancelled"
	OutcomeInfo      Outcome = "info"
)

// Activity is one entry in the Activity History.
type Activity struct {
	ID      int64
	At      time.Time
	Kind    string // e.g. "scan", "inventory.clear"
	Title   string
	Outcome Outcome
	// Summary is plain language for everyone.
	Summary string
	// Details is technical text for Advanced mode, one item per line.
	Details string
}

// Store persists scans and activity. Implemented by internal/database.
type Store interface {
	SaveScan(ctx context.Context, r ScanReport) (int64, error)
	LatestScan(ctx context.Context) (ScanSummary, bool, error)
	Software(ctx context.Context) ([]SoftwareItem, error)
	ClearInventory(ctx context.Context) error
	AddActivity(ctx context.Context, a Activity) (int64, error)
	RecentActivity(ctx context.Context, limit int) ([]Activity, error)
}

// InventoryService is the port the TUI uses for everything inventory
// related. Implemented by internal/inventory.Service.
type InventoryService interface {
	Sources() []SourceInfo
	// Scan runs every scanner, saves the result when appropriate, records
	// the activity, and returns the report. The error is non-nil only when
	// saving failed; scanner problems are described in the report.
	Scan(ctx context.Context, progress func(SourceEvent)) (ScanReport, error)
	// ClearInventory deletes ATLAS's saved software list and records it.
	ClearInventory(ctx context.Context) error
	LatestScan(ctx context.Context) (ScanSummary, bool, error)
	Software(ctx context.Context) ([]SoftwareItem, error)
	RecentActivity(ctx context.Context, limit int) ([]Activity, error)
}

// PackageManager is the port for a package manager adapter (winget, scoop,
// chocolatey, apt, snap, flatpak, ...). Defined for Phase 3; no adapter
// exists yet and nothing calls it.
type PackageManager interface {
	ID() string
	DisplayName() string
	Available(ctx context.Context) (bool, error)
	PlanUpdateAll(ctx context.Context) (ActionPlan, error)
}
