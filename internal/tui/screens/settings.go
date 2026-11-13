package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// Settings lets the user change persisted preferences.
type Settings struct {
	cursor int
}

// NewSettings creates the Settings screen.
func NewSettings() *Settings { return &Settings{} }

type settingRow struct {
	label  string
	value  string
	help   string
	toggle func(ctx Context) tea.Msg
	// action rows open a preview instead of flipping a value.
	action bool
}

func (s *Settings) rows(ctx Context) []settingRow {
	ui := ctx.Config.UI
	onOff := func(b bool) string {
		if b {
			return "On"
		}
		return "Off"
	}
	return []settingRow{
		{
			label:  "Mode",
			value:  ui.Mode.Label(),
			help:   "Simple hides technical detail. Advanced shows package IDs, versions, paths and command previews.",
			toggle: func(ctx Context) tea.Msg { return SetModeMsg{Mode: ctx.Config.UI.Mode.Toggle()} },
		},
		{
			label:  "Theme",
			value:  ui.Theme.Label(),
			help:   "Dark or light colours with a teal accent.",
			toggle: func(ctx Context) tea.Msg { return SetThemeMsg{Theme: ctx.Config.UI.Theme.Toggle()} },
		},
		{
			label:  "Keyboard hints",
			value:  onOff(ui.ShowHints),
			help:   "Show the key reminders at the bottom of the screen.",
			toggle: func(ctx Context) tea.Msg { return SetShowHintsMsg{Show: !ctx.Config.UI.ShowHints} },
		},
		{
			label:  "Preview read-only actions",
			value:  onOff(ui.ConfirmActions),
			help:   "Show what an action will do before it runs, even when it only reads information (such as scanning). Actions that change anything always ask first.",
			toggle: func(ctx Context) tea.Msg { return SetConfirmActionsMsg{Confirm: !ctx.Config.UI.ConfirmActions} },
		},
		{
			label:  "Saved software list",
			value:  savedListValue(ctx),
			help:   "Delete the list ATLAS saved from your last scan. No software on your computer is touched.",
			action: true,
			toggle: func(ctx Context) tea.Msg {
				return ShowActionPreviewMsg{Plan: ClearPlan(ctx), Confirm: ClearInventoryMsg{}}
			},
		},
		{
			label: "AI explanations",
			value: aiValue(ctx),
			help: "Off unless you turn it on. When on, press e on a Smart Analysis finding or an app in My Software to ask Claude, Anthropic's AI, to explain it. " +
				"You always see exactly what will be sent first, and nothing else leaves your computer. Needs an Anthropic API key in ANTHROPIC_API_KEY (or \"ant auth login\").",
			toggle: func(ctx Context) tea.Msg { return SetAIEnabledMsg{Enabled: !ctx.Config.AI.Enabled} },
		},
	}
}

func aiValue(ctx Context) string {
	switch {
	case !ctx.Config.AI.Enabled:
		return "Off"
	case !ctx.AI.Available:
		return "On · not available in this session"
	case !ctx.AI.Ready:
		return "On · no API key found"
	default:
		return "On"
	}
}

func savedListValue(ctx Context) string {
	d := ctx.Data
	switch {
	case !d.Available:
		return "Unavailable"
	case d.Summary == nil:
		return "Empty"
	default:
		return "Clear…"
	}
}

// Cursor returns the selected row (for tests).
func (s *Settings) Cursor() int { return s.cursor }

// Update handles row movement and toggling.
func (s *Settings) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	rows := s.rows(ctx)
	switch key.String() {
	case "up", "k":
		s.cursor = (s.cursor + len(rows) - 1) % len(rows)
	case "down", "j":
		s.cursor = (s.cursor + 1) % len(rows)
	case "enter", " ", "left", "right", "h", "l":
		row := rows[s.cursor]
		if row.action && key.String() != "enter" && key.String() != " " {
			return s, nil
		}
		return s, send(row.toggle(ctx))
	}
	return s, nil
}

// Hints lists Settings keys.
func (s *Settings) Hints(ctx Context) []components.KeyHint {
	return []components.KeyHint{
		{Key: "↑/↓", Desc: "choose setting"},
		{Key: "enter", Desc: "change"},
	}
}

// View renders the settings list and, in Advanced mode, file locations.
func (s *Settings) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	var b strings.Builder

	b.WriteString(t.Title.Render("Settings"))
	b.WriteString("\n")
	b.WriteString(t.Muted.Render(components.Wrap("Preferences are saved immediately and remembered next time.", w)))
	b.WriteString("\n\n")

	rows := s.rows(ctx)
	labelW := 0
	for _, r := range rows {
		labelW = max(labelW, len(r.label))
	}
	for i, r := range rows {
		prefix := "  "
		label := t.Body.Width(labelW + 2).Render(r.label)
		value := t.Button.Render(r.value)
		if i == s.cursor {
			prefix = t.Accent.Render("› ")
			label = t.Accent.Bold(true).Width(labelW + 2).Render(r.label)
			value = t.ButtonOn.Render(r.value)
		}
		b.WriteString(prefix + label + value)
		b.WriteString("\n")
		if i == s.cursor {
			b.WriteString("    " + t.Muted.Render(strings.ReplaceAll(components.Wrap(r.help, max(w-4, 10)), "\n", "\n    ")))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(s.fileStatus(ctx))

	if ctx.Advanced() {
		b.WriteString("\n\n")
		b.WriteString(t.Heading.Render("Where things live"))
		b.WriteString("\n")
		b.WriteString(components.LabelValue(t, [][2]string{
			{"Settings file", ctx.ConfigPath},
			{"Database", ctx.DBPath},
			{"Log file", ctx.LogPath},
			{"Log level", string(ctx.Config.Logging.Level)},
		}))
	}
	return b.String()
}

func (s *Settings) fileStatus(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	res := ctx.ConfigResult
	var b strings.Builder

	switch {
	case res.Err != nil:
		b.WriteString(components.ErrorState{
			Title:       "Settings file problem",
			Summary:     res.Err.Summary + ". Changing any setting here writes a fresh file; the old one is kept as a .bak copy.",
			Details:     res.Err.Details,
			ShowDetails: ctx.Advanced(),
		}.Render(t, w))
	case !res.Found:
		b.WriteString(t.Muted.Render(components.Wrap("No settings file yet. Defaults are in use; one is created the first time you change a setting.", w)))
	default:
		b.WriteString(t.Muted.Render(components.Wrap("Settings loaded from your settings file.", w)))
	}

	if len(res.Warnings) > 0 {
		b.WriteString("\n")
		b.WriteString(t.Warning.Render("Some settings were ignored:"))
		b.WriteString("\n")
		b.WriteString(components.Bullets(t, w, res.Warnings))
	}
	if !ctx.Data.Available {
		b.WriteString("\n")
		b.WriteString(components.ErrorState{
			Title:       "Saved data is unavailable",
			Summary:     "ATLAS couldn't open its database, so scanning and Activity History are turned off this session. Your settings still work.",
			Details:     ctx.Data.StoreErr,
			ShowDetails: ctx.Advanced(),
			Hint:        "Check that the data folder is writable, then restart ATLAS.",
		}.Render(t, w))
	}
	if ctx.LogError != "" {
		b.WriteString("\n")
		b.WriteString(components.ErrorState{
			Title:       "Logging is off",
			Summary:     "The log file could not be opened, so nothing is being recorded this session. ATLAS still works normally.",
			Details:     ctx.LogError,
			ShowDetails: ctx.Advanced(),
		}.Render(t, w))
	}
	return b.String()
}

var _ Screen = (*Settings)(nil)
