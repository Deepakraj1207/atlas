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
internal/inventory     scanners (core.Scanner), merge, RunAll, Service (core.InventoryService)
internal/updates       package managers (core.PackageManager), Service (core.UpdateService), terminal Job
  registry*.go         Windows Uninstall keys (pure mapping + windows-only enumeration)
  winget, scoop, chocolatey, dpkg, rpm, snap, flatpak
  defaults_{windows,other}.go   which scanners run on which OS
internal/tui           root model: keys, overlays, scan lifecycle, data loading
  nav/ layout/ theme/ components/ screens/
```

Dependency rule: `tui` depends only on `core` interfaces. `app` is the only
package that knows about `database`, `inventory` and `updates`.

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

## Phase 3 — Updates — DONE

| Step | Result |
|------|--------|
| Core: Update, UpdateStep, Command, Elevation, StepResult, TerminalJob, PackageManager, UpdateService | done |
| Schema v2: update_checks, update_check_sources, updates (migrates v1 in place) | done |
| Managers: winget, Chocolatey, Scoop, APT, DNF, Snap, Flatpak | done |
| Service: parallel check, enrich from saved list, plan, record results | done |
| Terminal job: runs steps with the real terminal, Ctrl+C stops, waits for Enter | done |
| TUI: Updates screen, check and apply previews, Home figures, failed-check explanation | done |

Changes from the plan:
- DNF was added alongside RPM inventory so Fedora/RHEL users get updates too.
- Apply runs in the user's terminal instead of in the background. That is the only
  way sudo, UAC, licence agreements and confirmation prompts work faithfully, and
  it means the user sees the package manager's real output.
- APT refuses to report "up to date" when no package lists were ever downloaded.

## Phase 4 — Smart Analysis and Dependencies (planned, not started)

- Smart Analysis: duplicates (same app from two sources), long-unused software where
  the platform records it, unmaintained or end-of-life runtimes, all explained plainly.
- Dependencies: which installed runtimes (.NET, Java, Python, VC++ redistributables)
  are needed by what, using package manager dependency data where available.
- Read-only. Any suggested action links to a preview in a later phase.

## Phase 5+ (outline)

Cleanup (removals with preview and dependency warnings), Discover Apps, optional AI explanations.
