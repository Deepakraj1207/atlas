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
internal/analysis      support table, runtime detection, dependency readers, usage records, rules, Service
internal/cleanup       removers, safety gate, tidiers (exact clean-ups), Service (core.CleanupService)
internal/handover      terminal Job shared by updates, cleanup and discover
internal/discover      embedded catalogue (catalog.yaml), assessment, install planning, Service
  registry*.go         Windows Uninstall keys (pure mapping + windows-only enumeration)
  winget, scoop, chocolatey, dpkg, rpm, snap, flatpak
  defaults_{windows,other}.go   which scanners run on which OS
internal/tui           root model: keys, overlays, scan lifecycle, data loading
  nav/ layout/ theme/ components/ screens/
```

Dependency rule: `tui` depends only on `core` interfaces. `app` is the only
package that knows about `database`, `inventory`, `updates`, `analysis`, `cleanup` and `discover`.

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

## Phase 4 — Smart Analysis and Dependencies — DONE

| Step | Result |
|------|--------|
| Core: Finding, Runtime, Support, CheckSummary, AnalysisResult, AnalysisService | done |
| Schema v3: analyses (latest result as one JSON document) | done |
| Support table with references, as of 2026-11 | done |
| Runtime detection for Python, Node.js, .NET, Java and Visual C++ across registry, scoop, choco, dpkg and rpm names | done |
| Dependency readers: dpkg (Depends, Pre-Depends, Provides), RPM (requires to provides), Chocolatey (.nuspec), Scoop (manifest depends) | done |
| Windows usage records (UserAssist), decoded and matched by install folder | done |
| Rules engine (pure) and service; analysis needs a scan first | done |
| TUI: Smart Analysis, Dependencies, Home figure and recommendations, stale notes | done |

Changes from the plan:
- "Long-unused" is Windows-only and needs positive evidence. Linux has no
  equivalent record, and the check says so.
- Java support isn't judged because it depends on the vendor; the check says so.
- Runtime detection deliberately ignores Debian's `python3` meta-package and
  versioned helper packages such as `python3.12-venv`.

## Phase 5 — Cleanup — DONE

| Step | Result |
|------|--------|
| Shared handover package; step actions (update, remove, clean) and sizes | done |
| Store: drop removed software from the saved list | done |
| Removers: winget (by ID or exact name), Chocolatey, Scoop, APT, DNF, Snap, Flatpak | done |
| Safety gate with fresh dependency data, protected lists, Essential/required, DNF protected.d | done |
| Tidiers: APT cache and autoremove (simulated), DNF cache and unneeded, old snap revisions, Scoop old versions and cache | done |
| TUI: Cleanup screen, `r` in My Software, previews with `r` confirmation, run and record | done |

Changes from the plan:
- "Leftover files from uninstalled programs" isn't offered. Their locations aren't
  certain enough to delete safely. Only package-manager caches and old versions are.
- Flatpak "unused runtimes" isn't offered, because ATLAS can't list them exactly beforehand.
- Clean-up results are kept for the session only. They go stale quickly and finding
  them again takes seconds.

## Phase 6 — Discover Apps — DONE

| Step | Result |
|------|--------|
| Core: CatalogEntry, DiscoverItem, Discovery, DiscoverService, install action | done |
| Embedded catalogue, strict validation (IDs per manager, categories, https homepages, product keys) | done |
| Assessment: installed by product key or package ID, platform filtering, manager preference, blocked managers, suggestions with reasons | done |
| Install planning per manager, exact commands, notes (classic snaps, batcat), rollback via My Software | done |
| TUI: Discover screen with categories and multi-select, install preview and handover, Home "Find New Tools" opens it | done |

## Phase 7 — Release and AI explanations — BUILT, AWAITING APPROVAL

| Step | Result |
|------|--------|
| MIT licence; version, commit and date in `atlas -version` | done |
| `packaging/build-release.sh`: Windows zips, Linux tarballs, `.deb` (dpkg-deb) and `.rpm` (rpmbuild), SHA256SUMS | done |
| ~~`tools/release`: winget manifests~~ | removed (D83): installs use the one-line installers instead |
| `install.ps1` / `install.sh`: one-line installers, checksum-verified, per-user, update by re-running; `install-check.yml` on clean Windows (PowerShell 5.1 and 7), Ubuntu, Fedora, Alpine | done |
| `release.yml`: tests, build, checks on Windows/Ubuntu/Fedora, draft release on `v*` tags | done (dry run on every packaging change) |
| `tools/catalogcheck` + `catalog-check.yml` (manual) | done |
| Go 1.27.4 | done |
| AI explanations: config, Settings row, `e` in Smart Analysis and My Software, preview of exact text, answer overlay, Activity History, plain-language errors | done |
| Docs: `docs/RELEASING.md`, README install section | done |

Not done, needing the user: code signing (needs a certificate in the user's name), and a live
AI request with the user's own API key.

## Later (not started)

- Code-signed Windows builds.
- Remaining Windows 11 checks: Ctrl+C during a real install, a live "Install
  cancelled", and the legacy conhost window (`docs/WINDOWS_TEST_PLAN.md`).
