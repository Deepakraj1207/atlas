// Package theme defines the dark and light colour themes and the Lip Gloss
// styles built from them. Colours are explicit (not terminal-adaptive) so the
// user's persisted choice always wins. The look is deliberately quiet: text
// carries the hierarchy, colour is kept for focus and status.
package theme

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/config"
)

// Palette is the small set of colours a theme is built from.
type Palette struct {
	Name config.Theme

	Accent     lipgloss.Color // focus, selection, the brand mark
	AccentSoft lipgloss.Color // dimmer accent: unfocused selection, badges
	Text       lipgloss.Color // primary text
	Muted      lipgloss.Color // secondary text
	Subtle     lipgloss.Color // tertiary text, hints
	Surface    lipgloss.Color // background of the selected row
	Border     lipgloss.Color // separators and the few panels
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Danger     lipgloss.Color
	Info       lipgloss.Color // running tasks
	OnAccent   lipgloss.Color // text drawn on top of Accent
}

// Dark is the default palette, tuned for dark terminal backgrounds such as
// Windows Terminal's and PowerShell's defaults.
var Dark = Palette{
	Name:       config.ThemeDark,
	Accent:     lipgloss.Color("#9AA8FF"),
	AccentSoft: lipgloss.Color("#6672C9"),
	Text:       lipgloss.Color("#E6E8EF"),
	Muted:      lipgloss.Color("#9097A8"),
	Subtle:     lipgloss.Color("#5E6576"),
	Surface:    lipgloss.Color("#262A3B"),
	Border:     lipgloss.Color("#363B4E"),
	Success:    lipgloss.Color("#6FD49A"),
	Warning:    lipgloss.Color("#F2C46D"),
	Danger:     lipgloss.Color("#F2797B"),
	Info:       lipgloss.Color("#7CC7E8"),
	OnAccent:   lipgloss.Color("#141726"),
}

// Light is the light palette.
var Light = Palette{
	Name:       config.ThemeLight,
	Accent:     lipgloss.Color("#4A55C2"),
	AccentSoft: lipgloss.Color("#7C86D9"),
	Text:       lipgloss.Color("#1E2230"),
	Muted:      lipgloss.Color("#5A6072"),
	Subtle:     lipgloss.Color("#9AA0AF"),
	Surface:    lipgloss.Color("#E8EAF8"),
	Border:     lipgloss.Color("#D3D6E2"),
	Success:    lipgloss.Color("#1E7F4B"),
	Warning:    lipgloss.Color("#A3640B"),
	Danger:     lipgloss.Color("#B4282B"),
	Info:       lipgloss.Color("#1F6E95"),
	OnAccent:   lipgloss.Color("#FFFFFF"),
}

// Theme is a palette plus the derived styles screens use.
type Theme struct {
	Palette

	Title    lipgloss.Style // screen titles
	Heading  lipgloss.Style // section headings inside a screen
	Label    lipgloss.Style // small section label such as "SYSTEM"
	Body     lipgloss.Style
	Muted    lipgloss.Style
	Subtle   lipgloss.Style
	Accent   lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Danger   lipgloss.Style
	Info     lipgloss.Style
	Strong   lipgloss.Style // emphasised figures, e.g. "87" in "87 apps"
	Logo     lipgloss.Style // the brand name
	Key      lipgloss.Style // keyboard hint key
	KeyDesc  lipgloss.Style // keyboard hint description
	Badge    lipgloss.Style // small label such as "Simple"
	NavItem  lipgloss.Style
	NavCur   lipgloss.Style // selected navigation item
	NavSoon  lipgloss.Style // not-yet-available navigation item
	Selected lipgloss.Style // the highlighted row of a focused list
	Button   lipgloss.Style
	ButtonOn lipgloss.Style // focused button
	Panel    lipgloss.Style // bordered container, used sparingly
	Divider  lipgloss.Style
	Status   lipgloss.Style // status bar text
}

// New builds a Theme from a palette.
func New(p Palette) Theme {
	t := Theme{Palette: p}
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(p.Text)
	t.Heading = lipgloss.NewStyle().Bold(true).Foreground(p.Text)
	t.Label = lipgloss.NewStyle().Bold(true).Foreground(p.Subtle)
	t.Body = lipgloss.NewStyle().Foreground(p.Text)
	t.Muted = lipgloss.NewStyle().Foreground(p.Muted)
	t.Subtle = lipgloss.NewStyle().Foreground(p.Subtle)
	t.Accent = lipgloss.NewStyle().Foreground(p.Accent)
	t.Success = lipgloss.NewStyle().Foreground(p.Success)
	t.Warning = lipgloss.NewStyle().Foreground(p.Warning)
	t.Danger = lipgloss.NewStyle().Foreground(p.Danger)
	t.Info = lipgloss.NewStyle().Foreground(p.Info)
	t.Strong = lipgloss.NewStyle().Bold(true).Foreground(p.Text)
	t.Logo = lipgloss.NewStyle().Bold(true).Foreground(p.Accent)
	t.Key = lipgloss.NewStyle().Bold(true).Foreground(p.Muted)
	t.KeyDesc = lipgloss.NewStyle().Foreground(p.Subtle)
	t.Badge = lipgloss.NewStyle().Foreground(p.Muted).Background(p.Surface).Padding(0, 1)
	t.NavItem = lipgloss.NewStyle().Foreground(p.Muted)
	t.NavCur = lipgloss.NewStyle().Bold(true).Foreground(p.Accent)
	t.NavSoon = lipgloss.NewStyle().Foreground(p.Subtle)
	t.Selected = lipgloss.NewStyle().Bold(true).Foreground(p.Text).Background(p.Surface)
	t.Button = lipgloss.NewStyle().Foreground(p.Muted).Background(p.Surface).Padding(0, 2)
	t.ButtonOn = lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Accent).Padding(0, 2)
	t.Panel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).Padding(0, 1)
	t.Divider = lipgloss.NewStyle().Foreground(p.Border)
	t.Status = lipgloss.NewStyle().Foreground(p.Muted)
	return t
}

// Tone is the meaning of a status indicator.
type Tone int

const (
	ToneNeutral Tone = iota
	ToneOK
	ToneBusy
	ToneWarn
	ToneBad
)

// Dot renders the coloured "●" status indicator for a tone. A neutral tone
// is a hollow "○": nothing known yet.
func (t Theme) Dot(tone Tone) string {
	switch tone {
	case ToneOK:
		return t.Success.Render("●")
	case ToneBusy:
		return t.Info.Render("●")
	case ToneWarn:
		return t.Warning.Render("●")
	case ToneBad:
		return t.Danger.Render("●")
	default:
		return t.Subtle.Render("○")
	}
}

// ToneStyle is the text style that goes with a tone.
func (t Theme) ToneStyle(tone Tone) lipgloss.Style {
	switch tone {
	case ToneOK:
		return t.Success
	case ToneBusy:
		return t.Info
	case ToneWarn:
		return t.Warning
	case ToneBad:
		return t.Danger
	default:
		return t.Muted
	}
}

// ByName returns the theme for a config value, defaulting to Dark.
func ByName(name config.Theme) Theme {
	if name == config.ThemeLight {
		return New(Light)
	}
	return New(Dark)
}
