# ATLAS — Implementation Plan

## Architecture (modular monolith, hexagonal)

```
cmd/atlas              entry point, flags
internal/app           composition root: paths -> config -> logging -> database -> inventory -> TUI
internal/core          domain types and ports (Scanner, Store, InventoryService, PackageManager)
internal/config        YAML preferences
internal/logging       slog JSON file logger
internal/platform      OS-specific paths (build-tagged)
internal/database      SQLite adapter for core.Store (modernc.org/sqlite, CGO-free)
internal/inventory     scanners (core.Scanner), merge, Service (core.InventoryService)
  registry*.go         Windows Uninstall keys (pure mapping + windows-only enumeration)
  winget, scoop, chocolatey, dpkg, rpm, snap, flatpak
  defaults_{windows,other}.go   which scanners run on which OS
internal/tui           root model: keys, overlays, scan lifecycle, data loading
  nav/ layout/ theme/ components/ screens/
```

Dependency rule: `tui` depends only on `core` interfaces. `app` is the only
package that knows about `database` and `inventory`.

## Phase 1 — Go bootstrap and TUI foundation — DONE

## Phase 2 — Inventory — DONE

| Step | Result |
|------|--------|
| Core types: SoftwareItem, Kind, ScanReport, SourceResult, Activity, ports | done |
| Platform data dir | done |
| SQLite schema v1 + migrations + store | done |
| Scanners: registry, winget, Chocolatey, Scoop, dpkg, RPM, Snap, Flatpak | done |
| Merge and app/component classification | done |
| Scan service with timeouts, cancellation, activity records | done |
| Scan flow in TUI with live progress and stop | done |
| My Software, Activity History, Home figures, Clear saved list | done |
| Scrollable previews and help | done |

Changes from the Phase 1 plan:
- Start Menu shortcuts were not scanned. They duplicate registry entries and add noise.
- pip and npm global packages were moved to a later phase. They need per-runtime
  discovery and deserve their own "developer tools" view.
- RPM was added so Fedora/RHEL/openSUSE users are not left with an empty list.

## Phase 3 — Updates (planned, not started)

- `core.PackageManager` adapters for winget, Chocolatey, Scoop, apt, Snap, Flatpak,
  each able to list available updates read-only first.
- Updates screen: what can be updated, from which source, with plain-language
  explanations. "Update all supported applications" builds one plan per item.
- Execution only after preview and confirmation; privilege elevation explained
  before it is requested; each step recorded in Activity History with outcome.
- Home "Updates available" figure becomes real.

## Phase 4+ (outline)

Smart Analysis, Dependencies, Cleanup, Discover Apps, optional AI explanations.
