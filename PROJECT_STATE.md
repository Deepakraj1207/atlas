# ATLAS — Project State

ATLAS (Application & Tool Lifecycle Analysis System) is a cross-platform terminal
application for understanding and managing installed software. Primary target:
Windows 11. Linux stays compatible.

## Current phase

**Phase 1 — Go bootstrap and TUI foundation: complete.** Awaiting approval to
start Phase 2.

## Workspace at start of Phase 1

- Empty git repository on branch `claude/nice-ptolemy-rlg34q`: no commits, no
  remote branches, no files. Everything below was created in Phase 1.

## What exists now

| Area | Status | Notes |
|------|--------|-------|
| Go module | done | `github.com/deepakraj1207/atlas`, Go 1.24, Bubble Tea 1.3, Lip Gloss 1.1, yaml.v3 |
| Entry point | done | `cmd/atlas` with `-config` and `-version`; version via ldflags |
| Configuration | done | YAML with defaults; missing file = defaults; malformed = defaults + visible warning; per-field validation; atomic save with backup of a malformed original |
| Logging | done | slog JSON lines appended to a file; never to the terminal UI; discard logger if the file cannot open |
| Platform paths | done | `internal/platform`, build-tagged Windows vs other; `ATLAS_HOME` override |
| TUI shell | done | Header, sidebar (wide) or tab bar (medium/narrow, three density tiers), content, status bar with hints |
| Home | done | Welcome line, 4 quick-action buttons, "Software overview" with explicit "not yet scanned" values, Recent activity empty state, Recommended actions, Diagnostics (Advanced only) |
| Settings | done | Mode, theme, keyboard hints, confirm-before-actions; file status; paths in Advanced mode |
| Other 7 sections | empty states | Title, summary, "What this will do" list, phase; no placeholder data |
| Simple / Advanced | done | `m` anywhere or Settings; changes visible labels and detail; persisted |
| Themes | done | Dark and light, teal/cyan accent; `t` anywhere or Settings; persisted |
| Responsive layout | done | Wide ≥110, Medium 80–109, Narrow 40–79, too-small below 40×10; header/hints drop on short terminals; content scrolls with PgUp/PgDn and shows "more" indicators |
| Help overlay | done | `?`; lists keys for the current screen, navigation and general keys |
| Action preview | done | `core.ActionPlan` rendered as what / why / changes / safe / reversible; Phase 1 plans are all marked not available |
| Core ports | stubs | `core.Scanner`, `core.PackageManager`, `core.SoftwareItem`; no implementations, nothing calls them |
| Inventory, package managers, SQLite, discovery, AI | not started | Phase 2 onward |

## Not in this phase (by design)

No installs, updates, removals, privilege requests, system scans, SQLite schema,
network access, or sample inventory data.
