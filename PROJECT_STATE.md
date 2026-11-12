# ATLAS — Project State

ATLAS (Application & Tool Lifecycle Analysis System) is a cross-platform terminal
application for understanding and managing installed software. Primary target:
Windows 11. Linux stays compatible.

## Current phase

**Phase 2 — Inventory: complete.** Awaiting approval to start Phase 3.

| Phase | Status |
|-------|--------|
| 1. Go bootstrap and TUI foundation | done |
| 2. Inventory: read-only scanning, SQLite, My Software, Activity History | done |
| 3. Updates | not started |

## What exists now

| Area | Status | Notes |
|------|--------|-------|
| Configuration | done | YAML with defaults, per-field validation, atomic save, backup of a malformed original |
| Logging | done | slog JSON lines to a file only |
| Platform paths | done | Config, log and data dirs; build-tagged; `ATLAS_HOME` override |
| TUI shell | done | Sidebar or three-tier tab bar, status bar, help, scrolling with indicators |
| Simple / Advanced mode, themes | done | Persisted; Advanced adds package IDs, paths, source IDs, technical errors |
| **Inventory scanners** | done | Windows: registry Uninstall keys, winget, Chocolatey, Scoop. Linux: dpkg, RPM, Snap, Flatpak. All read-only |
| **Merging** | done | Same name and compatible version across sources become one entry; winget-truncated names match full names |
| **Apps vs system components** | done | Heuristics per source; apt auto-installed packages and libraries are components; hidden by default |
| **Scan service** | done | Parallel, per-source timeouts, panic-safe, cancellable; complete/partial/failed/nothing/cancelled |
| **SQLite store** | done | Pure-Go driver, versioned migrations, latest list + scan summaries + activity |
| **Scan My System** | done | Preview → confirm → live per-source progress → status line → saved; `x` stops |
| **My Software** | done | Searchable list, apps/everything filter, details, data-sized columns, empty and error states |
| **Activity History** | done | Every scan and clear recorded with outcome; details in Advanced mode |
| **Home** | done | Real counts, last scan time, partial-scan warning, recent activity, state-aware recommendations |
| **Clear saved list** | done | Settings → preview → confirm; never touches installed software; recorded |
| Smart Analysis, Dependencies, Cleanup, Discover Apps, Updates | empty states | Honest "arrives in Phase N" screens |
| Package management, installs, updates, removals | not started | Phase 3+ |

## Guarantees still held

- No install, update, removal, or privilege request anywhere.
- No sample inventory data in shipped code. Test fixtures live only in `_test.go` files.
- Nothing runs without a preview, except read-only scans when the user turns previews off.
- Every scan and clear is recorded in Activity History.
