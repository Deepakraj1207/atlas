package theme

import (
	"testing"

	"github.com/deepakraj1207/atlas/internal/config"
)

func TestByNameSelectsPalette(t *testing.T) {
	if ByName(config.ThemeLight).Name != config.ThemeLight {
		t.Error("light theme not selected")
	}
	if ByName(config.ThemeDark).Name != config.ThemeDark {
		t.Error("dark theme not selected")
	}
	if ByName(config.Theme("nonsense")).Name != config.ThemeDark {
		t.Error("unknown theme should default to dark")
	}
}

func TestPalettesAreComplete(t *testing.T) {
	for _, p := range []Palette{Dark, Light} {
		for name, c := range map[string]string{
			"Accent": string(p.Accent), "Text": string(p.Text), "Muted": string(p.Muted),
			"Border": string(p.Border), "Danger": string(p.Danger), "OnAccent": string(p.OnAccent),
		} {
			if c == "" {
				t.Errorf("%s palette missing %s", p.Name, name)
			}
		}
	}
}
