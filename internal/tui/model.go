// Package tui is the Bubble Tea root model: it owns navigation, the
// mode/theme toggles, overlays, the status bar, and delegates the content
// area to the screen for the current section.
package tui

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/layout"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Options configure the root model.
type Options struct {
	ConfigResult config.Result
	LogPath      string
	LogError     string
	Logger       *slog.Logger
	Version      string

	// Inventory and Updates are nil when the database could not be opened;
	// StoreErr then explains why.
	Inventory core.InventoryService
	Updates   core.UpdateService
	Analysis  core.AnalysisService
	StoreErr  string
	DBPath    string
	// Elevated is true when ATLAS runs as administrator or root.
	Elevated bool
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg     config.Config
	cfgRes  config.Result
	logPath string
	logErr  string
	log     *slog.Logger
	version string

	theme   theme.Theme
	nav     nav.State
	layout  layout.Layout
	screens map[nav.Section]screens.Screen

	inv      core.InventoryService
	upd      core.UpdateService
	elevated bool
	check    *scanRun
	ana      core.AnalysisService
	analysis *scanRun
	// exec hands the terminal to a job; replaced in tests.
	exec     func(core.TerminalJob, tea.ExecCallback) tea.Cmd
	storeErr string
	dbPath   string
	data     screens.DataView
	scan     *scanRun
	spinner  spinner.Model
	now      func() time.Time

	helpOpen bool
	// overlayScroll is the scroll offset of the help or preview overlay.
	overlayScroll int
	preview       *core.ActionPlan
	// confirm is dispatched when an available previewed plan is accepted.
	confirm tea.Msg
	// scroll is the content scroll offset per section, in lines.
	scroll map[nav.Section]int

	status    components.Status
	statusSeq int
	ready     bool
}

// New builds the root model from loaded configuration.
func New(opts Options) Model {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	m := Model{
		cfg:      opts.ConfigResult.Config,
		cfgRes:   opts.ConfigResult,
		logPath:  opts.LogPath,
		logErr:   opts.LogError,
		log:      log,
		version:  opts.Version,
		theme:    theme.ByName(opts.ConfigResult.Config.UI.Theme),
		nav:      nav.NewState(),
		layout:   layout.Compute(80, 24),
		screens:  map[nav.Section]screens.Screen{},
		scroll:   map[nav.Section]int{},
		inv:      opts.Inventory,
		upd:      opts.Updates,
		ana:      opts.Analysis,
		elevated: opts.Elevated,
		exec:     func(j core.TerminalJob, cb tea.ExecCallback) tea.Cmd { return tea.Exec(j, cb) },
		storeErr: opts.StoreErr,
		dbPath:   opts.DBPath,
		now:      time.Now,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Line)),
	}
	m.data.Available = m.inv != nil
	m.data.UpdatesAvailable = m.upd != nil
	m.data.AnalysisAvailable = m.ana != nil
	m.data.StoreErr = m.storeErr
	if m.inv == nil && m.storeErr == "" {
		m.data.StoreErr = "no database was configured"
	}
	m.spinner.Style = m.theme.Accent
	for _, s := range nav.All() {
		switch s {
		case nav.Home:
			m.screens[s] = screens.NewHome()
		case nav.MySoftware:
			m.screens[s] = screens.NewMySoftware()
		case nav.SmartAnalysis:
			m.screens[s] = screens.NewSmartAnalysis()
		case nav.Dependencies:
			m.screens[s] = screens.NewDependencies()
		case nav.Updates:
			m.screens[s] = screens.NewUpdates()
		case nav.ActivityHistory:
			m.screens[s] = screens.NewActivityHistory()
		case nav.Settings:
			m.screens[s] = screens.NewSettings()
		default:
			m.screens[s] = screens.NewComingSoon(s)
		}
	}
	m.status = m.startupStatus()
	return m
}

// startupStatus picks the first message the user sees.
func (m Model) startupStatus() components.Status {
	switch {
	case m.inv == nil:
		return components.Status{Text: "ATLAS can't open its saved data, so scanning is off. See Settings for details.", Kind: components.StatusWarning}
	case m.cfgRes.Err != nil:
		return components.Status{Text: m.cfgRes.Err.Summary + ". See Settings for details.", Kind: components.StatusWarning}
	case len(m.cfgRes.Warnings) > 0:
		return components.Status{Text: "Some settings were ignored. See Settings for details.", Kind: components.StatusWarning}
	case m.logErr != "":
		return components.Status{Text: "Logging could not start; ATLAS still works. See Settings for details.", Kind: components.StatusWarning}
	case !m.cfgRes.Found:
		return components.Status{Text: "Welcome! Using default settings. Press ? for help."}
	default:
		return components.Status{Text: "Settings loaded. Press ? for help."}
	}
}

// statusClearAfter is how long a status message stays before clearing.
// Tests shorten it.
var statusClearAfter = 8 * time.Second

// Messages internal to the root model.

type statusClearMsg struct{ seq int }

type configSavedMsg struct {
	err  error
	what string
}

// Init starts the status-clear timer and loads saved data.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.scheduleStatusClear(), loadData(m.inv), loadUpdates(m.upd), loadAnalysis(m.ana))
}

// Accessors used by tests.

// Section returns the active section.
func (m Model) Section() nav.Section { return m.nav.Current() }

// Config returns the effective configuration.
func (m Model) Config() config.Config { return m.cfg }

// Layout returns the current layout decisions.
func (m Model) Layout() layout.Layout { return m.layout }

// HelpOpen reports whether the help overlay is showing.
func (m Model) HelpOpen() bool { return m.helpOpen }

// PreviewOpen reports whether an action preview is showing.
func (m Model) PreviewOpen() bool { return m.preview != nil }

// Data returns the current data snapshot (for tests).
func (m Model) Data() screens.DataView { return m.dataView() }

// Scanning reports whether a scan is running.
func (m Model) Scanning() bool { return m.scan != nil }

// Checking reports whether an update check is running.
func (m Model) Checking() bool { return m.check != nil }

// Analyzing reports whether an analysis is running.
func (m Model) Analyzing() bool { return m.analysis != nil }

// Screen returns the screen for a section (for tests).
func (m Model) Screen(s nav.Section) screens.Screen { return m.screens[s] }

// Scroll returns the content scroll offset of the current section.
func (m Model) Scroll() int { return m.scroll[m.nav.Current()] }

// Status returns the current status bar message.
func (m Model) Status() components.Status { return m.status }

// ThemeName returns the active theme.
func (m Model) ThemeName() config.Theme { return m.theme.Name }

func (m Model) dataView() screens.DataView {
	d := m.data
	if m.scan != nil {
		p := m.scan.progress
		p.Spinner = m.spinner.View()
		d.Scan = &p
	}
	if m.check != nil {
		p := m.check.progress
		p.Spinner = m.spinner.View()
		d.Check = &p
	}
	if m.analysis != nil {
		p := m.analysis.progress
		p.Spinner = m.spinner.View()
		d.Analyze = &p
	}
	for _, a := range d.Activity {
		if a.Kind == "updates.apply" {
			d.LastApplied = a.At
			break
		}
	}
	return d
}

func (m Model) ctx() screens.Context {
	var sources, managers []core.SourceInfo
	if m.inv != nil {
		sources = m.inv.Sources()
	}
	if m.upd != nil {
		managers = m.upd.Managers()
	}
	var anaSources []core.SourceInfo
	if m.ana != nil {
		anaSources = m.ana.Sources()
	}
	return screens.Context{
		Theme:           m.theme,
		Layout:          m.layout,
		Config:          m.cfg,
		Now:             m.now(),
		ConfigPath:      m.cfgRes.Path,
		ConfigResult:    m.cfgRes,
		LogPath:         m.logPath,
		LogError:        m.logErr,
		DBPath:          m.dbPath,
		Version:         m.version,
		Platform:        runtime.GOOS + "/" + runtime.GOARCH,
		Sources:         sources,
		Managers:        managers,
		AnalysisSources: anaSources,
		Elevated:        m.elevated,
		Data:            m.dataView(),
	}
}

// Update is the Bubble Tea update loop.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = layout.Compute(msg.Width, msg.Height)
		m.ready = true
		m.log.Debug("resize", "width", msg.Width, "height", msg.Height, "breakpoint", m.layout.Breakpoint.String())
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case statusClearMsg:
		if msg.seq == m.statusSeq {
			m.status = components.Status{}
		}
		return m, nil

	case configSavedMsg:
		if msg.err != nil {
			m.log.Error("save settings", "error", msg.err)
			return m, m.setStatus(components.Status{
				Text: msg.what + ", but it could not be saved: " + msg.err.Error(),
				Kind: components.StatusError,
			})
		}
		m.log.Info("settings saved", "path", m.cfgRes.Path)
		// A successful save repairs an earlier load problem.
		m.cfgRes.Err = nil
		m.cfgRes.Warnings = nil
		m.cfgRes.Found = true
		return m, m.setStatus(components.Status{Text: msg.what + " (saved)", Kind: components.StatusSuccess})

	case screens.ShowActionPreviewMsg:
		plan := msg.Plan
		if plan.Available && !plan.ChangesSystem() && !m.cfg.UI.ConfirmActions && msg.Confirm != nil {
			m.log.Info("action started without preview (read-only, previews off)", "action", plan.Title)
			return m.Update(msg.Confirm)
		}
		m.preview = &plan
		m.confirm = msg.Confirm
		m.overlayScroll = 0
		m.log.Info("action preview opened", "action", plan.Title, "available", plan.Available)
		return m, nil

	case screens.StartScanMsg:
		return m, m.startScan()

	case screens.StartCheckMsg:
		return m, m.startCheck()

	case screens.RequestApplyMsg:
		if m.upd == nil {
			return m, m.setStatus(components.Status{Text: "Updates are unavailable because ATLAS can't open its saved data.", Kind: components.StatusError})
		}
		steps := m.upd.Plan(msg.Updates)
		plan := screens.ApplyPlan(m.ctx(), steps)
		return m.Update(screens.ShowActionPreviewMsg{Plan: plan, Confirm: screens.ApplyUpdatesMsg{Steps: steps}})

	case screens.ApplyUpdatesMsg:
		if m.upd == nil || m.busy() {
			return m, m.setStatus(components.Status{Text: "Updates can't start right now. Nothing was changed.", Kind: components.StatusWarning})
		}
		job := m.upd.NewJob(msg.Steps)
		m.log.Info("handing terminal to update job", "steps", len(msg.Steps))
		return m, m.exec(job, func(err error) tea.Msg {
			return updatesFinishedMsg{results: job.Results(), err: err}
		})

	case updatesFinishedMsg:
		if msg.err != nil {
			m.log.Error("update job", "error", msg.err)
		}
		if len(msg.results) == 0 {
			text := "The updates couldn't start. Nothing was changed."
			if msg.err != nil {
				text += " (" + msg.err.Error() + ")"
			}
			return m, tea.Batch(m.setStatus(components.Status{Text: text, Kind: components.StatusError}), tea.ClearScreen)
		}
		return m, tea.Batch(recordResults(m.upd, msg.results), tea.ClearScreen)

	case updatesRecordedMsg:
		if msg.err != nil {
			m.log.Error("record update results", "error", msg.err)
		}
		return m, tea.Batch(m.setStatus(applyResultStatus(msg.results, msg.err)), loadData(m.inv), loadUpdates(m.upd))

	case checkEventMsg:
		applyEvent(m.check, msg.ev)
		if m.check == nil {
			return m, nil
		}
		return m, listen(m.check.events)

	case checkDoneMsg:
		m.check = nil
		switch msg.check.Status {
		case core.ScanFailed, core.ScanNothingToScan:
			c := msg.check
			m.data.FailedCheck = &c
		case core.ScanComplete, core.ScanPartial:
			m.data.FailedCheck = nil
		}
		return m, tea.Batch(m.setStatus(checkResultStatus(msg.check, msg.err)), loadData(m.inv), loadUpdates(m.upd))

	case updatesLoadedMsg:
		if msg.err != nil {
			m.log.Error("load updates", "error", msg.err)
			m.data.LoadErr = msg.err.Error()
		}
		m.data.UpdateSummary, m.data.Updates = msg.summary, msg.updates
		return m, nil

	case screens.ClearInventoryMsg:
		if m.inv == nil || m.scan != nil || m.data.Summary == nil {
			return m, m.setStatus(components.Status{Text: "There is no saved list to clear right now.", Kind: components.StatusWarning})
		}
		return m, clearInventory(m.inv)

	case scanEventMsg:
		applyEvent(m.scan, msg.ev)
		if m.scan == nil {
			return m, nil
		}
		return m, listen(m.scan.events)

	case scanDoneMsg:
		m.scan = nil
		st := scanResultStatus(msg.report, msg.err, m.cfg.UI.Mode == config.ModeAdvanced)
		return m, tea.Batch(m.setStatus(st), loadData(m.inv))

	case inventoryClearedMsg:
		if msg.err != nil {
			m.log.Error("clear inventory", "error", msg.err)
			return m, tea.Batch(m.setStatus(components.Status{Text: "The saved list couldn't be cleared: " + msg.err.Error(), Kind: components.StatusError}), loadData(m.inv))
		}
		return m, tea.Batch(m.setStatus(components.Status{Text: "Saved software list cleared. No software on your computer was touched.", Kind: components.StatusSuccess}), loadData(m.inv), loadAnalysis(m.ana))

	case screens.StartAnalysisMsg:
		return m, m.startAnalysis()

	case analysisEventMsg:
		applyEvent(m.analysis, msg.ev)
		if m.analysis == nil {
			return m, nil
		}
		return m, listen(m.analysis.events)

	case analysisDoneMsg:
		m.analysis = nil
		return m, tea.Batch(m.setStatus(analysisResultStatus(msg.result, msg.err)), loadData(m.inv), loadAnalysis(m.ana))

	case analysisLoadedMsg:
		if msg.err != nil {
			m.log.Error("load analysis", "error", msg.err)
			m.data.LoadErr = msg.err.Error()
		}
		m.data.Analysis = msg.result
		return m, nil

	case dataLoadedMsg:
		m.data.Loaded = true
		m.data.LoadErr = ""
		if msg.err != nil {
			m.data.LoadErr = msg.err.Error()
			m.log.Error("load saved data", "error", msg.err)
		}
		m.data.Summary, m.data.Items, m.data.Activity = msg.summary, msg.items, msg.activity
		return m, nil

	case spinner.TickMsg:
		if !m.busy() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case screens.SetModeMsg:
		return m.setMode(msg.Mode)

	case screens.SetThemeMsg:
		return m.setTheme(msg.Theme)

	case screens.SetShowHintsMsg:
		m.cfg.UI.ShowHints = msg.Show
		what := "Keyboard hints off"
		if msg.Show {
			what = "Keyboard hints on"
		}
		return m, m.save(what)

	case screens.SetConfirmActionsMsg:
		m.cfg.UI.ConfirmActions = msg.Confirm
		what := "Action previews off"
		if msg.Confirm {
			what = "Action previews on"
		}
		return m, m.save(what)

	case screens.StatusMsg:
		return m, m.setStatus(msg.Status)

	case screens.NavigateMsg:
		m.nav.Go(msg.Section)
		return m, nil
	}

	return m.updateScreen(msg)
}

func (m Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := key.String()

	if k == "ctrl+c" {
		return m.quit()
	}

	// Overlays capture input first.
	if m.preview != nil || m.helpOpen {
		switch k {
		case "pgdown", "ctrl+d", "down", "j":
			step := 1
			if k == "pgdown" || k == "ctrl+d" {
				step = max(m.layout.ContentHeight/2, 1)
			}
			m.overlayScroll += step
			return m, nil
		case "pgup", "ctrl+u", "up", "k":
			step := 1
			if k == "pgup" || k == "ctrl+u" {
				step = max(m.layout.ContentHeight/2, 1)
			}
			m.overlayScroll = max(m.overlayScroll-step, 0)
			return m, nil
		}
	}
	if m.preview != nil {
		switch k {
		case "enter":
			plan, confirm := *m.preview, m.confirm
			m.preview, m.confirm = nil, nil
			if plan.Available && confirm != nil {
				m.log.Info("action confirmed", "action", plan.Title)
				return m.Update(confirm)
			}
			return m, m.setStatus(components.Status{Text: "Preview closed. Nothing was changed."})
		case "esc", "q", " ":
			m.preview, m.confirm = nil, nil
			return m, m.setStatus(components.Status{Text: "Cancelled. Nothing was changed."})
		case "m":
			return m.setMode(m.cfg.UI.Mode.Toggle())
		}
		return m, nil
	}
	if m.helpOpen {
		switch k {
		case "?", "esc", "q", "enter":
			m.helpOpen = false
		}
		return m, nil
	}

	cur := m.screens[m.nav.Current()]
	if ic, ok := cur.(screens.InputCapturer); ok && ic.CapturesInput() {
		return m.updateScreen(key)
	}
	_, selfScroll := cur.(screens.SelfScrolling)

	switch k {
	case "q":
		return m.quit()
	case "x":
		if m.busy() {
			m.cancelTasks()
			return m, nil
		}
	case "?":
		m.helpOpen = true
		m.overlayScroll = 0
		return m, nil
	case "tab":
		m.nav.Next()
		m.scroll[m.nav.Current()] = 0
		return m, nil
	case "shift+tab":
		m.nav.Prev()
		m.scroll[m.nav.Current()] = 0
		return m, nil
	case "m":
		return m.setMode(m.cfg.UI.Mode.Toggle())
	case "t":
		return m.setTheme(m.cfg.UI.Theme.Toggle())
	case "pgdown", "ctrl+d":
		if !selfScroll {
			m.scroll[m.nav.Current()] += m.layout.ContentHeight / 2
			return m, nil
		}
	case "pgup", "ctrl+u":
		if !selfScroll {
			m.scroll[m.nav.Current()] = max(m.scroll[m.nav.Current()]-m.layout.ContentHeight/2, 0)
			return m, nil
		}
	}
	if m.nav.GoShortcut(k) {
		m.scroll[m.nav.Current()] = 0
		m.log.Debug("navigate", "section", m.nav.Current().Title())
		return m, nil
	}
	return m.updateScreen(key)
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	m.cancelTasks()
	m.log.Info("quit requested")
	return m, tea.Quit
}

func (m Model) updateScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	cur := m.nav.Current()
	scr, cmd := m.screens[cur].Update(msg, m.ctx())
	m.screens[cur] = scr
	return m, cmd
}

func (m Model) setMode(mode config.Mode) (tea.Model, tea.Cmd) {
	if m.cfg.UI.Mode == mode {
		return m, nil
	}
	m.cfg.UI.Mode = mode
	m.log.Info("mode changed", "mode", string(mode))
	what := "Simple mode: technical detail hidden"
	if mode == config.ModeAdvanced {
		what = "Advanced mode: showing package IDs, paths and command previews"
	}
	return m, m.save(what)
}

func (m Model) setTheme(th config.Theme) (tea.Model, tea.Cmd) {
	if m.cfg.UI.Theme == th {
		return m, nil
	}
	m.cfg.UI.Theme = th
	m.theme = theme.ByName(th)
	m.spinner.Style = m.theme.Accent
	m.log.Info("theme changed", "theme", string(th))
	return m, m.save(th.Label() + " theme")
}

// save persists the configuration in the background and reports the result.
func (m Model) save(what string) tea.Cmd {
	cfg := m.cfg
	path := m.cfgRes.Path
	backup := m.cfgRes.Err != nil && m.cfgRes.Err.Malformed
	return func() tea.Msg {
		if path == "" {
			return configSavedMsg{err: fmt.Errorf("no settings location is available"), what: what}
		}
		return configSavedMsg{err: config.Save(path, cfg, backup), what: what}
	}
}

func (m *Model) setStatus(st components.Status) tea.Cmd {
	m.status = st
	return m.scheduleStatusClear()
}

func (m *Model) scheduleStatusClear() tea.Cmd {
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(statusClearAfter, func(time.Time) tea.Msg { return statusClearMsg{seq: seq} })
}

// View renders the whole terminal.
func (m Model) View() string {
	if !m.ready {
		return "Starting ATLAS…"
	}
	l := m.layout
	t := m.theme

	if l.Breakpoint == layout.TooSmall {
		msg := fmt.Sprintf("ATLAS needs a terminal of at least %d×%d. This one is %d×%d.\nMake the window bigger, or press q to quit.",
			layout.MinWidth, layout.MinHeight, l.Width, l.Height)
		return t.Warning.Render(msg)
	}

	var b strings.Builder
	modeBadge := t.Badge.Render(m.cfg.UI.Mode.Label() + " mode")

	if l.ShowHeader {
		b.WriteString(components.Header(t, l.Width, modeBadge+" "+t.Subtle.Render(t.Name.Label())))
		b.WriteString("\n")
	}
	if l.ShowTabBar {
		b.WriteString(components.TabBar(t, m.nav.Current(), l.Width, l.UseShortTitles))
		b.WriteString("\n")
	}

	content := m.content()
	if l.ShowSidebar {
		footer := ""
		if !l.ShowHeader {
			footer = modeBadge
		}
		side := components.Sidebar(t, m.nav.Current(), l.SidebarWidth, l.ContentHeight, footer)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, side, content))
	} else {
		b.WriteString(content)
	}
	b.WriteString("\n")

	right := t.Subtle.Render(m.nav.Current().Title())
	if !l.ShowHeader {
		right = modeBadge + " " + right
	}
	status := m.status
	if m.busy() && (status.Text == "" || status.Kind == components.StatusInfo) {
		status = m.busyStatus()
	}
	b.WriteString(components.StatusBar(t, l.Width, status, right, m.hints(), l.ShowHints && m.cfg.UI.ShowHints))
	return b.String()
}

func (m Model) content() string {
	ctx := m.ctx()
	l := m.layout
	w, h := l.ContentWidth, l.ContentHeight
	inner := lipgloss.NewStyle().Padding(0, 1)
	w -= 2 // horizontal padding
	ctx.Layout.ContentWidth = w

	var body string
	switch {
	case m.preview != nil:
		body, m.overlayScroll = components.Scroll(m.theme, components.ActionPreview(m.theme, w, *m.preview, ctx.Advanced()), w, h, m.overlayScroll)
	case m.helpOpen:
		body, m.overlayScroll = components.Scroll(m.theme, components.Help(m.theme, w, m.helpContext(), m.helpGroups()), w, h, m.overlayScroll)
	default:
		cur := m.nav.Current()
		scr := m.screens[cur]
		if _, ok := scr.(screens.SelfScrolling); ok {
			body = scr.View(ctx)
			break
		}
		// Other screens render at full height; the root scrolls and clips
		// so a short terminal still reaches everything.
		full := ctx
		full.Layout.ContentHeight = 10000
		body, m.scroll[cur] = components.Scroll(m.theme, scr.View(full), w, h, m.scroll[cur])
	}
	return inner.Render(components.Fit(body, w, h))
}

func (m Model) hints() []components.KeyHint {
	if m.preview != nil {
		if m.preview.Available && m.confirm != nil {
			return []components.KeyHint{{Key: "enter", Desc: "continue"}, {Key: "esc", Desc: "cancel"}, {Key: "↑/↓", Desc: "scroll"}, {Key: "m", Desc: "mode"}}
		}
		return []components.KeyHint{{Key: "esc", Desc: "close preview"}, {Key: "↑/↓", Desc: "scroll"}, {Key: "m", Desc: "mode"}}
	}
	if m.helpOpen {
		return []components.KeyHint{{Key: "esc", Desc: "close help"}, {Key: "↑/↓", Desc: "scroll"}}
	}
	cur := m.screens[m.nav.Current()]
	screenHints := cur.Hints(m.ctx())
	if ic, ok := cur.(screens.InputCapturer); ok && ic.CapturesInput() {
		return screenHints
	}
	var out []components.KeyHint
	switch {
	case m.scan != nil:
		out = append(out, components.KeyHint{Key: "x", Desc: "stop scan"})
	case m.analysis != nil:
		out = append(out, components.KeyHint{Key: "x", Desc: "stop analysis"})
	case m.check != nil:
		out = append(out, components.KeyHint{Key: "x", Desc: "stop check"})
	}
	out = append(out, screenHints...)
	return append(out, globalHints...)
}

func (m Model) helpContext() string {
	info := m.nav.Current().Info()
	return info.Title + ": " + info.Summary
}

func (m Model) helpGroups() []components.HelpGroup {
	cur := m.nav.Current()
	groups := []components.HelpGroup{}
	if sh := m.screens[cur].Hints(m.ctx()); len(sh) > 0 {
		groups = append(groups, components.HelpGroup{Title: "On this screen (" + cur.Title() + ")", Hints: sh})
	}
	groups = append(groups,
		components.HelpGroup{Title: "Navigation", Hints: navigationHelp},
		components.HelpGroup{Title: "General", Hints: generalHelp},
	)
	return groups
}
