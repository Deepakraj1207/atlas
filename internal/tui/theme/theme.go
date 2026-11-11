// Package theme defines the dark and light colour themes and the Lip Gloss
// styles built from them. Colours are explicit (not terminal-adaptive) so the
// user's persisted choice always wins.
package theme

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/config"
)

// Palette is the small set of colours a theme is built from.
type Palette struct {
	Name config.Theme

	Accent     lipgloss.Color // teal/cyan: focus, selection, links
	AccentSoft lipgloss.Color // dimmer accent for secondary highlights
	Text       lipgloss.Color // primary text
	Muted      lipgloss.Color // secondary text
	Subtle     lipgloss.Color // tertiary text, separators
	Surface    lipgloss.Color // background for selected rows / buttons
	Border     lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Danger     lipgloss.Color
	OnAccent   lipgloss.Color // text drawn on top of Accent
}

// Dark is the default palette.
var Dark = Palette{
	Name:       config.ThemeDark,
	Accent:     lipgloss.Color("#2DD4BF"),
	AccentSoft: lipgloss.Color("#14B8A6"),
	Text:       lipgloss.Color("#E6EDF3"),
	Muted:      lipgloss.Color("#9BA7B4"),
	Subtle:     lipgloss.Color("#5C6773"),
	Surface:    lipgloss.Color("#1F2A33"),
	Border:     lipgloss.Color("#3A4650"),
	Success:    lipgloss.Color("#4ADE80"),
	Warning:    lipgloss.Color("#FBBF24"),
	Danger:     lipgloss.Color("#F87171"),
	OnAccent:   lipgloss.Color("#06201C"),
}

// Light is the light palette.
var Light = Palette{
	Name:       config.ThemeLight,
	Accent:     lipgloss.Color("#0D9488"),
	AccentSoft: lipgloss.Color("#0F766E"),
	Text:       lipgloss.Color("#1F2933"),
	Muted:      lipgloss.Color("#52606D"),
	Subtle:     lipgloss.Color("#9AA5B1"),
	Surface:    lipgloss.Color("#E6FFFA"),
	Border:     lipgloss.Color("#CBD2D9"),
	Success:    lipgloss.Color("#15803D"),
	Warning:    lipgloss.Color("#B45309"),
	Danger:     lipgloss.Color("#B91C1C"),
	OnAccent:   lipgloss.Color("#FFFFFF"),
}

// Theme is a palette plus the derived styles screens use.
type Theme struct {
	Palette

	Title    lipgloss.Style // app / screen titles
	Heading  lipgloss.Style // section headings inside a screen
	Body     lipgloss.Style
	Muted    lipgloss.Style
	Subtle   lipgloss.Style
	Accent   lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Danger   lipgloss.Style
	Key      lipgloss.Style // keyboard hint key
	KeyDesc  lipgloss.Style // keyboard hint description
	Badge    lipgloss.Style // small label such as "Simple"
	NavItem  lipgloss.Style
	NavCur   lipgloss.Style // selected navigation item
	NavSoon  lipgloss.Style // not-yet-available navigation item
	Button   lipgloss.Style
	ButtonOn lipgloss.Style // focused button
	Panel    lipgloss.Style // bordered container, used sparingly
	Divider  lipgloss.Style
	Status   lipgloss.Style // status bar background row
}

// New builds a Theme from a palette.
func New(p Palette) Theme {
	t := Theme{Palette: p}
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(p.Accent)
	t.Heading = lipgloss.NewStyle().Bold(true).Foreground(p.Text)
	t.Body = lipgloss.NewStyle().Foreground(p.Text)
	t.Muted = lipgloss.NewStyle().Foreground(p.Muted)
	t.Subtle = lipgloss.NewStyle().Foreground(p.Subtle)
	t.Accent = lipgloss.NewStyle().Foreground(p.Accent)
	t.Success = lipgloss.NewStyle().Foreground(p.Success)
	t.Warning = lipgloss.NewStyle().Foreground(p.Warning)
	t.Danger = lipgloss.NewStyle().Foreground(p.Danger)
	t.Key = lipgloss.NewStyle().Bold(true).Foreground(p.Accent)
	t.KeyDesc = lipgloss.NewStyle().Foreground(p.Muted)
	t.Badge = lipgloss.NewStyle().Foreground(p.OnAccent).Background(p.AccentSoft).Padding(0, 1)
	t.NavItem = lipgloss.NewStyle().Foreground(p.Text).Padding(0, 1)
	t.NavCur = lipgloss.NewStyle().Bold(true).Foreground(p.Accent).Padding(0, 1)
	t.NavSoon = lipgloss.NewStyle().Foreground(p.Muted).Padding(0, 1)
	t.Button = lipgloss.NewStyle().Foreground(p.Text).Background(p.Surface).Padding(0, 2)
	t.ButtonOn = lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Accent).Padding(0, 2)
	t.Panel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).Padding(0, 1)
	t.Divider = lipgloss.NewStyle().Foreground(p.Border)
	t.Status = lipgloss.NewStyle().Foreground(p.Muted)
	return t
}

// ByName returns the theme for a config value, defaulting to Dark.
func ByName(name config.Theme) Theme {
	if name == config.ThemeLight {
		return New(Light)
	}
	return New(Dark)
}
