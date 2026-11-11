package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsSimpleDarkWithHints(t *testing.T) {
	d := Default()
	if d.UI.Mode != ModeSimple || d.UI.Theme != ThemeDark || !d.UI.ShowHints || !d.UI.ConfirmActions {
		t.Errorf("unexpected defaults: %+v", d)
	}
	if d.Logging.Level != LogInfo {
		t.Errorf("default log level = %q, want info", d.Logging.Level)
	}
}

func TestLoadMissingFileUsesDefaultsWithoutError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	res := Load(path)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Found {
		t.Error("Found should be false for a missing file")
	}
	if res.Config != Default() {
		t.Errorf("config = %+v, want defaults", res.Config)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Load must not create the file")
	}
}

func TestLoadMalformedFileFallsBackAndReportsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ui: [this is: not: valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Load(path)
	if res.Err == nil {
		t.Fatal("expected a LoadError")
	}
	if !res.Err.Malformed {
		t.Error("Malformed should be true")
	}
	if !strings.Contains(res.Err.Summary, "formatting problem") {
		t.Errorf("summary should be plain language, got %q", res.Err.Summary)
	}
	if res.Config != Default() {
		t.Errorf("config = %+v, want defaults", res.Config)
	}
}

func TestLoadPartialFileKeepsDefaultsForMissingFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ui:\n  mode: advanced\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Load(path)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Config.UI.Mode != ModeAdvanced {
		t.Errorf("mode = %q, want advanced", res.Config.UI.Mode)
	}
	if res.Config.UI.Theme != ThemeDark {
		t.Errorf("theme = %q, want default dark", res.Config.UI.Theme)
	}
	if !res.Config.UI.ShowHints {
		t.Error("show_hints should keep its default (true) when absent")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
}

func TestLoadExplicitFalseBooleanIsHonoured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ui:\n  show_hints: false\n  confirm_actions: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Load(path)
	if res.Config.UI.ShowHints {
		t.Error("show_hints: false should be honoured")
	}
	if res.Config.UI.ConfirmActions {
		t.Error("confirm_actions: false should be honoured")
	}
}

func TestLoadInvalidEnumValuesWarnPerField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "ui:\n  theme: purple\n  mode: expert\nlogging:\n  level: loud\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Load(path)
	if res.Err != nil {
		t.Fatalf("invalid values should warn, not error: %v", res.Err)
	}
	if res.Config != Default() {
		t.Errorf("config = %+v, want defaults", res.Config)
	}
	if len(res.Warnings) != 3 {
		t.Errorf("want 3 warnings, got %d: %v", len(res.Warnings), res.Warnings)
	}
}

func TestLoadNewerVersionWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("version: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Load(path)
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "version 99") {
		t.Errorf("want version warning, got %v", res.Warnings)
	}
	if res.Config.Version != CurrentVersion {
		t.Errorf("version = %d, want %d", res.Config.Version, CurrentVersion)
	}
}

func TestSaveRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "config.yaml")
	cfg := Default()
	cfg.UI.Mode = ModeAdvanced
	cfg.UI.Theme = ThemeLight
	cfg.UI.ShowHints = false

	if err := Save(path, cfg, false); err != nil {
		t.Fatalf("Save: %v", err)
	}
	res := Load(path)
	if res.Err != nil {
		t.Fatalf("Load after Save: %v", res.Err)
	}
	if res.Config != cfg {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", res.Config, cfg)
	}
}

func TestSaveBacksUpExistingFileWhenAsked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	broken := []byte("ui: [broken\n")
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, Default(), true); err != nil {
		t.Fatalf("Save: %v", err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(bak) != string(broken) {
		t.Error("backup content differs from original")
	}
	if Load(path).Err != nil {
		t.Error("saved file should load cleanly")
	}
}

func TestToggles(t *testing.T) {
	if ModeSimple.Toggle() != ModeAdvanced || ModeAdvanced.Toggle() != ModeSimple {
		t.Error("Mode.Toggle broken")
	}
	if ThemeDark.Toggle() != ThemeLight || ThemeLight.Toggle() != ThemeDark {
		t.Error("Theme.Toggle broken")
	}
	if ModeSimple.Label() != "Simple" || ThemeLight.Label() != "Light" {
		t.Error("labels broken")
	}
}
