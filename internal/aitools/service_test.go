package aitools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

type recorder struct{ acts []core.Activity }

func (r *recorder) AddActivity(_ context.Context, a core.Activity) (int64, error) {
	r.acts = append(r.acts, a)
	return int64(len(r.acts)), nil
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture builds a home folder with every app ATLAS knows about.
func fixture(t *testing.T) Env {
	t.Helper()
	root := t.TempDir()
	env := Env{Home: filepath.Join(root, "home"), ConfigDir: filepath.Join(root, "config"), LocalAppData: filepath.Join(root, "local"), DataDir: filepath.Join(root, "atlas")}
	h := env.Home
	write(t, filepath.Join(h, ".claude.json"), `{
  "numStartups": 12,
  "mcpServers": {
    "github": {"type": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"], "env": {"GITHUB_TOKEN": "secret-token"}},
    "docs": {"type": "http", "url": "https://example.com/mcp", "headers": {"Authorization": "Bearer secret"}}
  },
  "projects": {
    "/work/app": {"mcpServers": {"db": {"command": "db-mcp"}}, "allowedTools": []}
  }
}`)
	write(t, filepath.Join(h, ".claude", "settings.json"), `{
  "model": "opus",
  "enabledPlugins": {"formatter@tools": true, "old@tools": false}
}`)
	plug := filepath.Join(h, ".claude", "plugins", "cache", "formatter")
	write(t, filepath.Join(h, ".claude", "plugins", "installed_plugins.json"), `{"version": 2, "plugins": {
  "formatter@tools": [{"scope": "user", "installPath": `+quote(plug)+`, "version": "1.2.0"}],
  "old@tools": [{"scope": "user", "installPath": "/nowhere", "version": "unknown"}]
}}`)
	write(t, filepath.Join(plug, ".claude-plugin", "plugin.json"), `{"name": "formatter", "description": "Formats code on save"}`)
	write(t, filepath.Join(plug, "skills", "fmt", "SKILL.md"), "---\nname: fmt\ndescription: Format files\n---\nBody")
	write(t, filepath.Join(plug, ".mcp.json"), `{"mcpServers": {"fmt-server": {"command": "fmt-mcp"}}}`)
	write(t, filepath.Join(h, ".claude", "skills", "pdf", "SKILL.md"), "---\nname: pdf\ndescription: \"Read and fill PDF forms\"\n---\n# PDF")
	write(t, filepath.Join(h, ".claude", "skills", "notes", "README.md"), "not a skill")
	write(t, filepath.Join(h, ".cursor", "mcp.json"), `{"mcpServers": {"browser": {"command": "browser-mcp", "disabled": true}}}`)
	write(t, filepath.Join(h, ".codeium", "windsurf", "mcp_config.json"), `{"mcpServers": {}}`)
	write(t, filepath.Join(h, ".gemini", "settings.json"), `{"theme": "dark", "mcpServers": {"search": {"httpUrl": "x", "url": "https://search.example/mcp"}}}`)
	write(t, filepath.Join(h, ".gemini", "extensions", "security", "gemini-extension.json"), `{"name": "security", "version": "0.3.0", "description": "Security checks"}`)
	write(t, filepath.Join(h, ".codex", "config.toml"), "model = \"o4\"\n\n[mcp_servers.fs]\ncommand = \"fs-mcp\"\nargs = [\"--root\", \"/tmp\"]\n\n[mcp_servers.fs.env]\nKEY = \"secret\"\n")
	write(t, filepath.Join(env.ConfigDir, "Claude", "claude_desktop_config.json"), `{"mcpServers": {"files": {"command": "files-mcp"}}}`)
	write(t, filepath.Join(env.ConfigDir, "Code", "User", "mcp.json"), "{\n  // my servers\n  \"servers\": {\n    \"playwright\": {\"command\": \"npx\", \"args\": [\"@playwright/mcp\"]},\n  },\n}\n")
	write(t, filepath.Join(env.LocalAppData, "Packages", "Claude_pzs8sxrjxfjjc", "LocalCache", "Roaming", "Claude", "claude_desktop_config.json"), `{"mcpServers": {"store": {"command": "store-mcp"}}}`)
	return env
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

func find(t *testing.T, r core.ExtensionReport, kind core.ExtensionKind, app, name string) core.Extension {
	t.Helper()
	for _, e := range r.Extensions {
		if e.Kind == kind && e.App == app && e.Name == name {
			return e
		}
	}
	t.Fatalf("%s %q in %s not found among %d", kind, name, app, len(r.Extensions))
	return core.Extension{}
}

func TestListsEveryAppsExtensions(t *testing.T) {
	env := fixture(t)
	s := NewService(env, nil, nil)
	r, err := s.ListExtensions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	gh := find(t, r, core.ExtMCP, "Claude Code", "github")
	if gh.Detail != "npx -y @modelcontextprotocol/server-github" || !gh.CanRemove || gh.Scope != "user" {
		t.Errorf("github = %+v", gh)
	}
	if d := find(t, r, core.ExtMCP, "Claude Code", "docs"); d.Detail != "https://example.com/mcp" {
		t.Errorf("docs = %+v", d)
	}
	if db := find(t, r, core.ExtMCP, "Claude Code", "db"); db.Scope != "project /work/app" || strings.Join(db.Locator, "/") != "projects//work/app/mcpServers/db" {
		t.Errorf("db = %+v", db)
	}
	f := find(t, r, core.ExtPlugin, "Claude Code", "formatter")
	if !f.Enabled || !f.CanToggle || f.Detail != "Formats code on save · version 1.2.0" {
		t.Errorf("formatter = %+v", f)
	}
	if o := find(t, r, core.ExtPlugin, "Claude Code", "old"); o.Enabled || o.Detail != "From tools" {
		t.Errorf("old = %+v", o)
	}
	if fs := find(t, r, core.ExtSkill, "Claude Code", "fmt"); fs.CanRemove || fs.Scope != "plugin formatter" || fs.Note == "" {
		t.Errorf("plugin skill = %+v", fs)
	}
	if fm := find(t, r, core.ExtMCP, "Claude Code", "fmt-server"); fm.CanRemove {
		t.Errorf("plugin server must be managed through its plugin: %+v", fm)
	}
	if p := find(t, r, core.ExtSkill, "Claude Code", "pdf"); p.Detail != "Read and fill PDF forms" || !p.CanRemove {
		t.Errorf("pdf = %+v", p)
	}
	if b := find(t, r, core.ExtMCP, "Cursor", "browser"); b.Enabled {
		t.Errorf("disabled: true should show as off: %+v", b)
	}
	find(t, r, core.ExtMCP, "Gemini CLI", "search")
	if g := find(t, r, core.ExtPlugin, "Gemini CLI", "security"); g.CanToggle || g.CanRemove || !strings.Contains(g.Note, "gemini extensions") {
		t.Errorf("gemini extension = %+v", g)
	}
	if c := find(t, r, core.ExtMCP, "Codex", "fs"); c.Detail != "fs-mcp --root /tmp" || c.CanRemove {
		t.Errorf("codex = %+v", c)
	}
	find(t, r, core.ExtMCP, "Claude Desktop", "files")
	find(t, r, core.ExtMCP, "Claude Desktop", "store")
	if p := find(t, r, core.ExtMCP, "VS Code", "playwright"); p.CanRemove || !strings.Contains(p.Note, "comments") { // a file with comments and trailing commas
		t.Errorf("playwright = %+v", p)
	}

	for _, e := range r.Extensions {
		if strings.Contains(e.Detail, "secret") {
			t.Errorf("secrets must never be read into an extension: %+v", e)
		}
		if e.Kind == core.ExtSkill && e.Name == "notes" {
			t.Error("a folder without SKILL.md is not a skill")
		}
	}
	// MCP servers first, then plugins, then skills.
	last := -1
	for _, e := range r.Extensions {
		if k := kindOrder(e.Kind); k < last {
			t.Fatalf("not grouped by kind at %s", e.Name)
		} else {
			last = k
		}
	}
	for _, src := range r.Sources {
		if src.Status != core.SourceOK {
			t.Errorf("source %+v", src)
		}
	}
}

func TestNothingInstalledIsNotAnError(t *testing.T) {
	s := NewService(Env{Home: t.TempDir(), ConfigDir: t.TempDir()}, nil, nil)
	r, err := s.ListExtensions(context.Background())
	if err != nil || len(r.Extensions) != 0 || len(r.Sources) != 0 {
		t.Errorf("empty home: %+v %v", r, err)
	}
}

func TestBrokenConfigIsReportedNotFatal(t *testing.T) {
	env := Env{Home: t.TempDir()}
	write(t, filepath.Join(env.Home, ".cursor", "mcp.json"), `{"mcpServers": {`)
	r, _ := NewService(env, nil, nil).ListExtensions(context.Background())
	if len(r.Sources) != 1 || r.Sources[0].Status != core.SourceFailed || r.Sources[0].App != "Cursor" {
		t.Errorf("sources = %+v", r.Sources)
	}
}

func TestRemoveServerKeepsTheRestOfTheFileAndABackup(t *testing.T) {
	env := fixture(t)
	rec := &recorder{}
	s := NewService(env, rec, nil)
	s.now = func() time.Time { return time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC) }
	r, _ := s.ListExtensions(context.Background())
	path := filepath.Join(env.Home, ".claude.json")
	before, _ := os.ReadFile(path)

	res, err := s.RemoveExtension(context.Background(), find(t, r, core.ExtMCP, "Claude Code", "github"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Backup != path+".atlas-backup-20261121-100000" || !strings.Contains(res.Summary, "Removed the MCP server github from Claude Code") {
		t.Errorf("result = %+v", res)
	}
	if b, _ := os.ReadFile(res.Backup); string(b) != string(before) {
		t.Error("the backup must be the file exactly as it was")
	}
	after, _ := os.ReadFile(path)
	got := string(after)
	if strings.Contains(got, "server-github") || strings.Contains(got, "GITHUB_TOKEN") || !strings.Contains(got, `"docs"`) {
		t.Errorf("only github should go:\n%s", got)
	}
	// Other keys stay, in their original order.
	if i, j, k := strings.Index(got, "numStartups"), strings.Index(got, "mcpServers"), strings.Index(got, "projects"); !(i < j && j < k) || i < 0 {
		t.Errorf("key order changed:\n%s", got)
	}
	if len(rec.acts) != 1 || rec.acts[0].Outcome != core.OutcomeSuccess || rec.acts[0].Kind != "extension.remove" || !strings.Contains(rec.acts[0].Details, res.Backup) {
		t.Errorf("activity = %+v", rec.acts)
	}

	// A project server is removed from its project only.
	r, _ = s.ListExtensions(context.Background())
	s.now = func() time.Time { return time.Date(2026, 11, 21, 10, 0, 1, 0, time.UTC) }
	if _, err := s.RemoveExtension(context.Background(), find(t, r, core.ExtMCP, "Claude Code", "db")); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if strings.Contains(string(after), "db-mcp") || !strings.Contains(string(after), "allowedTools") {
		t.Errorf("project server:\n%s", after)
	}
}

func TestRemoveRefusesWhatItCantDoSafely(t *testing.T) {
	env := fixture(t)
	rec := &recorder{}
	s := NewService(env, rec, nil)
	r, _ := s.ListExtensions(context.Background())
	vsPath := filepath.Join(env.ConfigDir, "Code", "User", "mcp.json")
	before, _ := os.ReadFile(vsPath)
	if _, err := s.RemoveExtension(context.Background(), find(t, r, core.ExtMCP, "VS Code", "playwright")); err == nil || !strings.Contains(err.Error(), "comments") {
		t.Errorf("a file with comments must not be rewritten: %v", err)
	}
	if after, _ := os.ReadFile(vsPath); string(after) != string(before) {
		t.Error("VS Code's file changed")
	}
	for _, e := range []core.Extension{find(t, r, core.ExtMCP, "Codex", "fs"), find(t, r, core.ExtSkill, "Claude Code", "fmt"), find(t, r, core.ExtPlugin, "Claude Code", "formatter")} {
		if _, err := s.RemoveExtension(context.Background(), e); err == nil {
			t.Errorf("%s %s should be refused", e.Kind, e.Name)
		}
	}
	for _, a := range rec.acts {
		if a.Outcome != core.OutcomeFailed || !strings.HasPrefix(a.Summary, "Not changed") {
			t.Errorf("a refusal is recorded as not changed: %+v", a)
		}
	}
}

func TestTogglePlugin(t *testing.T) {
	env := fixture(t)
	s := NewService(env, nil, nil)
	r, _ := s.ListExtensions(context.Background())
	res, err := s.SetExtensionEnabled(context.Background(), find(t, r, core.ExtPlugin, "Claude Code", "formatter"), false)
	if err != nil || !strings.Contains(res.Summary, "Turned the plugin formatter off") || res.Backup == "" {
		t.Fatalf("%+v %v", res, err)
	}
	r, _ = s.ListExtensions(context.Background())
	if find(t, r, core.ExtPlugin, "Claude Code", "formatter").Enabled {
		t.Error("formatter should be off now")
	}
	data, _ := os.ReadFile(filepath.Join(env.Home, ".claude", "settings.json"))
	if !strings.Contains(string(data), `"model": "opus"`) {
		t.Errorf("other settings must stay:\n%s", data)
	}
	if _, err := s.SetExtensionEnabled(context.Background(), find(t, r, core.ExtPlugin, "Claude Code", "formatter"), true); err != nil {
		t.Fatal(err)
	}
	r, _ = s.ListExtensions(context.Background())
	if !find(t, r, core.ExtPlugin, "Claude Code", "formatter").Enabled {
		t.Error("formatter should be on again")
	}
	if _, err := s.SetExtensionEnabled(context.Background(), find(t, r, core.ExtPlugin, "Gemini CLI", "security"), false); err == nil {
		t.Error("Gemini extensions are managed by Gemini CLI")
	}
}

func TestToggleCreatesSettingsWhenMissing(t *testing.T) {
	env := fixture(t)
	settings := filepath.Join(env.Home, ".claude", "settings.json")
	os.Remove(settings)
	s := NewService(env, nil, nil)
	r, _ := s.ListExtensions(context.Background())
	res, err := s.SetExtensionEnabled(context.Background(), find(t, r, core.ExtPlugin, "Claude Code", "old"), true)
	if err != nil || res.Backup != "" {
		t.Fatalf("%+v %v", res, err)
	}
	if data, _ := os.ReadFile(settings); !strings.Contains(string(data), `"old@tools": true`) {
		t.Errorf("settings:\n%s", data)
	}
}

func TestRemoveSkillMovesItAside(t *testing.T) {
	env := fixture(t)
	s := NewService(env, nil, nil)
	r, _ := s.ListExtensions(context.Background())
	pdf := find(t, r, core.ExtSkill, "Claude Code", "pdf")
	res, err := s.RemoveExtension(context.Background(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pdf.Path); !os.IsNotExist(err) {
		t.Error("the skill folder should be gone from Claude Code")
	}
	if b, err := os.ReadFile(filepath.Join(res.Backup, "SKILL.md")); err != nil || !strings.Contains(string(b), "# PDF") {
		t.Errorf("the skill must be kept in ATLAS's folder: %v", err)
	}
	if !strings.HasPrefix(res.Backup, filepath.Join(env.DataDir, "removed-skills")) || !strings.Contains(res.Summary, "move it back to restore it") {
		t.Errorf("result = %+v", res)
	}
}

func TestJSONEditRoundTrip(t *testing.T) {
	in := `{"b": 1, "a": {"x": [1, 2.50, "é <tag> & \"q\""], "e": {}}, "n": null, "t": true}`
	n, err := parseJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"b\": 1,\n  \"a\": {\n    \"x\": [\n      1,\n      2.50,\n      \"é <tag> & \\\"q\\\"\"\n    ],\n    \"e\": {}\n  },\n  \"n\": null,\n  \"t\": true\n}\n"
	if got := string(n.encode()); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := string(stripJSONC([]byte("{\"a\": \"// not a comment\", /* c */ \"b\": [1,],\n// x\n}"))); !strings.Contains(got, "// not a comment") || strings.Contains(got, "/*") {
		t.Errorf("stripJSONC = %q", got)
	}
}
