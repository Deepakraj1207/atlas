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

func TestAIIsOffByDefaultAndValidated(t *testing.T) {
	if d := Default(); d.AI.Enabled || d.AI.Model != "" {
		t.Errorf("AI must be off with the built-in model by default: %+v", d.AI)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	os.WriteFile(path, []byte("ai:\n  enabled: true\n  model: claude-sonnet-5-5\n"), 0o644)
	res := Load(path)
	if !res.Config.AI.Enabled || res.Config.AI.Model != "claude-sonnet-5-5" || len(res.Warnings) != 0 {
		t.Errorf("loaded %+v, warnings %v", res.Config.AI, res.Warnings)
	}
	os.WriteFile(path, []byte("ai:\n  enabled: true\n  model: \"gpt; rm -rf\"\n"), 0o644)
	res = Load(path)
	if res.Config.AI.Model != "" || len(res.Warnings) != 1 {
		t.Errorf("a bad model name should be ignored with a warning: %+v %v", res.Config.AI, res.Warnings)
	}
	// Round trip: saving keeps the choice.
	if err := Save(path, res.Config, false); err != nil {
		t.Fatal(err)
	}
	if !Load(path).Config.AI.Enabled {
		t.Error("enabled lost on save")
	}
}

func TestAIProviderAndLimits(t *testing.T) {
	d := Default().AI
	if d.AIProviderName() != AIProviderAnthropic || d.OutputTokens() != DefaultAIMaxOutputTokens || d.OllamaEndpoint() != DefaultOllamaURL {
		t.Errorf("defaults: %s %d %s", d.AIProviderName(), d.OutputTokens(), d.OllamaEndpoint())
	}
	path := filepath.Join(t.TempDir(), FileName)
	load := func(yaml string) Result {
		os.WriteFile(path, []byte(yaml), 0o644)
		return Load(path)
	}
	res := load("ai:\n  enabled: true\n  provider: ollama\n  model: \"qwen2.5:7b-instruct\"\n  max_output_tokens: 2048\n  ollama_url: http://127.0.0.1:11434/\n")
	a := res.Config.AI
	if len(res.Warnings) != 0 || a.AIProviderName() != AIProviderOllama || a.Model != "qwen2.5:7b-instruct" || a.OutputTokens() != 2048 || a.OllamaEndpoint() != "http://127.0.0.1:11434" {
		t.Errorf("ollama settings: %+v %v", a, res.Warnings)
	}
	if err := Save(path, res.Config, false); err != nil {
		t.Fatal(err)
	}
	if again := Load(path).Config.AI; again != a {
		t.Errorf("save lost settings: %+v vs %+v", again, a)
	}

	for _, c := range []struct {
		yaml, warn string
		check      func(AIConfig) bool
	}{
		{"ai:\n  provider: openai\n", "Unknown AI provider", func(a AIConfig) bool { return a.AIProviderName() == AIProviderAnthropic }},
		{"ai:\n  provider: ollama\n  model: claude-opus-5-5 && x\n", "Ollama model name", func(a AIConfig) bool { return a.Model == "" }},
		{"ai:\n  model: llama3.2\n", "Claude model ID", func(a AIConfig) bool { return a.Model == "" }},
		{"ai:\n  max_output_tokens: 100\n", "too small", func(a AIConfig) bool { return a.OutputTokens() == MinAIMaxOutputTokens }},
		{"ai:\n  max_output_tokens: 128000\n", "more than", func(a AIConfig) bool { return a.OutputTokens() == MaxAIMaxOutputTokens }},
		{"ai:\n  ollama_url: file:///etc/passwd\n", "Ollama address", func(a AIConfig) bool { return a.OllamaEndpoint() == DefaultOllamaURL }},
		{"ai:\n  ollama_url: http://user:pw@host:11434\n", "Ollama address", func(a AIConfig) bool { return a.OllamaEndpoint() == DefaultOllamaURL }},
	} {
		res := load(c.yaml)
		if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], c.warn) || !c.check(res.Config.AI) {
			t.Errorf("%q: %+v %v", c.yaml, res.Config.AI, res.Warnings)
		}
	}
}
