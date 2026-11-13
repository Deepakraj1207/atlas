// Package screens contains one screen per navigation section. Screens are
// small Bubble Tea sub-models that receive a Context from the root model and
// talk back through the messages in msgs.go.
package screens

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/layout"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Context is everything a screen needs from the root model to render.
type Context struct {
	Theme  theme.Theme
	Layout layout.Layout
	Config config.Config
	Now    time.Time

	// Paths and diagnostics, shown in Advanced mode.
	ConfigPath   string
	ConfigResult config.Result
	LogPath      string
	// LogError is non-empty when the log file could not be opened.
	LogError string
	DBPath   string
	Version  string
	Platform string

	// Sources are the scanners available on this platform, in priority order.
	Sources []core.SourceInfo
	// Managers are the package managers ATLAS can check for updates.
	Managers []core.SourceInfo
	// Elevated is true when ATLAS runs as administrator or root.
	Elevated bool
	// AnalysisSources are the extra data sources Smart Analysis reads.
	AnalysisSources []core.SourceInfo
	// Tidiers are the places "look for things to tidy" examines.
	Tidiers []core.SourceInfo
	Data    DataView
}

// DataView is the root model's read-only snapshot of saved data.
type DataView struct {
	// Available is false when the database could not be opened.
	Available bool
	StoreErr  string

	Loaded  bool
	LoadErr string
	// Summary is nil until a scan has been saved.
	Summary  *core.ScanSummary
	Items    []core.SoftwareItem
	Activity []core.Activity

	// Scan is non-nil while a scan is running.
	Scan *ScanProgress

	// UpdatesAvailable is false when update checking is unavailable.
	UpdatesAvailable bool
	// UpdateSummary is nil until an update check has been saved.
	UpdateSummary *core.UpdateCheckSummary
	// Updates are the pending updates from the latest check.
	Updates []core.Update
	// Check is non-nil while an update check is running.
	Check *ScanProgress
	// UpdatesSinceScan is true when updates were installed after the latest scan.
	UpdatesSinceScan bool
	// FailedCheck is this session's most recent update check if it could
	// not produce results (failed or nothing to check). It isn't saved, so
	// it is kept here to explain what went wrong.
	FailedCheck *core.UpdateCheck

	// AnalysisAvailable is false when analysis is unavailable.
	AnalysisAvailable bool
	// Analysis is nil until an analysis has been saved.
	Analysis *core.AnalysisResult
	// Analyze is non-nil while an analysis is running.
	Analyze *ScanProgress

	// CleanupAvailable is false when cleanup is unavailable.
	CleanupAvailable bool
	// Tidy is this session's latest search for things to tidy (nil if none).
	Tidy *core.TidyReport
	// Tidying is non-nil while that search runs.
	Tidying *ScanProgress
	// RemovedSinceAnalysis is true when software was removed after the
	// latest analysis.
	RemovedSinceAnalysis bool

	// DiscoverAvailable is false when Discover Apps is unavailable.
	DiscoverAvailable bool
	// Discovery is the catalogue assessed for this computer (nil until loaded).
	Discovery *core.Discovery
	// InstalledNow lists catalogue IDs installed this session, before a rescan.
	InstalledNow map[string]bool
}

// Busy reports whether a background task is running.
func (d DataView) Busy() bool {
	return d.Scan != nil || d.Check != nil || d.Analyze != nil || d.Tidying != nil
}

// AnalysisStale reports whether the software list changed after the
// saved analysis was made (a newer scan, or software removed since).
func (d DataView) AnalysisStale() bool {
	if d.Analysis == nil {
		return false
	}
	if d.Summary != nil && d.Analysis.BasedOnScanID != 0 && d.Summary.ID != d.Analysis.BasedOnScanID {
		return true
	}
	if d.Summary != nil && d.Analysis.BasedOnScanID == 0 && d.Summary.FinishedAt.After(d.Analysis.BasedOnScan) {
		return true // analyses saved before scan IDs were recorded
	}
	return d.RemovedSinceAnalysis
}

// PendingUpdates counts updates that "update all" would include.
func (d DataView) PendingUpdates() (apps, components int) {
	for _, u := range d.Updates {
		switch {
		case u.Held:
		case u.Component:
			components++
		default:
			apps++
		}
	}
	return apps, components
}

// ScanProgress is the live state of a running scan.
type ScanProgress struct {
	Started    time.Time
	Sources    []SourceProgress
	Spinner    string
	Cancelling bool
}

// SourceProgress tracks one source during a scan.
type SourceProgress struct {
	Info    core.SourceInfo
	Running bool
	Result  *core.SourceResult
}

// Finished counts sources that have reported a result.
func (p ScanProgress) Finished() int {
	n := 0
	for _, s := range p.Sources {
		if s.Result != nil {
			n++
		}
	}
	return n
}

// Advanced reports whether the UI is in Advanced mode.
func (c Context) Advanced() bool { return c.Config.UI.Mode == config.ModeAdvanced }

// Width is the content width available to the screen.
func (c Context) Width() int { return c.Layout.ContentWidth }

// Height is the content height available to the screen.
func (c Context) Height() int { return c.Layout.ContentHeight }

// SourceLabel returns the plain-language name for a source ID.
func (c Context) SourceLabel(id string) string {
	for _, s := range c.Sources {
		if s.ID == id {
			return s.Label
		}
	}
	return id
}

// ManagerLabel returns the plain-language name for a package manager ID.
func (c Context) ManagerLabel(id string) string {
	for _, m := range c.Managers {
		if m.ID == id {
			return m.Label
		}
	}
	return id
}

// Screen is the interface every section screen implements.
type Screen interface {
	// Update handles a message. It returns the (possibly updated) screen
	// and any command for the root model.
	Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd)
	// View renders the screen within ctx.Width(). Screens that do not
	// implement SelfScrolling may return more lines than ctx.Height(); the
	// root scrolls them.
	View(ctx Context) string
	// Hints lists the screen-specific keys for the footer and help overlay.
	Hints(ctx Context) []components.KeyHint
}

// SelfScrolling screens fit their own output to ctx.Height() and receive
// PgUp/PgDn themselves.
type SelfScrolling interface {
	SelfScrolling() bool
}

// InputCapturer screens receive every key (except ctrl+c) while
// CapturesInput is true, for example while the user types a search.
type InputCapturer interface {
	CapturesInput() bool
}
