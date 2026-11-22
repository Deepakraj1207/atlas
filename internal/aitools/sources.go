package aitools

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Env says where to look. Tests point it at a temporary folder.
type Env struct {
	// Home is the user's home folder.
	Home string
	// ConfigDir is %APPDATA% on Windows and ~/.config on Linux.
	ConfigDir string
	// LocalAppData is %LOCALAPPDATA% on Windows; empty elsewhere.
	LocalAppData string
	// DataDir is ATLAS's own data folder; removed skills are kept there.
	DataDir string
}

// DefaultEnv reads the folders for the current user.
func DefaultEnv(dataDir string) Env {
	home, _ := os.UserHomeDir()
	cfg, _ := os.UserConfigDir()
	return Env{Home: home, ConfigDir: cfg, LocalAppData: os.Getenv("LOCALAPPDATA"), DataDir: dataDir}
}

// reader collects what one read finds.
type reader struct {
	env     Env
	exts    []core.Extension
	sources []core.ExtensionSource
}

func (r *reader) add(e core.Extension) {
	e.ID = string(e.Kind) + "|" + e.App + "|" + e.Scope + "|" + e.Name
	r.exts = append(r.exts, e)
}

// source records a file or folder that was looked at. Missing ones are
// left out: most people don't have every app.
func (r *reader) source(app, path string, found int, err error) {
	st := core.ExtensionSource{App: app, Path: path, Status: core.SourceOK, Found: found}
	if err != nil {
		st.Status, st.Details = core.SourceFailed, err.Error()
	}
	r.sources = append(r.sources, st)
}

// readAll looks in every place ATLAS knows about.
func (r *reader) readAll() {
	home, cfg := r.env.Home, r.env.ConfigDir
	if home != "" {
		r.claudeCode(home)
		r.mcpFile("Cursor", filepath.Join(home, ".cursor", "mcp.json"), "mcpServers", true)
		r.mcpFile("Windsurf", filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"), "mcpServers", true)
		r.mcpFile("Gemini CLI", filepath.Join(home, ".gemini", "settings.json"), "mcpServers", true)
		r.geminiExtensions(filepath.Join(home, ".gemini", "extensions"))
		r.codexConfig(filepath.Join(home, ".codex", "config.toml"))
		r.skillDir("Codex", filepath.Join(home, ".codex", "skills"), "user", true)
	}
	if cfg != "" {
		r.mcpFile("Claude Desktop", filepath.Join(cfg, "Claude", "claude_desktop_config.json"), "mcpServers", true)
		r.mcpFile("VS Code", filepath.Join(cfg, "Code", "User", "mcp.json"), "servers", true)
		r.mcpFile("VS Code Insiders", filepath.Join(cfg, "Code - Insiders", "User", "mcp.json"), "servers", true)
	}
	if r.env.LocalAppData != "" {
		// Claude Desktop from the Microsoft Store keeps its settings here.
		matches, _ := filepath.Glob(filepath.Join(r.env.LocalAppData, "Packages", "Claude_*", "LocalCache", "Roaming", "Claude", "claude_desktop_config.json"))
		for _, m := range matches {
			r.mcpFile("Claude Desktop", m, "mcpServers", true)
		}
	}
	sort.SliceStable(r.exts, func(i, j int) bool {
		a, b := r.exts[i], r.exts[j]
		if a.Kind != b.Kind {
			return kindOrder(a.Kind) < kindOrder(b.Kind)
		}
		if !strings.EqualFold(a.Name, b.Name) {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		return a.App < b.App
	})
}

func kindOrder(k core.ExtensionKind) int {
	switch k {
	case core.ExtMCP:
		return 0
	case core.ExtPlugin:
		return 1
	}
	return 2
}

// readJSON reads a JSON (or JSON-with-comments) file. exists is false when
// there is no file.
func readJSON(path string) (n *node, raw []byte, exists bool, err error) {
	raw, err = os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, true, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return &node{kind: 'o'}, raw, true, nil
	}
	n, err = parseJSON(raw)
	if err != nil {
		if n2, err2 := parseJSON(stripJSONC(raw)); err2 == nil {
			return n2, raw, true, nil
		}
		return nil, raw, true, errors.New("couldn't read it as JSON: " + err.Error())
	}
	return n, raw, true, nil
}

// editable says whether ATLAS can rewrite a settings file without losing
// anything: a file with comments would lose them, so it is left alone.
func editable(app string, raw []byte, removable bool) (bool, string) {
	if removable && !isStrictJSON(raw) {
		return false, "This settings file has comments, which ATLAS would lose. Remove the server in " + app + " instead."
	}
	return removable, ""
}

// serverDetail describes an MCP server by its command or address. The
// environment and headers are skipped: they often hold keys.
func serverDetail(s *node) string {
	if u := s.get("url").str(); u != "" {
		return u
	}
	if u := s.get("serverUrl").str(); u != "" {
		return u
	}
	parts := []string{s.get("command").str()}
	if args := s.get("args"); args != nil && args.kind == 'a' {
		for _, a := range args.items {
			parts = append(parts, a.str())
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// servers adds every MCP server in obj. path is obj's key path in the file.
func (r *reader) servers(app, file, scope string, obj *node, path []string, removable bool, note string) int {
	if obj == nil || obj.kind != 'o' {
		return 0
	}
	for _, m := range obj.members {
		enabled := true
		if d, ok := m.val.get("disabled").boolean(); ok && d {
			enabled = false
		}
		if d, ok := m.val.get("enabled").boolean(); ok && !d {
			enabled = false
		}
		r.add(core.Extension{
			Kind: core.ExtMCP, Name: m.key, App: app, Scope: scope,
			Detail: serverDetail(m.val), Path: file,
			Locator: append(append([]string{}, path...), m.key),
			Enabled: enabled, CanRemove: removable, Note: note,
		})
	}
	return len(obj.members)
}

func (r *reader) mcpFile(app, path, key string, removable bool) {
	n, raw, exists, err := readJSON(path)
	if !exists {
		return
	}
	found := 0
	if err == nil {
		removable, note := editable(app, raw, removable)
		found = r.servers(app, path, "user", n.get(key), []string{key}, removable, note)
	}
	r.source(app, path, found, err)
}

// claudeCode reads Claude Code's MCP servers, plugins and skills.
func (r *reader) claudeCode(home string) {
	const app = "Claude Code"
	cfg := filepath.Join(home, ".claude.json")
	if n, raw, exists, err := readJSON(cfg); exists {
		found := 0
		if err == nil {
			ok, note := editable(app, raw, true)
			found += r.servers(app, cfg, "user", n.get("mcpServers"), []string{"mcpServers"}, ok, note)
			if projects := n.get("projects"); projects != nil && projects.kind == 'o' {
				for _, p := range projects.members {
					found += r.servers(app, cfg, "project "+p.key, p.val.get("mcpServers"), []string{"projects", p.key, "mcpServers"}, ok, note)
				}
			}
		}
		r.source(app, cfg, found, err)
	}

	dir := filepath.Join(home, ".claude")
	r.skillDir(app, filepath.Join(dir, "skills"), "user", true)

	settingsPath := filepath.Join(dir, "settings.json")
	settings, _, _, _ := readJSON(settingsPath)
	enabledPlugins := settings.get("enabledPlugins")

	installed := filepath.Join(dir, "plugins", "installed_plugins.json")
	n, _, exists, err := readJSON(installed)
	if !exists {
		return
	}
	found := 0
	if plugins := n.get("plugins"); err == nil && plugins != nil && plugins.kind == 'o' {
		for _, p := range plugins.members {
			// Version 2 keeps a list of installs per plugin, version 1 one.
			inst := p.val
			if inst.kind == 'a' && len(inst.items) > 0 {
				inst = inst.items[0]
			}
			name, market, _ := strings.Cut(p.key, "@")
			on, _ := enabledPlugins.get(p.key).boolean()
			detail := pluginDescription(inst.get("installPath").str())
			if detail == "" && market != "" {
				detail = "From " + market
			}
			if v := inst.get("version").str(); v != "" && v != "unknown" {
				detail += " · version " + v
			}
			r.add(core.Extension{
				Kind: core.ExtPlugin, Name: name, App: app, Scope: "user",
				Detail: strings.TrimPrefix(detail, " · "), Path: settingsPath, Locator: []string{p.key},
				Enabled: on, CanToggle: true,
				Note: "To uninstall it, run: claude plugin uninstall " + p.key,
			})
			found++
			// What the plugin brings with it is managed through the plugin.
			if ip := inst.get("installPath").str(); ip != "" {
				scope := "plugin " + name
				r.skillDir(app, filepath.Join(ip, "skills"), scope, false)
				if m, _, ok, err := readJSON(filepath.Join(ip, ".mcp.json")); ok && err == nil {
					servers := m.get("mcpServers")
					if servers == nil {
						servers = m
					}
					r.servers(app, filepath.Join(ip, ".mcp.json"), scope, servers, nil, false, "Comes with the plugin "+name+"; turn the plugin off to stop it.")
				}
			}
		}
	}
	r.source(app, installed, found, err)
}

// pluginDescription reads the description from a plugin's manifest.
func pluginDescription(installPath string) string {
	if installPath == "" {
		return ""
	}
	for _, f := range []string{filepath.Join(".claude-plugin", "plugin.json"), "plugin.json"} {
		if n, _, ok, err := readJSON(filepath.Join(installPath, f)); ok && err == nil {
			return n.get("description").str()
		}
	}
	return ""
}

// skillDir adds every <dir>/<name>/SKILL.md.
func (r *reader) skillDir(app, dir, scope string, removable bool) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() && e.Type()&fs.ModeSymlink == 0 {
			continue
		}
		folder := filepath.Join(dir, e.Name())
		name, desc, ok := readSkill(filepath.Join(folder, "SKILL.md"))
		if !ok {
			continue
		}
		if name == "" {
			name = e.Name()
		}
		note := ""
		if !removable {
			note = "Comes with the " + strings.TrimPrefix(scope, "plugin ") + " plugin; turn the plugin off instead."
		}
		r.add(core.Extension{
			Kind: core.ExtSkill, Name: name, App: app, Scope: scope, Detail: desc,
			Path: folder, Enabled: true, CanRemove: removable, Note: note,
		})
		found++
	}
	if scope == "user" || err != nil {
		r.source(app, dir, found, err)
	}
}

// readSkill reads name and description from SKILL.md's front matter.
func readSkill(path string) (name, desc string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	inFront := false
	for i := 0; sc.Scan() && i < 200; i++ {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "---" {
			if inFront {
				break
			}
			if i == 0 {
				inFront = true
				continue
			}
		}
		if !inFront {
			break
		}
		k, v, found := strings.Cut(line, ":")
		if !found || strings.HasPrefix(k, " ") {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch strings.TrimSpace(k) {
		case "name":
			name = v
		case "description":
			desc = v
		}
	}
	return name, desc, true
}

// geminiExtensions lists Gemini CLI extensions.
func (r *reader) geminiExtensions(dir string) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest := filepath.Join(dir, e.Name(), "gemini-extension.json")
		n, _, ok, merr := readJSON(manifest)
		if !ok || merr != nil {
			continue
		}
		name := n.get("name").str()
		if name == "" {
			name = e.Name()
		}
		detail := n.get("description").str()
		if v := n.get("version").str(); v != "" {
			detail = strings.TrimPrefix(detail+" · version "+v, " · ")
		}
		r.add(core.Extension{
			Kind: core.ExtPlugin, Name: name, App: "Gemini CLI", Scope: "user", Detail: detail,
			Path: filepath.Join(dir, e.Name()), Enabled: true,
			Note: "To manage it, run: gemini extensions disable " + name + " (or uninstall)",
		})
		found++
	}
	r.source("Gemini CLI", dir, found, err)
}

// codexConfig lists the [mcp_servers.NAME] tables in Codex's config.toml.
// ATLAS only reads TOML; it doesn't edit it.
func (r *reader) codexConfig(path string) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	found := 0
	if err == nil {
		var cur *core.Extension
		var args string
		flush := func() {
			if cur != nil {
				cur.Detail = strings.TrimSpace(cur.Detail + " " + args)
				r.add(*cur)
				found++
			}
			cur, args = nil, ""
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[") {
				flush()
				header := strings.Trim(line, "[] ")
				if name, ok := strings.CutPrefix(header, "mcp_servers."); ok && !strings.Contains(name, ".") {
					cur = &core.Extension{
						Kind: core.ExtMCP, Name: strings.Trim(name, `"`), App: "Codex", Scope: "user",
						Path: path, Enabled: true,
						Note: "Codex keeps its settings in TOML, which ATLAS doesn't edit. Remove it with: codex mcp remove " + strings.Trim(name, `"`),
					}
				}
				continue
			}
			if cur == nil {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch strings.TrimSpace(k) {
			case "command", "url":
				cur.Detail = strings.Trim(v, `"`)
			case "args":
				args = strings.NewReplacer(`"`, "", "[", "", "]", "", ",", "").Replace(v)
			case "enabled":
				cur.Enabled = v != "false"
			}
		}
		flush()
	}
	r.source("Codex", path, found, err)
}
