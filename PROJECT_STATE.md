# ATLAS — Project State

ATLAS (Application & Tool Lifecycle Analysis System) is a cross-platform terminal
application for understanding and managing installed software. Primary target:
Windows 11. Linux stays compatible.

## Current phase

**Phase 6 — Discover Apps: complete.** All nine sections are now real. Awaiting
direction for what comes next (see IMPLEMENTATION_PLAN.md, Phase 7+).

| Phase | Status |
|-------|--------|
| 1. Go bootstrap and TUI foundation | done |
| 2. Inventory: read-only scanning, SQLite, My Software, Activity History | done |
| 3. Updates: check, review, install through the package managers | done |
| 4. Smart Analysis and Dependencies | done |
| 5. Cleanup: remove software and tidy caches, with a fresh dependency gate | done |
| 6. Discover Apps: curated catalogue, local suggestions, install with preview | done |

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
| **Update checks** | done | winget, Chocolatey, Scoop (Windows); APT, DNF, Snap, Flatpak (Linux). Read-only, parallel, timeouts, cancellable |
| **Updates screen** | done | Apps listed, system components behind `f`, held packages shown but never selected; select, update selected, update all |
| **Installing updates** | done | Preview lists every change, permission prompts, rollback per manager, exact commands (Advanced). Then ATLAS hands the terminal to each package manager |
| **Elevation** | done | sudo on Linux (password asked by sudo), Windows installers may show UAC, Chocolatey and global Scoop blocked with an explanation unless ATLAS runs as administrator |
| **Results** | done | Per-step outcome, exit codes and restart notices in Activity History; applied updates leave the list; Home suggests a rescan |
| **Smart Analysis** | done | Runtimes past or near end of support (Python, Node.js, .NET, Visual C++), same app from two tools, several versions, apps not opened in over a year (Windows, positive evidence only), runtimes nothing needs, updates waiting, stale data. Each finding says why it matters, what you can do, and its source |
| **What ATLAS checked** | done | Every check listed with its result, including "none found" and "not checked" with the reason |
| **Dependencies** | done | Runtimes grouped by version with support status; direct and indirect dependents from dpkg, RPM, Chocolatey and Scoop records; "not recorded" where Windows keeps none |
| **Analysis storage** | done | Schema v3; latest analysis kept as one document; cleared with the software list; marked stale after a newer scan |
| **Removing software** | done | From My Software (`r`) or Cleanup suggestions. Every plan re-reads dependency records; blocked items say why |
| **Safety gate** | done | Blocks: anything other installed software needs; dpkg Essential/required; DNF protected list; tools ATLAS or the OS rely on; runtimes and components whose users can't be determined; software owned by more than one package manager |
| **Things to tidy** | done | Exact, read-only discovery: APT and DNF caches, packages APT or DNF marks unneeded, old snap revisions, old Scoop versions and downloads; sizes shown |
| **Destructive confirmation** | done | Removal and clean-up previews need `r`; Enter alone never confirms a deletion |
| **Run and record** | done | Same terminal handover as updates; removed software leaves My Software; Smart Analysis marked stale; Activity History records everything |
| **Discover Apps** | done | 30 curated tools in 7 categories, embedded and validated at build time, dated 2026-11; suggestions only from local data with a reason ("because you have Git"); installed and not-installable tools clearly marked with the reason |
| **Installing** | done | Picks the best available manager (winget, Scoop, Chocolatey; APT, DNF, Flatpak, Snap); blocks Chocolatey without admin, APT without package lists, Flatpak without Flathub; adds a missing Scoop bucket as a visible step; classic snaps explained |
| Removals, cleanup actions | not started | Phase 5 |

## Guarantees still held

- ATLAS changes the system only by installing updates, installing tools the user picked in
  Discover, removing software, or clearing caches, and only after a preview the user confirms.
  Removals and clean-ups need `r`, not Enter.
- Discover makes no network calls to build suggestions; they come from the saved list.
- ATLAS never answers a package manager's prompt, accepts an agreement, or handles a password.
  The package manager asks the user directly in the terminal.
- ATLAS never asks for elevation for itself. It explains when sudo, UAC or PolicyKit may ask.
- No `-y`, purge, force or delete-data flags are ever passed. Users' settings and data are kept.
- No sample data in shipped code; test fixtures live only in `_test.go` files.
- Every scan, check, analysis, clear and update run is recorded in Activity History.
- Analysis never infers from missing data: no usage record means no claim about usage,
  and no dependency record means "treat it as needed".
