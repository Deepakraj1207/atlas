package discover

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/handover"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/updates"
)

// Config wires the service to this computer.
type Config struct {
	Elevated   bool
	ScoopRoots []inventory.ScoopRoot
	// AptListsDir is checked before offering APT installs ("" skips it).
	AptListsDir string
	// Run is used for read-only checks such as "flatpak remotes".
	Run inventory.Runner
	// Windows overrides runtime.GOOS (tests).
	Windows *bool
}

// Service implements core.DiscoverService.
type Service struct {
	catalog Catalog
	cfg     Config
	store   core.Store
	log     *slog.Logger
	now     func() time.Time
	start   handover.ProcessStarter
	notify  func() (<-chan os.Signal, func())
}

var _ core.DiscoverService = (*Service)(nil)

// NewService builds the service with the built-in catalogue.
func NewService(cfg Config, store core.Store, log *slog.Logger) (*Service, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}
	return NewServiceWithCatalog(c, cfg, store, log), nil
}

// NewServiceWithCatalog builds the service with a given catalogue (tests).
func NewServiceWithCatalog(c Catalog, cfg Config, store core.Store, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{catalog: c, cfg: cfg, store: store, log: log, now: time.Now, start: handover.ExecStarter, notify: handover.NotifyInterrupt}
}

// WithProcessStarter replaces how commands are started (tests).
func (s *Service) WithProcessStarter(p handover.ProcessStarter) *Service { s.start = p; return s }

func (s *Service) windows() bool {
	if s.cfg.Windows != nil {
		return *s.cfg.Windows
	}
	return runtime.GOOS == "windows"
}

// Discover assesses the catalogue against the saved software list.
func (s *Service) Discover(ctx context.Context) (core.Discovery, error) {
	here := Here{Windows: s.windows(), Managers: map[string]bool{}, Blocked: map[string]string{}}
	sum, ok, err := s.store.LatestScan(ctx)
	if err != nil {
		return core.Discovery{}, err
	}
	if ok {
		here.Scanned = true
		if here.Items, err = s.store.Software(ctx); err != nil {
			return core.Discovery{}, err
		}
		for _, src := range sum.Sources {
			if m, ok := sourceToManager[src.Source]; ok && src.Status != core.SourceUnavailable {
				here.Managers[m] = true
			}
		}
	}
	if !s.cfg.Elevated {
		here.Blocked["chocolatey"] = "Chocolatey only installs from an administrator terminal. Start ATLAS with “Run as administrator” to use it."
	}
	if here.Managers["flatpak"] && !s.hasFlathub(ctx) {
		here.Blocked["flatpak"] = flathubMissing
	}
	if here.Managers["apt"] && s.cfg.AptListsDir != "" && !updates.HasAptLists(s.cfg.AptListsDir) {
		here.Blocked["apt"] = "APT has no package information yet. Run 'sudo apt update' (your system's normal update step) first."
	}
	return Assess(s.catalog, here), nil
}

// PlanInstall returns install steps. Entries already installed, or not
// installable here, come back as blocked steps with the reason.
func (s *Service) PlanInstall(ctx context.Context, ids []string) ([]core.UpdateStep, error) {
	d, err := s.Discover(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[string]core.DiscoverItem{}
	for _, it := range d.Items {
		byID[it.Entry.ID] = it
	}
	var steps []core.UpdateStep
	for _, id := range ids {
		it, ok := byID[id]
		if !ok {
			continue
		}
		target := core.Update{Manager: it.Via, Name: it.Entry.Name, PackageID: it.Package.ID}
		st := core.UpdateStep{Manager: it.Via, Label: it.ViaLabel, Action: core.ActionInstall, Updates: []core.Update{target}}
		switch {
		case it.Installed:
			st.Blocked = "It's already installed (" + it.InstalledAs + ")."
		case it.Via == "":
			st.Label, st.Blocked = "Not installable here", it.Unavailable
		default:
			s.fill(ctx, &st, it.Package)
		}
		steps = append(steps, st)
	}
	return steps, nil
}

// fill sets the commands, elevation, rollback and any blocking reason.
func (s *Service) fill(ctx context.Context, st *core.UpdateStep, p core.CatalogPackage) {
	sudo := func(c core.Command) (core.Command, core.Elevation) {
		if s.cfg.Elevated {
			return c, core.ElevationAdmin
		}
		return core.Command{Program: "sudo", Args: append([]string{c.Program}, c.Args...)}, core.ElevationSudo
	}
	removal := "You can remove it later from My Software (select it and press r)."
	switch p.Manager {
	case "winget":
		st.Commands = []core.Command{{Program: "winget", Args: []string{"install", "--id", p.ID, "--exact", "--source", "winget"}}}
		st.Elevation = core.ElevationInstaller
	case "scoop":
		bucket, _, _ := strings.Cut(p.ID, "/")
		if !s.hasScoopBucket(bucket) {
			st.Commands = append(st.Commands, core.Command{Program: "scoop", Args: []string{"bucket", "add", bucket}})
		}
		st.Commands = append(st.Commands, core.Command{Program: "scoop", Args: []string{"install", p.ID}})
		st.Elevation = core.ElevationNone
	case "chocolatey":
		st.Commands = []core.Command{{Program: "choco", Args: []string{"install", p.ID}, RestartCodes: []int{1641, 3010}}}
		st.Elevation = core.ElevationAdmin
	case "apt":
		c, e := sudo(core.Command{Program: "apt-get", Args: []string{"install", p.ID}})
		st.Commands, st.Elevation = []core.Command{c}, e
	case "dnf":
		c, e := sudo(core.Command{Program: "dnf", Args: []string{"install", p.ID}})
		st.Commands, st.Elevation = []core.Command{c}, e
	case "snap":
		args := []string{"install", p.ID}
		if p.Classic {
			args = append(args, "--classic")
		}
		c, e := sudo(core.Command{Program: "snap", Args: args})
		st.Commands, st.Elevation = []core.Command{c}, e
	case "flatpak":
		st.Commands = []core.Command{{Program: "flatpak", Args: []string{"install", "flathub", p.ID}}}
		st.Elevation = core.ElevationPolkit
		if !s.hasFlathub(ctx) {
			st.Blocked = flathubMissing
		}
	}
	st.Rollback = removal
	if p.Note != "" {
		st.Rollback = p.Note + " " + removal
	}
	if p.Classic {
		st.Rollback = "This snap uses classic confinement, which gives it the same access to your system as a normally installed program. " + st.Rollback
	}
}

const flathubMissing = "Flathub isn't set up on this computer, so Flatpak can't install from it. See flathub.org/setup."

func (s *Service) hasScoopBucket(bucket string) bool {
	for _, r := range s.cfg.ScoopRoots {
		if _, err := os.Stat(filepath.Join(r.Dir, "buckets", bucket)); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) hasFlathub(ctx context.Context) bool {
	if s.cfg.Run == nil {
		return true
	}
	out, err := s.cfg.Run(ctx, "flatpak", "remotes", "--columns=name")
	if err != nil {
		return false
	}
	for _, l := range strings.Fields(string(out)) {
		if l == "flathub" {
			return true
		}
	}
	return false
}

// NewJob prepares a terminal job. Nothing runs until the TUI calls Run.
func (s *Service) NewJob(steps []core.UpdateStep) core.TerminalJob {
	return handover.New(steps, s.start, s.notify)
}

// RecordResults records the outcome in Activity History.
func (s *Service) RecordResults(ctx context.Context, results []core.StepResult) error {
	var done, failed, stopped, skipped []string
	var lines []string
	restart := false
	for _, r := range results {
		name := ""
		if len(r.Step.Updates) > 0 {
			name = r.Step.Updates[0].Name
		}
		switch r.Status {
		case core.StepDone:
			done = append(done, name)
			restart = restart || r.Restart
		case core.StepFailed:
			failed = append(failed, name)
		case core.StepStopped:
			stopped = append(stopped, name)
		case core.StepSkipped:
			skipped = append(skipped, name)
		}
		line := fmt.Sprintf("%s: %s", r.Step.Label, r.Status)
		if r.Status == core.StepFailed {
			line += fmt.Sprintf(" (exit code %d)", r.ExitCode)
		}
		if r.Command != "" {
			line += " — " + r.Command
		}
		if r.Note != "" {
			line += " — " + r.Note
		}
		lines = append(lines, line)
		s.log.Info("install step finished", "manager", r.Step.Manager, "status", string(r.Status), "exit_code", r.ExitCode, "command", r.Command)
	}
	a := core.Activity{At: s.now(), Kind: "discover.install", Details: strings.Join(lines, "\n")}
	switch {
	case len(done) > 0 && len(failed)+len(stopped) == 0:
		a.Title, a.Outcome = "Installed "+inventory.Plural(len(done), "app"), core.OutcomeSuccess
	case len(done) > 0:
		a.Title, a.Outcome = "Some installs didn't finish", core.OutcomePartial
	case len(stopped) > 0:
		a.Title, a.Outcome = "Installs stopped", core.OutcomeCancelled
	case len(failed) > 0:
		a.Title, a.Outcome = "Installs didn't work", core.OutcomeFailed
	default:
		a.Title, a.Outcome = "Nothing was installed", core.OutcomeInfo
	}
	var parts []string
	for _, p := range []struct {
		label string
		names []string
	}{{"Installed", done}, {"Didn't work", failed}, {"Stopped", stopped}, {"Skipped", skipped}} {
		if len(p.names) > 0 {
			parts = append(parts, p.label+": "+strings.Join(p.names, ", ")+".")
		}
	}
	if len(done) > 0 {
		parts = append(parts, "Scan again to add them to My Software.")
	}
	if restart {
		parts = append(parts, "Restart your computer to finish.")
	}
	a.Summary = strings.Join(parts, " ")
	_, err := s.store.AddActivity(ctx, a)
	return err
}
