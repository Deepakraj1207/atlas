// Package config loads and saves ATLAS user preferences from a YAML file.
//
// Loading never writes to disk. A missing file yields defaults. A malformed
// file yields defaults plus a LoadError that the UI surfaces to the user;
// the broken file is left untouched until the user saves a change, at which
// point it is backed up first.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the name of the configuration file inside the config directory.
const FileName = "config.yaml"

// CurrentVersion is the schema version written by this build.
const CurrentVersion = 1

// Mode selects how much technical detail the UI shows.
type Mode string

const (
	ModeSimple   Mode = "simple"
	ModeAdvanced Mode = "advanced"
)

// Toggle returns the other mode.
func (m Mode) Toggle() Mode {
	if m == ModeAdvanced {
		return ModeSimple
	}
	return ModeAdvanced
}

// Label is the user-facing name of the mode.
func (m Mode) Label() string {
	if m == ModeAdvanced {
		return "Advanced"
	}
	return "Simple"
}

// Theme names a colour theme.
type Theme string

const (
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
)

// Toggle returns the other theme.
func (t Theme) Toggle() Theme {
	if t == ThemeLight {
		return ThemeDark
	}
	return ThemeLight
}

// Label is the user-facing name of the theme.
func (t Theme) Label() string {
	if t == ThemeLight {
		return "Light"
	}
	return "Dark"
}

// LogLevel names a slog level.
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// Config is the persisted user preference set.
type Config struct {
	Version int       `yaml:"version"`
	UI      UIConfig  `yaml:"ui"`
	Logging LogConfig `yaml:"logging"`
	AI      AIConfig  `yaml:"ai"`
}

// AIConfig controls the optional AI explanations. Off by default: nothing is
// ever sent to an AI service unless the user turns this on, and even then
// only after a preview of exactly what will be sent.
type AIConfig struct {
	Enabled bool `yaml:"enabled"`
	// Provider is who answers: "anthropic" (Claude, the default; billed to
	// the user's Anthropic API account) or "ollama" (a model running
	// locally in Ollama; free and nothing leaves the computer).
	Provider string `yaml:"provider,omitempty"`
	// Model is the model asked. Empty means the built-in default for
	// Anthropic; Ollama needs one the user has pulled.
	Model string `yaml:"model,omitempty"`
	// MaxOutputTokens caps each answer's length, and so its cost. Empty
	// means DefaultAIMaxOutputTokens.
	MaxOutputTokens int `yaml:"max_output_tokens,omitempty"`
	// OllamaURL is where Ollama listens. Empty means DefaultOllamaURL.
	OllamaURL string `yaml:"ollama_url,omitempty"`
}

// AI providers.
const (
	AIProviderAnthropic = "anthropic"
	AIProviderOllama    = "ollama"
)

// AI defaults and limits.
const (
	DefaultAIMaxOutputTokens = 4096
	// MinAIMaxOutputTokens leaves room for the model's reasoning, which
	// counts towards the limit on current Claude models.
	MinAIMaxOutputTokens = 1024
	// MaxAIMaxOutputTokens keeps a single request under the SDK's HTTP
	// timeout without streaming.
	MaxAIMaxOutputTokens = 16000
	DefaultOllamaURL     = "http://localhost:11434"
)

// AIProviderName is the configured provider, defaulting to Anthropic.
func (a AIConfig) AIProviderName() string {
	if a.Provider == "" {
		return AIProviderAnthropic
	}
	return a.Provider
}

// OutputTokens is the configured answer limit, or the default.
func (a AIConfig) OutputTokens() int {
	if a.MaxOutputTokens == 0 {
		return DefaultAIMaxOutputTokens
	}
	return a.MaxOutputTokens
}

// OllamaEndpoint is the configured Ollama address, or the default.
func (a AIConfig) OllamaEndpoint() string {
	if a.OllamaURL == "" {
		return DefaultOllamaURL
	}
	return a.OllamaURL
}

// UIConfig holds presentation preferences.
type UIConfig struct {
	Theme Theme `yaml:"theme"`
	Mode  Mode  `yaml:"mode"`
	// ShowHints controls the keyboard hint row in the status bar.
	ShowHints bool `yaml:"show_hints"`
	// ConfirmActions keeps the action preview on for every consequential
	// action. Phase 1 never performs actions, but the preference exists so
	// later phases cannot forget it.
	ConfirmActions bool `yaml:"confirm_actions"`
}

// LogConfig holds logging preferences.
type LogConfig struct {
	Level LogLevel `yaml:"level"`
}

// Default returns the built-in defaults: dark theme, Simple mode, hints on,
// confirmations on, info-level logging.
func Default() Config {
	return Config{
		Version: CurrentVersion,
		UI: UIConfig{
			Theme:          ThemeDark,
			Mode:           ModeSimple,
			ShowHints:      true,
			ConfirmActions: true,
		},
		Logging: LogConfig{Level: LogInfo},
	}
}

// LoadError describes a configuration file problem that was recovered from.
// Load returns it alongside a usable Config so the app always starts.
type LoadError struct {
	Path string
	// Summary is a plain-language sentence suitable for the status bar.
	Summary string
	// Details holds the underlying error text for Advanced mode.
	Details string
	// Malformed is true when the file exists but could not be parsed. Save
	// uses this to back the file up before overwriting it.
	Malformed bool
}

func (e *LoadError) Error() string { return e.Summary + ": " + e.Details }

// Result is what Load returns: the effective configuration plus metadata.
type Result struct {
	Config Config
	Path   string
	// Found is false when no file existed and defaults were used.
	Found bool
	// Warnings lists per-field values that were ignored and replaced by
	// defaults (e.g. an unknown theme name). Plain language.
	Warnings []string
	// Err is non-nil when the file existed but could not be read or parsed.
	Err *LoadError
}

// Load reads the configuration at path. It never returns an unusable Config.
func Load(path string) Result {
	res := Result{Config: Default(), Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return res
		}
		res.Err = &LoadError{
			Path:    path,
			Summary: "Your settings file couldn't be read, so default settings are in use",
			Details: err.Error(),
		}
		return res
	}
	res.Found = true

	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		res.Err = &LoadError{
			Path:      path,
			Summary:   "Your settings file has a formatting problem, so default settings are in use",
			Details:   err.Error(),
			Malformed: true,
		}
		return res
	}

	res.Config, res.Warnings = merge(Default(), raw, data)
	return res
}

// merge overlays raw onto base, validating each field. Fields missing from
// the file keep their defaults; invalid values produce a warning and keep
// the default.
func merge(base, raw Config, data []byte) (Config, []string) {
	var warnings []string
	out := base

	if raw.Version != 0 {
		out.Version = raw.Version
	}
	if raw.Version > CurrentVersion {
		warnings = append(warnings, fmt.Sprintf("Settings file is version %d but this build understands version %d; unknown options are ignored", raw.Version, CurrentVersion))
		out.Version = CurrentVersion
	}

	switch raw.UI.Theme {
	case "":
	case ThemeDark, ThemeLight:
		out.UI.Theme = raw.UI.Theme
	default:
		warnings = append(warnings, fmt.Sprintf("Unknown theme %q ignored; using %s", raw.UI.Theme, base.UI.Theme))
	}

	switch raw.UI.Mode {
	case "":
	case ModeSimple, ModeAdvanced:
		out.UI.Mode = raw.UI.Mode
	default:
		warnings = append(warnings, fmt.Sprintf("Unknown mode %q ignored; using %s", raw.UI.Mode, base.UI.Mode))
	}

	out.AI, warnings = validateAI(raw.AI, warnings)

	switch raw.Logging.Level {
	case "":
	case LogDebug, LogInfo, LogWarn, LogError:
		out.Logging.Level = raw.Logging.Level
	default:
		warnings = append(warnings, fmt.Sprintf("Unknown log level %q ignored; using %s", raw.Logging.Level, base.Logging.Level))
	}

	// Booleans cannot distinguish "false" from "absent" after Unmarshal, so
	// look at the raw document for the keys that default to true.
	keys := presentKeys(data)
	if keys["ui.show_hints"] {
		out.UI.ShowHints = raw.UI.ShowHints
	}
	if keys["ui.confirm_actions"] {
		out.UI.ConfirmActions = raw.UI.ConfirmActions
	}

	return out, warnings
}

// reModel accepts Claude model IDs such as claude-opus-5-5.
var reModel = regexp.MustCompile(`^claude-[a-z0-9.-]+$`)

// reOllamaModel accepts Ollama model names such as llama3.2:3b or
// library/qwen2.5:7b-instruct.
var reOllamaModel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)

func validateAI(raw AIConfig, warnings []string) (AIConfig, []string) {
	out := AIConfig{Enabled: raw.Enabled}
	switch raw.Provider {
	case "", AIProviderAnthropic, AIProviderOllama:
		out.Provider = raw.Provider
	default:
		warnings = append(warnings, fmt.Sprintf("Unknown AI provider %q ignored; using %s", raw.Provider, AIProviderAnthropic))
	}
	if raw.Model != "" {
		switch {
		case out.AIProviderName() == AIProviderOllama && reOllamaModel.MatchString(raw.Model):
			out.Model = raw.Model
		case out.AIProviderName() == AIProviderAnthropic && reModel.MatchString(raw.Model):
			out.Model = raw.Model
		case out.AIProviderName() == AIProviderOllama:
			warnings = append(warnings, fmt.Sprintf("AI model name %q doesn't look like an Ollama model name; ignored", raw.Model))
		default:
			warnings = append(warnings, fmt.Sprintf("AI model name %q doesn't look like a Claude model ID; using the default", raw.Model))
		}
	}
	switch n := raw.MaxOutputTokens; {
	case n == 0:
	case n < MinAIMaxOutputTokens:
		out.MaxOutputTokens = MinAIMaxOutputTokens
		warnings = append(warnings, fmt.Sprintf("AI answer limit %d is too small (the model's reasoning counts towards it); using %d", n, MinAIMaxOutputTokens))
	case n > MaxAIMaxOutputTokens:
		out.MaxOutputTokens = MaxAIMaxOutputTokens
		warnings = append(warnings, fmt.Sprintf("AI answer limit %d is more than %d; using %d", n, MaxAIMaxOutputTokens, MaxAIMaxOutputTokens))
	default:
		out.MaxOutputTokens = n
	}
	if raw.OllamaURL != "" {
		if u, err := url.Parse(raw.OllamaURL); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil {
			out.OllamaURL = strings.TrimRight(raw.OllamaURL, "/")
		} else {
			warnings = append(warnings, fmt.Sprintf("Ollama address %q isn't a web address like %s; using that", raw.OllamaURL, DefaultOllamaURL))
		}
	}
	return out, warnings
}

// presentKeys reports which dotted keys exist in the YAML document.
func presentKeys(data []byte) map[string]bool {
	keys := map[string]bool{}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return keys
	}
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		for k, child := range m {
			full := k
			if prefix != "" {
				full = prefix + "." + k
			}
			keys[full] = true
			walk(full, child)
		}
	}
	walk("", doc)
	return keys
}

// Save writes cfg to path, creating parent directories. If backupFirst is
// true and a file already exists, it is copied to path+".bak" before being
// replaced. The write is atomic (temp file + rename).
func Save(path string, cfg Config, backupFirst bool) error {
	cfg.Version = CurrentVersion
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings folder: %w", err)
	}

	if backupFirst {
		if old, err := os.ReadFile(path); err == nil {
			if err := os.WriteFile(path+".bak", old, 0o644); err != nil {
				return fmt.Errorf("back up previous settings: %w", err)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("# ATLAS settings. Edit by hand or change them in the app under Settings.\n")
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(sb.String()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace settings file: %w", err)
	}
	return nil
}
