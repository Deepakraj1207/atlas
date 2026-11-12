package core

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
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

// SourceError lets a scanner or package manager fail with a plain-language
// explanation instead of a generic "couldn't be read".
type SourceError struct {
	// Message is shown to everyone, e.g. "No package information downloaded yet".
	Message string
	// Details is technical, for Advanced mode.
	Details string
}

func (e *SourceError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}

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

// Store persists scans, update checks and activity. Implemented by
// internal/database.
type Store interface {
	SaveScan(ctx context.Context, r ScanReport) (int64, error)
	LatestScan(ctx context.Context) (ScanSummary, bool, error)
	Software(ctx context.Context) ([]SoftwareItem, error)
	ClearInventory(ctx context.Context) error
	// DeleteSoftware drops removed items from the saved list, matched by
	// source, name and version.
	DeleteSoftware(ctx context.Context, items []ItemRef) error

	SaveUpdateCheck(ctx context.Context, c UpdateCheck) (int64, error)
	// LatestUpdateCheck returns the latest check and its remaining updates.
	LatestUpdateCheck(ctx context.Context) (UpdateCheckSummary, []Update, bool, error)
	// RemoveUpdates drops updates that have been applied.
	RemoveUpdates(ctx context.Context, ids []int64) error

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

// Update is one newer version offered by a package manager.
type Update struct {
	// ID is assigned by the store; zero until saved.
	ID int64
	// Manager is the package manager's source ID, e.g. "winget" or "apt".
	Manager   string
	PackageID string
	Name      string
	// CurrentVersion may be empty when the manager does not report it and
	// the saved software list has no match.
	CurrentVersion   string
	AvailableVersion string
	// Origin is the repository or source, e.g. "winget", "noble-updates".
	Origin string
	// Held is true when the user or the manager has pinned the package.
	// "Update all" never includes held packages.
	Held bool
	// Note is a short plain-language remark, e.g. why it is held.
	Note string
	// Scope is "machine" or "user" when it matters for applying.
	Scope string
	// Component is true for libraries, runtimes and other supporting parts
	// (from the saved software list). They are updated with everything else
	// but listed separately.
	Component bool
}

// UpdateCheck is the result of asking every package manager for updates.
type UpdateCheck struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Status     ScanStatus
	Sources    []SourceResult
	Updates    []Update
}

// UpdateCheckSummary is what the store remembers about the latest check.
type UpdateCheckSummary struct {
	ID         int64
	StartedAt  time.Time
	FinishedAt time.Time
	Status     ScanStatus
	Sources    []SourceResult
}

// Elevation describes how an update step gets the rights it needs.
type Elevation string

const (
	// ElevationNone: runs with the user's normal rights.
	ElevationNone Elevation = "none"
	// ElevationSudo: the command runs through sudo, which asks for the
	// user's password in the terminal.
	ElevationSudo Elevation = "sudo"
	// ElevationInstaller: an installer may ask Windows for permission
	// (a User Account Control prompt) while it runs.
	ElevationInstaller Elevation = "installer"
	// ElevationPolkit: the package manager may ask the system for the
	// user's password through PolicyKit.
	ElevationPolkit Elevation = "polkit"
	// ElevationAdmin: ATLAS is already running with administrator rights.
	ElevationAdmin Elevation = "admin"
)

// Command is one program invocation, shown to the user before it runs.
type Command struct {
	Program string
	Args    []string
	// OKCodes are exit codes that mean success. Empty means just 0.
	OKCodes []int
	// RestartCodes are exit codes that mean success but a restart is needed.
	RestartCodes []int
}

// String renders the command the way a user would type it.
func (c Command) String() string {
	parts := []string{c.Program}
	for _, a := range c.Args {
		if a == "" || strings.ContainsAny(a, " \t\"'") {
			a = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// Interpret classifies an exit code.
func (c Command) Interpret(code int) (ok, restart bool) {
	if slices.Contains(c.RestartCodes, code) {
		return true, true
	}
	if len(c.OKCodes) == 0 {
		return code == 0, false
	}
	return slices.Contains(c.OKCodes, code), false
}

// Step actions.
const (
	ActionUpdate = ""       // install newer versions (the default)
	ActionRemove = "remove" // uninstall software
	ActionClean  = "clean"  // delete caches or leftovers
)

// UpdateStep is one unit of work run in the user's terminal: the commands
// one package manager needs to update, remove or clean up something.
// Nothing in a step runs until the user confirms the preview that lists it.
// For removals and clean-ups, Updates holds the targets (CurrentVersion is
// the installed version; AvailableVersion is empty).
type UpdateStep struct {
	Manager string
	Label   string
	// Action is ActionUpdate, ActionRemove or ActionClean.
	Action   string
	Updates  []Update
	Commands []Command
	// Bytes is the space a clean-up frees, when known (0 = unknown).
	Bytes int64
	Elevation
	// Blocked explains why this step can't run from here. Blocked steps are
	// shown in the preview and skipped.
	Blocked string
	// Rollback says, in plain language, how to go back if needed.
	Rollback string
}

// StepStatus is the outcome of one step.
type StepStatus string

const (
	StepDone    StepStatus = "done"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
	StepStopped StepStatus = "stopped"
)

// StepResult reports what happened to one step.
type StepResult struct {
	Step     UpdateStep
	Status   StepStatus
	ExitCode int
	Duration time.Duration
	// Command is the command that finished last (or failed).
	Command string
	// Note is plain language, e.g. "Restart needed" or why it was skipped.
	Note string
	// Restart is true when the package manager asked for a restart.
	Restart bool
}

// TerminalJob runs update steps attached to the user's terminal so the
// package managers' own output and prompts (passwords, permission dialogs,
// confirmations) are shown directly. Its method set matches Bubble Tea's
// ExecCommand so the TUI can hand the terminal over and take it back.
type TerminalJob interface {
	Run() error
	SetStdin(io.Reader)
	SetStdout(io.Writer)
	SetStderr(io.Writer)
	Results() []StepResult
}

// PackageManager is the port for a package manager adapter.
type PackageManager interface {
	ID() string
	Label() string
	// Detail says technically how updates are checked (Advanced mode).
	Detail() string
	// CheckUpdates lists available updates. It must be read-only and return
	// ErrSourceUnavailable when the manager is not installed.
	CheckUpdates(ctx context.Context) ([]Update, error)
	// PlanUpdates returns the steps that would apply these updates (all from
	// this manager). It runs nothing.
	PlanUpdates(updates []Update) []UpdateStep
}

// UpdateService is the port the TUI uses for updates. Implemented by
// internal/updates.Service.
type UpdateService interface {
	Managers() []SourceInfo
	// Check asks every manager for updates, saves the result when
	// appropriate, records the activity and returns it. The error is
	// non-nil only when saving failed.
	Check(ctx context.Context, progress func(SourceEvent)) (UpdateCheck, error)
	LatestCheck(ctx context.Context) (UpdateCheckSummary, []Update, bool, error)
	// Plan groups updates into steps. Held updates are left out.
	Plan(updates []Update) []UpdateStep
	// NewJob prepares a terminal job for the steps. It runs nothing yet.
	NewJob(steps []UpdateStep) TerminalJob
	// RecordResults records the outcome in Activity History and removes
	// applied updates from the saved list.
	RecordResults(ctx context.Context, results []StepResult) error
}

// Severity ranks a finding.
type Severity string

const (
	// SeverityAttention: worth acting on soon, e.g. software no longer
	// receiving security fixes.
	SeverityAttention Severity = "attention"
	// SeverityInfo: worth knowing; nothing urgent.
	SeverityInfo Severity = "info"
)

// ItemRef points at a software item a finding is about.
type ItemRef struct {
	Name, Version, Source, PackageID string
	// Note is item-specific context, e.g. "last opened 3 Mar 2024".
	Note string
}

// Finding is one plain-language observation from Smart Analysis.
type Finding struct {
	// Key is stable across runs, e.g. "eol:python:3.8".
	Key      string
	Kind     string
	Severity Severity
	Title    string
	// Summary says what was found.
	Summary string
	// Why says why it matters.
	Why string
	// Suggestion says what the user can do now. Never an action ATLAS
	// takes on its own.
	Suggestion string
	Items      []ItemRef
	// Details are technical lines for Advanced mode.
	Details []string
	// Reference names where the underlying facts come from.
	Reference string
}

// Support describes a runtime version's support status.
type Support struct {
	// State is "supported", "ending", "ended" or "unknown".
	State string
	// EndDate is YYYY-MM-DD or YYYY-MM when known.
	EndDate   string
	Reference string
	// DistroManaged is true for distribution packages, whose maintainers may
	// keep fixing them after the upstream project stops.
	DistroManaged bool
	Note          string
}

// Runtime is an installed runtime other software may need.
type Runtime struct {
	// Family is "Python", "Node.js", ".NET", "Java" or "Visual C++".
	Family string
	// Branch is the support line, e.g. "3.12", "20", "8.0", "2015-2022 x64".
	Branch    string
	Name      string
	Version   string
	Source    string
	PackageID string
	Support   Support
	// DepsKnown is false when the source records no dependency data.
	DepsKnown bool
	// DepsSource names where dependency data came from.
	DepsSource string
	// Dependents are installed packages that directly need this runtime.
	Dependents []string
	// Indirect counts packages that need it through something else.
	Indirect int
}

// CheckSummary reports one analysis check, including "found nothing" and
// "couldn't check", so the user knows what was and wasn't looked at.
type CheckSummary struct {
	Name  string
	Found int
	// Note explains a check that couldn't run, or its limits.
	Note      string
	Available bool
}

// AnalysisResult is the outcome of Analyze My Software.
type AnalysisResult struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Status     ScanStatus
	// Sources are the extra data sources read (dependency data, usage records).
	Sources  []SourceResult
	Findings []Finding
	Runtimes []Runtime
	Checks   []CheckSummary
	// BasedOnScan is when the analysed software list was scanned.
	BasedOnScan time.Time
	// SupportDataAsOf is the date of ATLAS's built-in support table.
	SupportDataAsOf string
}

// Counts returns how many findings need attention and how many are info.
func (r AnalysisResult) Counts() (attention, info int) {
	for _, f := range r.Findings {
		if f.Severity == SeverityAttention {
			attention++
		} else {
			info++
		}
	}
	return attention, info
}

// AnalysisStore persists the latest analysis. Implemented by
// internal/database.
type AnalysisStore interface {
	SaveAnalysis(ctx context.Context, r AnalysisResult) error
	LatestAnalysis(ctx context.Context) (AnalysisResult, bool, error)
}

// AnalysisService is the port the TUI uses for Smart Analysis and
// Dependencies. Implemented by internal/analysis.Service.
type AnalysisService interface {
	Sources() []SourceInfo
	// Analyze reads extra data, applies the rules to the saved software
	// list, saves and records the result. The error is non-nil only when
	// there is no saved software list or saving failed.
	Analyze(ctx context.Context, progress func(SourceEvent)) (AnalysisResult, error)
	Latest(ctx context.Context) (AnalysisResult, bool, error)
}

// TidyReport is the result of looking for caches and leftovers to clean.
type TidyReport struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Status     ScanStatus
	Sources    []SourceResult
	// Steps are clean-up steps with exact targets. Nothing has run.
	Steps []UpdateStep
}

// Bytes is the total known space the report's steps would free.
func (r TidyReport) Bytes() int64 {
	var n int64
	for _, s := range r.Steps {
		n += s.Bytes
	}
	return n
}

// CleanupService is the port the TUI uses for Cleanup. Implemented by
// internal/cleanup.Service.
type CleanupService interface {
	// Tidiers lists what "look for things to tidy" examines.
	Tidiers() []SourceInfo
	// FindTidyUps looks for caches and leftovers. Read-only.
	FindTidyUps(ctx context.Context, progress func(SourceEvent)) (TidyReport, error)
	// PlanRemoval re-reads dependency records and returns removal steps.
	// Items that other installed software needs, that the system relies on,
	// or whose users can't be determined come back as blocked steps with
	// the reason. It runs nothing.
	PlanRemoval(ctx context.Context, items []SoftwareItem) ([]UpdateStep, error)
	NewJob(steps []UpdateStep) TerminalJob
	// RecordResults records the outcome and drops removed software from
	// the saved list.
	RecordResults(ctx context.Context, results []StepResult) error
}

// ActionInstall installs new software (a step action).
const ActionInstall = "install"

// CatalogPackage says how one package manager installs a catalogue entry.
type CatalogPackage struct {
	// Manager is "winget", "scoop", "chocolatey", "apt", "dnf", "snap" or "flatpak".
	Manager string `yaml:"manager"`
	ID      string `yaml:"id"`
	// Classic marks snaps that need classic confinement (full system access).
	Classic bool `yaml:"classic,omitempty"`
	// Note is a plain remark, e.g. "the command is batcat on Debian/Ubuntu".
	Note string `yaml:"note,omitempty"`
}

// CatalogEntry is one curated tool in Discover Apps.
type CatalogEntry struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	// Summary says what it is in one plain sentence.
	Summary string `yaml:"summary"`
	// Why says who it helps.
	Why      string           `yaml:"why"`
	Homepage string           `yaml:"homepage"`
	Packages []CatalogPackage `yaml:"packages"`
	// Provides are product keys that mean it's already installed.
	Provides []string `yaml:"provides"`
	// Related are product keys of installed software that make this a
	// suggestion ("because you have …").
	Related []string `yaml:"related,omitempty"`
	// Everyone marks tools useful to almost anyone.
	Everyone bool `yaml:"everyone,omitempty"`
}

// DiscoverItem is a catalogue entry assessed against this computer.
type DiscoverItem struct {
	Entry CatalogEntry
	// Installed is true when the saved software list already has it.
	Installed   bool
	InstalledAs string
	// Via is the package manager ATLAS would use, "" if none is available.
	Via      string
	ViaLabel string
	Package  CatalogPackage
	// Unavailable explains why it can't be installed here.
	Unavailable string
	// Because lists plain reasons it is suggested; empty means browse-only.
	Because []string
}

// Discovery is Discover Apps' view of the catalogue for this computer.
type Discovery struct {
	Items []DiscoverItem
	// Scanned is false when there is no saved software list, so installed
	// software can't be recognised.
	Scanned      bool
	CatalogAsOf  string
	Categories   []string
	ManagersHere []string
}

// DiscoverService is the port the TUI uses for Discover Apps. Implemented
// by internal/discover.Service.
type DiscoverService interface {
	// Discover assesses the catalogue against the saved software list.
	// Read-only and local: no network access.
	Discover(ctx context.Context) (Discovery, error)
	// PlanInstall returns install steps for catalogue entries. It runs nothing.
	PlanInstall(ctx context.Context, ids []string) ([]UpdateStep, error)
	NewJob(steps []UpdateStep) TerminalJob
	RecordResults(ctx context.Context, results []StepResult) error
}
