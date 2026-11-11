# ATLAS — Implementation Plan

## Architecture (modular monolith, hexagonal)

```
cmd/atlas              entry point, flags, wiring
internal/app           composition root: paths -> config -> logging -> TUI
internal/config        YAML preferences: load (never writes), save (atomic, backup)
internal/logging       slog JSON file logger
internal/platform      OS-specific paths (build-tagged files)
internal/core          domain types and ports; no I/O, no imports from tui/adapters
internal/tui           Bubble Tea root model: keys, overlays, status bar, scrolling
  nav/                 section list, metadata, navigation state (pure)
  layout/              terminal size -> layout decisions (pure)
  theme/               dark/light palettes and derived styles
  components/          empty state, error state, help, status bar, sidebar/tab bar,
                       action preview, scroll window, text helpers
  screens/             Screen interface, Context, messages, Home, Settings, ComingSoon
```

Rules:
- `core` depends on nothing inside the repo. Adapters implement its interfaces.
- Screens talk to the root only through messages in `screens/msgs.go`.
- Platform checks go through `internal/platform`, never `runtime.GOOS` in UI code
  (the one exception is the read-only diagnostics label on Home).
- Every consequential action is a `core.ActionPlan` and goes through the preview.

## Phase 1 — Go bootstrap and TUI foundation — DONE

Delivered as listed in `PROJECT_STATE.md`. Validation risks found at the start
and how they were handled:

| Risk | Handling |
|------|----------|
| Bubble Tea has no z-layer compositing for overlays | Help and preview replace the content area; header/nav/footer stay (D4) |
| Nine sections cannot fit a tab bar at 80 columns | Three-tier tab bar: full, short, compact (digits + current name) |
| Short terminals clip content silently | Root-level scroll with explicit "more above/below" indicators |
| YAML booleans cannot distinguish false from absent | Presence check on raw document for keys that default to true |
| Writing config on first run would be a silent side effect | Load never writes; save only on user change, reported in the status bar (D5) |
| Malformed config could be destroyed by the first save | Backed up to `config.yaml.bak` first (D5) |
| Logging to the terminal would corrupt the TUI | File-only logger; discard logger if the file cannot be opened; problem shown in Settings |
| Legacy conhost and some fonts lack box-drawing glyphs | Borders are used only for the sidebar divider and rules; no emoji |

## Phase 2 — Inventory (planned, not started)

- `internal/inventory`: read-only scanners implementing `core.Scanner`
  (Windows: Uninstall registry keys, Start Menu, winget/scoop/choco lists;
  Linux: dpkg/apt, snap, flatpak). Each runs with a timeout and reports partial
  results with plain-language status.
- `internal/database`: SQLite schema and repository for scan results and activity
  log; pure-Go driver preferred to keep `CGO_ENABLED=0` cross-builds.
- "Scan My System" becomes available: preview -> confirm -> progress -> summary.
- My Software screen with real data, search and filters (Bubbles list/textinput).
- Activity History backed by the database; Recent activity on Home.
- Home overview figures switch from "not yet scanned" to real counts with a
  "last scanned" timestamp.

## Phase 3+ (outline)

- Updates: `core.PackageManager` adapters (winget, scoop, choco, apt, snap,
  flatpak); "Update all supported applications" with per-item preview and skip.
- Smart Analysis, Dependencies, Cleanup, Discover Apps: each read-only first,
  then actions behind the preview with explicit confirmation.
- Security: privilege requests only when an action needs them, explained first.
- AI explanations: optional, off by default, local-first.
