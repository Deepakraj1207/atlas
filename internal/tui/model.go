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

	helpOpen bool
	preview  *core.ActionPlan
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
		cfg:     opts.ConfigResult.Config,
		cfgRes:  opts.ConfigResult,
		logPath: opts.LogPath,
		logErr:  opts.LogError,
		log:     log,
		version: opts.Version,
		theme:   theme.ByName(opts.ConfigResult.Config.UI.Theme),
		nav:     nav.NewState(),
		layout:  layout.Compute(80, 24),
		screens: map[nav.Section]screens.Screen{},
		scroll:  map[nav.Section]int{},
	}
	for _, s := range nav.All() {
		switch s {
		case nav.Home:
			m.screens[s] = screens.NewHome()
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

// Init starts the status-clear timer.
func (m Model) Init() tea.Cmd {
	return m.scheduleStatusClear()
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

// Scroll returns the content scroll offset of the current section.
func (m Model) Scroll() int { return m.scroll[m.nav.Current()] }

// Status returns the current status bar message.
func (m Model) Status() components.Status { return m.status }

// ThemeName returns the active theme.
func (m Model) ThemeName() config.Theme { return m.theme.Name }

func (m Model) ctx() screens.Context {
	return screens.Context{
		Theme:        m.theme,
		Layout:       m.layout,
		Config:       m.cfg,
		ConfigPath:   m.cfgRes.Path,
		ConfigResult: m.cfgRes,
		LogPath:      m.logPath,
		LogError:     m.logErr,
		Version:      m.version,
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
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
		m.preview = &plan
		m.log.Info("action preview opened", "action", plan.Title, "available", plan.Available)
		return m, nil

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

	// Overlays capture input first.
	if m.preview != nil {
		switch k {
		case "esc", "enter", "q", " ":
			m.preview = nil
			return m, m.setStatus(components.Status{Text: "Preview closed. Nothing was changed."})
		case "m":
			return m.setMode(m.cfg.UI.Mode.Toggle())
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	if m.helpOpen {
		switch k {
		case "?", "esc", "q", "enter":
			m.helpOpen = false
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	switch k {
	case "ctrl+c", "q":
		m.log.Info("quit requested")
		return m, tea.Quit
	case "?":
		m.helpOpen = true
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
		m.scroll[m.nav.Current()] += m.layout.ContentHeight / 2
		return m, nil
	case "pgup", "ctrl+u":
		m.scroll[m.nav.Current()] = max(m.scroll[m.nav.Current()]-m.layout.ContentHeight/2, 0)
		return m, nil
	}
	if m.nav.GoShortcut(k) {
		m.scroll[m.nav.Current()] = 0
		m.log.Debug("navigate", "section", m.nav.Current().Title())
		return m, nil
	}
	return m.updateScreen(key)
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
	b.WriteString(components.StatusBar(t, l.Width, m.status, right, m.hints(), l.ShowHints && m.cfg.UI.ShowHints))
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
		body = components.ActionPreview(m.theme, w, h, *m.preview, ctx.Advanced())
	case m.helpOpen:
		body = components.Help(m.theme, w, h, m.helpContext(), m.helpGroups())
	default:
		// Screens render at full height; the root scrolls and clips so a
		// short terminal still reaches everything.
		full := ctx
		full.Layout.ContentHeight = 10000
		cur := m.nav.Current()
		body, m.scroll[cur] = components.Scroll(m.theme, m.screens[cur].View(full), w, h, m.scroll[cur])
	}
	return inner.Render(components.Fit(body, w, h))
}

func (m Model) hints() []components.KeyHint {
	if m.preview != nil {
		return []components.KeyHint{{Key: "esc", Desc: "close preview"}, {Key: "m", Desc: "mode"}}
	}
	if m.helpOpen {
		return []components.KeyHint{{Key: "esc", Desc: "close help"}}
	}
	screenHints := m.screens[m.nav.Current()].Hints(m.ctx())
	return append(screenHints, globalHints...)
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
