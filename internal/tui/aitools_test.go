package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/aitools"
	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bootAITools starts ATLAS with the real AI Tools service reading a fake
// home folder.
func bootAITools(t *testing.T) (Model, aitools.Env, *database.DB) {
	t.Helper()
	root := t.TempDir()
	env := aitools.Env{Home: filepath.Join(root, "home"), ConfigDir: filepath.Join(root, "cfg"), DataDir: filepath.Join(root, "atlas")}
	writeTestFile(t, filepath.Join(env.Home, ".cursor", "mcp.json"), `{"mcpServers": {"browser": {"command": "browser-mcp"}, "github": {"command": "gh-mcp", "env": {"TOKEN": "secret"}}}}`)
	writeTestFile(t, filepath.Join(env.Home, ".claude", "settings.json"), `{"enabledPlugins": {"formatter@tools": true}}`)
	writeTestFile(t, filepath.Join(env.Home, ".claude", "plugins", "installed_plugins.json"), `{"version": 2, "plugins": {"formatter@tools": [{"installPath": "/none", "version": "1.0.0"}]}}`)
	writeTestFile(t, filepath.Join(env.Home, ".claude", "skills", "pdf", "SKILL.md"), "---\nname: pdf\ndescription: Read PDF files\n---\n")
	writeTestFile(t, filepath.Join(env.Home, ".codex", "config.toml"), "[mcp_servers.fs]\ncommand = \"fs-mcp\"\n")

	db, err := database.Open(context.Background(), filepath.Join(root, "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := New(Options{ConfigResult: config.Load(filepath.Join(root, "c.yaml")), Extensions: aitools.NewService(env, db, nil)})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return drain(t, next.(Model), m.Init()), env, db
}

func TestAIToolsListsAndManages(t *testing.T) {
	m, env, db := bootAITools(t)
	m = press(t, m, "0")
	if m.Section() != nav.AITools {
		t.Fatalf("0 should open AI Tools, got %v", m.Section())
	}
	v := m.View()
	for _, want := range []string{"AI Tools", "3 MCP servers · 1 plugin · 1 skill in 3 apps", "MCP servers (3)", "browser", "Cursor", "Plugins (1)", "formatter", "Skills (1)", "pdf", "r] remove"} {
		if !strings.Contains(v, want) {
			t.Errorf("AI Tools missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "secret") {
		t.Error("an MCP server's environment must never be shown")
	}

	// Remove the highlighted server: previewed, and Enter alone doesn't confirm.
	m = press(t, m, "r")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Remove the MCP server browser") || !strings.Contains(m.View(), "A copy of the file as it was is saved next to it first") {
		t.Fatalf("remove should open a preview:\n%s", m.View())
	}
	m = press(t, m, "enter")
	data, _ := os.ReadFile(filepath.Join(env.Home, ".cursor", "mcp.json"))
	if !strings.Contains(string(data), "browser-mcp") {
		t.Fatal("Enter must not confirm a removal")
	}
	if !m.PreviewOpen() {
		m = press(t, m, "r")
	}
	m = press(t, m, "r")
	data, _ = os.ReadFile(filepath.Join(env.Home, ".cursor", "mcp.json"))
	if strings.Contains(string(data), "browser-mcp") || !strings.Contains(string(data), "gh-mcp") {
		t.Fatalf("browser should be removed, github kept:\n%s", data)
	}
	if st := m.Status(); !strings.Contains(st.Text, "Removed the MCP server browser from Cursor") {
		t.Errorf("status = %+v", st)
	}
	if v := m.View(); strings.Contains(v, "● browser") || !strings.Contains(v, "MCP servers (2)") {
		t.Errorf("the list should be read again:\n%s", v)
	}
	acts, _ := db.RecentActivity(context.Background(), 10)
	if len(acts) == 0 || acts[0].Kind != "extension.remove" || acts[0].Outcome != core.OutcomeSuccess {
		t.Errorf("activity = %+v", acts)
	}

	// Plugins: space previews turning it off; Enter confirms (not destructive).
	m = press(t, m, "right")
	m = press(t, m, "right")
	if v := m.View(); !strings.Contains(v, "[Plugins]") || !strings.Contains(v, "space] turn off") {
		t.Fatalf("plugins tab:\n%s", v)
	}
	m = press(t, m, " ")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Turn the plugin formatter off") {
		t.Fatalf("toggle should preview:\n%s", m.View())
	}
	m = press(t, m, "enter")
	settings, _ := os.ReadFile(filepath.Join(env.Home, ".claude", "settings.json"))
	if !strings.Contains(string(settings), `"formatter@tools": false`) {
		t.Errorf("settings:\n%s", settings)
	}
	if v := m.View(); !strings.Contains(v, "· off") || !strings.Contains(m.Status().Text, "Restart Claude Code") {
		t.Errorf("after turning off:\n%s\n%+v", v, m.Status())
	}

	// Codex's TOML isn't edited: the preview says how to do it instead.
	m = press(t, m, "left")
	m = press(t, m, "up")
	for i := 0; i < 5 && !strings.Contains(m.View(), "Codex keeps its settings in TOML"); i++ {
		m = press(t, m, "down")
	}
	m = press(t, m, "r")
	if v := m.View(); !strings.Contains(v, "Not available") || !strings.Contains(v, "codex mcp remove fs") {
		t.Errorf("codex removal should be explained, not done:\n%s", v)
	}
	m = press(t, m, "r")
	if _, err := os.Stat(filepath.Join(env.Home, ".codex", "config.toml")); err != nil {
		t.Error("config.toml must be untouched")
	}
}

func TestAIToolsFitsEverySize(t *testing.T) {
	m, _, _ := bootAITools(t)
	m = press(t, m, "0")
	for _, sz := range [][2]int{{60, 20}, {80, 24}, {100, 30}, {120, 30}, {160, 45}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		for i, l := range strings.Split(next.(Model).View(), "\n") {
			if w := lipgloss.Width(l); w > sz[0] {
				t.Errorf("%dx%d line %d is %d wide", sz[0], sz[1], i, w)
			}
		}
	}
}

func TestAIToolsWithNothingInstalled(t *testing.T) {
	root := t.TempDir()
	m := New(Options{ConfigResult: config.Load(filepath.Join(root, "c.yaml")), Extensions: aitools.NewService(aitools.Env{Home: root}, nil, nil)})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drain(t, next.(Model), m.Init())
	m = press(t, m, "0")
	if v := m.View(); !strings.Contains(v, "No MCP servers, plugins or skills found") || !strings.Contains(v, "Gemini CLI and Codex") {
		t.Errorf("empty state:\n%s", v)
	}
}
