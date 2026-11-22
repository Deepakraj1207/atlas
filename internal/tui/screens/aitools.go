package screens

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// AITools lists the MCP servers, plugins and skills set up in the user's
// AI apps, and manages the ones ATLAS can change safely. Every change is
// previewed first.
type AITools struct {
	cursor int
	kind   int // index into aiKinds
}

// aiKinds are the filter tabs; "" is all of them.
var aiKinds = []core.ExtensionKind{"", core.ExtMCP, core.ExtPlugin, core.ExtSkill}

// NewAITools creates the AI Tools screen.
func NewAITools() *AITools { return &AITools{} }

// SelfScrolling keeps the highlighted entry in view.
func (a *AITools) SelfScrolling() bool { return true }

// Cursor is the highlighted row (for tests).
func (a *AITools) Cursor() int { return a.cursor }

func aiKindLabel(k core.ExtensionKind, plural bool) string {
	switch k {
	case core.ExtMCP:
		return map[bool]string{true: "MCP servers", false: "MCP server"}[plural]
	case core.ExtPlugin:
		return map[bool]string{true: "Plugins", false: "plugin"}[plural]
	case core.ExtSkill:
		return map[bool]string{true: "Skills", false: "skill"}[plural]
	}
	return "All"
}

func (a *AITools) visible(ctx Context) []core.Extension {
	r := ctx.Data.Extensions
	if r == nil {
		return nil
	}
	want := aiKinds[a.kind]
	var out []core.Extension
	for _, e := range r.Extensions {
		if want == "" || e.Kind == want {
			out = append(out, e)
		}
	}
	return out
}

func (a *AITools) selected(ctx Context) (core.Extension, bool) {
	vis := a.visible(ctx)
	if len(vis) == 0 {
		return core.Extension{}, false
	}
	a.cursor = min(max(a.cursor, 0), len(vis)-1)
	return vis[a.cursor], true
}

// Update moves through the list and asks for previews of changes.
func (a *AITools) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}
	n := len(a.visible(ctx))
	switch key.String() {
	case "up", "k":
		a.cursor = max(a.cursor-1, 0)
	case "down", "j":
		a.cursor = min(a.cursor+1, max(n-1, 0))
	case "home", "g":
		a.cursor = 0
	case "end", "G":
		a.cursor = max(n-1, 0)
	case "right", "l":
		a.kind, a.cursor = (a.kind+1)%len(aiKinds), 0
	case "left", "h":
		a.kind, a.cursor = (a.kind+len(aiKinds)-1)%len(aiKinds), 0
	case "s", "f5":
		return a, send(RefreshExtensionsMsg{})
	case " ", "enter":
		if e, ok := a.selected(ctx); ok && e.CanToggle {
			return a, send(ShowActionPreviewMsg{Plan: ToggleExtensionPlan(ctx, e, !e.Enabled), Confirm: ChangeExtensionMsg{Ext: e, On: !e.Enabled}})
		}
	case "r", "delete":
		if e, ok := a.selected(ctx); ok {
			return a, send(ShowActionPreviewMsg{Plan: RemoveExtensionPlan(ctx, e), Confirm: ChangeExtensionMsg{Ext: e, Remove: true}})
		}
	}
	return a, nil
}

// Hints lists the keys that apply to the highlighted entry.
func (a *AITools) Hints(ctx Context) []components.KeyHint {
	h := []components.KeyHint{{Key: "↑/↓", Desc: "choose"}, {Key: "←/→", Desc: "type"}}
	if e, ok := a.selected(ctx); ok {
		if e.CanToggle {
			h = append(h, components.KeyHint{Key: "space", Desc: map[bool]string{true: "turn off", false: "turn on"}[e.Enabled]})
		}
		if e.CanRemove {
			h = append(h, components.KeyHint{Key: "r", Desc: "remove…"})
		}
	}
	return append(h, components.KeyHint{Key: "s", Desc: "read again"})
}

// summary is "5 MCP servers · 2 plugins · 7 skills in 4 apps".
func aiSummary(r *core.ExtensionReport) string {
	counts := map[core.ExtensionKind]int{}
	apps := map[string]bool{}
	for _, e := range r.Extensions {
		counts[e.Kind]++
		apps[e.App] = true
	}
	parts := []string{
		components.Plural(counts[core.ExtMCP], "MCP server"),
		components.Plural(counts[core.ExtPlugin], "plugin"),
		components.Plural(counts[core.ExtSkill], "skill"),
	}
	return strings.Join(parts, " · ") + " in " + components.Plural(len(apps), "app")
}

// View renders the list, or what to expect when there is nothing.
func (a *AITools) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	r := d.Extensions
	sub := "MCP servers, plugins and skills in your AI apps"
	if r != nil && len(r.Extensions) > 0 {
		sub = aiSummary(r) + " · read " + components.HumanTime(r.At, ctx.Now)
	}
	head := components.PageHead(t, "AI Tools", sub, w) + "\n\n"

	switch {
	case !d.ExtensionsAvailable:
		return head + components.ErrorState{Title: "AI Tools is unavailable", Summary: "ATLAS couldn't set up reading your AI apps."}.Render(t, w)
	case d.ExtensionsErr != "":
		return head + components.ErrorState{Title: "Couldn't read your AI apps", Details: d.ExtensionsErr, ShowDetails: ctx.Advanced(), Hint: "Press s to try again."}.Render(t, w)
	case r == nil:
		return head + t.Muted.Render("Reading your AI apps…")
	case len(r.Extensions) == 0:
		return head + components.EmptyState{
			Title:   "No MCP servers, plugins or skills found",
			Message: "ATLAS looked in the settings of these AI apps and found none set up:",
			Plan:    []string{"Claude Code and Claude Desktop", "Cursor, VS Code and Windsurf", "Gemini CLI and Codex"},
			Hint:    "Press s to look again after adding some.",
		}.Render(t, w) + a.problems(ctx, w)
	}

	var tabs []string
	for i, k := range aiKinds {
		label := aiKindLabel(k, true)
		if i == a.kind {
			tabs = append(tabs, t.Accent.Bold(true).Render("["+label+"]"))
		} else {
			tabs = append(tabs, t.Muted.Render(label))
		}
	}
	head += components.Truncate(strings.Join(tabs, "  "), w) + "\n\n"

	vis := a.visible(ctx)
	a.cursor = min(max(a.cursor, 0), max(len(vis)-1, 0))
	nameW := 0
	for _, e := range vis {
		nameW = max(nameW, len([]rune(e.Name)))
	}
	nameW = min(nameW, max(w/3, 12))

	var lines []string
	selStart, selEnd := 0, 0
	body := func(text string, style func(...string) string) {
		for _, l := range strings.Split(components.Wrap(text, max(w-6, 10)), "\n") {
			lines = append(lines, "      "+style(l))
		}
	}
	var group core.ExtensionKind = "none"
	counts := map[core.ExtensionKind]int{}
	for _, e := range vis {
		counts[e.Kind]++
	}
	for i, e := range vis {
		if aiKinds[a.kind] == "" && e.Kind != group {
			if group != "none" {
				lines = append(lines, "")
			}
			group = e.Kind
			lines = append(lines, components.Section(t, t.Heading.Render(fmt.Sprintf("%s (%d)", aiKindLabel(e.Kind, true), counts[e.Kind])), w))
		}
		dot := t.Success.Render("●")
		if !e.Enabled {
			dot = t.Subtle.Render("○")
		}
		where := e.App
		if e.Scope != "user" {
			where += " · " + e.Scope
		}
		name := components.PadRight(e.Name, nameW)
		prefix, style := "  ", t.Body.Render
		if i == a.cursor {
			prefix, style = t.Accent.Render("› "), t.Accent.Bold(true).Render
			selStart = len(lines)
		}
		row := prefix + dot + " " + style(name) + "  " + t.Subtle.Render(where)
		if !e.Enabled {
			row += t.Subtle.Render(" · off")
		}
		lines = append(lines, components.Truncate(row, w))
		if i != a.cursor {
			continue
		}
		if e.Detail != "" {
			body(e.Detail, t.Muted.Render)
		}
		switch {
		case e.CanToggle && e.CanRemove:
			body("Space turns it "+map[bool]string{true: "off", false: "on"}[e.Enabled]+" · r removes it", t.Accent.Render)
		case e.CanToggle:
			body("Space turns it "+map[bool]string{true: "off", false: "on"}[e.Enabled]+". Nothing changes until you confirm", t.Accent.Render)
		case e.CanRemove:
			body("r removes it. Nothing changes until you confirm", t.Accent.Render)
		}
		if e.Note != "" && !e.CanRemove {
			body(e.Note, t.Subtle.Render)
		}
		if ctx.Advanced() {
			body("Where: "+e.Path, t.Subtle.Render)
		}
		selEnd = len(lines)
	}
	if len(vis) == 0 {
		lines = append(lines, t.Muted.Render("None of this type."))
	}
	tail := a.problems(ctx, w)
	avail := ctx.Height() - strings.Count(head, "\n") - strings.Count(tail, "\n")
	return head + strings.Join(windowLines(lines, selStart, max(selEnd, selStart+1), avail), "\n") + tail
}

// problems lists configuration files that couldn't be read.
func (a *AITools) problems(ctx Context, w int) string {
	r := ctx.Data.Extensions
	if r == nil {
		return ""
	}
	var bad []core.ExtensionSource
	for _, s := range r.Sources {
		if s.Status != core.SourceOK {
			bad = append(bad, s)
		}
	}
	if len(bad) == 0 {
		return ""
	}
	sort.SliceStable(bad, func(i, j int) bool { return bad[i].App < bad[j].App })
	t := ctx.Theme
	var b strings.Builder
	b.WriteString("\n\n")
	for _, s := range bad {
		line := "! Couldn't read " + s.App + "'s settings"
		if ctx.Advanced() {
			line += " (" + s.Path + "): " + s.Details
		}
		b.WriteString(t.Warning.Render(components.Truncate(line, w)) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// RemoveExtensionPlan previews removing an MCP server or a skill.
func RemoveExtensionPlan(ctx Context, e core.Extension) core.ActionPlan {
	kind := aiKindLabel(e.Kind, false)
	p := core.ActionPlan{
		Title:       fmt.Sprintf("Remove the %s %s", kind, e.Name),
		Destructive: true,
		Available:   e.CanRemove,
		Technical:   []string{"File or folder: " + e.Path},
	}
	if !e.CanRemove {
		p.Unavailability = "ATLAS can't remove this " + kind + " safely. " + e.Note
	}
	switch e.Kind {
	case core.ExtMCP:
		p.WhatHappens = fmt.Sprintf("ATLAS will delete the %s entry from %s's settings file, so %s stops starting this server.", e.Name, e.App, e.App)
		p.Why = "Fewer servers means fewer tools for the AI to choose from, less to start up, and less access to your files and accounts."
		p.WhatChanges = []string{
			fmt.Sprintf("The %s entry is removed from %s", e.Name, e.Path),
			"A copy of the file as it was is saved next to it first",
		}
		p.Safety = "Safe. Only that one entry changes; everything else in the file stays. The server's program itself isn't uninstalled. Close " + e.App + " first if it is open, so it doesn't write the file back."
		p.Reversible = "Yes: copy the backup file over the settings file, or add the server again in " + e.App + "."
		if len(e.Locator) > 0 {
			p.Technical = append(p.Technical, "JSON key: "+strings.Join(e.Locator, " › "))
		}
	case core.ExtSkill:
		p.WhatHappens = fmt.Sprintf("ATLAS will move the %s skill's folder out of %s into ATLAS's own folder, so %s no longer sees it.", e.Name, e.App, e.App)
		p.Why = "Skills you don't use still take up room in what the AI reads when it starts."
		p.WhatChanges = []string{"The folder " + e.Path + " moves into ATLAS's removed-skills folder"}
		p.Safety = "Safe. Nothing is deleted: the folder is moved, with all its files."
		p.Reversible = "Yes: move the folder back. Activity History shows where it went."
	default:
		p.WhatHappens = "ATLAS doesn't remove plugins."
		p.Available = false
		if p.Unavailability == "" {
			p.Unavailability = e.Note
		}
	}
	return p
}

// ToggleExtensionPlan previews turning a plugin on or off.
func ToggleExtensionPlan(ctx Context, e core.Extension, on bool) core.ActionPlan {
	verb := map[bool]string{true: "on", false: "off"}[on]
	p := core.ActionPlan{
		Title:       fmt.Sprintf("Turn the plugin %s %s", e.Name, verb),
		WhatHappens: fmt.Sprintf("ATLAS will mark %s as %s in %s's settings. %s applies it the next time it starts.", e.Name, verb, e.App, e.App),
		Why:         "A plugin that is off keeps its files but adds no commands, skills or servers.",
		WhatChanges: []string{fmt.Sprintf("enabledPlugins › %s is set to %v in %s", strings.Join(e.Locator, ""), on, e.Path)},
		Safety:      "Safe. The plugin isn't uninstalled and no other setting changes. A copy of the settings file is kept next to it.",
		Reversible:  fmt.Sprintf("Yes: turn it %s again here, or in %s.", map[bool]string{true: "off", false: "on"}[on], e.App),
		Technical:   []string{"File: " + e.Path},
		Available:   e.CanToggle,
	}
	if !e.CanToggle {
		p.Unavailability = e.Note
	}
	return p
}

var (
	_ Screen        = (*AITools)(nil)
	_ SelfScrolling = (*AITools)(nil)
)
