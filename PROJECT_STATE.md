# ATLAS — Project State

ATLAS (Application & Tool Lifecycle Analysis System) is a cross-platform terminal
application for understanding and managing installed software. Primary target:
Windows 11. Linux stays compatible.

## Current phase

**Phase 7 — Release and AI explanations: done; v0.1.0 tagged.** The tag (commit 11d7977)
built and passed every release check, and a **draft** release "ATLAS 0.1.0" with all
files exists on GitHub. Next, by the user: publish the draft, let `winget-check` pass,
then submit the winget manifests (see `docs/RELEASING.md`).

- MIT licence; `atlas -version` shows version, commit and build date.
- Release workflow: Windows zips (x64, ARM64), Linux tarballs, `.deb` and `.rpm`
  (`atlas-lifecycle`), checksums and winget manifests, all checked automatically; a
  version tag creates a draft release for a person to publish.
- winget package `Deepakraj1207.ATLAS` (moniker and command `atlas`); manifests pass
  `winget validate` on the user's Windows 11 PC. Submitting to winget is manual.
- Catalogue check: a manual workflow confirms every Discover package ID still exists.
- AI explanations: off by default; `e` on a finding or an app, after a preview of exactly
  what is sent, asks Claude (official Anthropic Go SDK, `claude-opus-5-5`, billed to the
  user's API account) or a local model in Ollama (free, nothing leaves the computer).
- Go 1.27.4 (Go 1.24 is out of support; the SDK needs 1.25+).

**Windows verification: done, apart from three small checks.** CI on Windows Server 2025
and 2022 runs the tests, the self-test and scripted interface checks in a real Windows
console on every push. On the user's Windows 11 PC: self-test 15/15, all screens, a real
winget install and removal (jq), and the UAC prompt declined and approved (Notepad++).
Fifteen problems found this way are fixed (see TEST_STATUS.md). Still open: Ctrl+C during
a real install, seeing "Install cancelled" live, and the legacy conhost window.

**Engineering audit remediation (2026-11-14): done.** Five issues from a senior review,
fixed one at a time, each with regression tests that fail on the old code:

- **P0 cleanup dependency safety (D77).** The removal gate fails closed: an item is
  refused when any dependency source it came from failed, timed out, was unavailable,
  was stopped, came back empty or doesn't list it. Scoop dependents are now found.
- **P0 Linux routing (D78).** `/etc/os-release` picks the RPM tool: DNF on Fedora/RHEL
  and relatives, Zypper on openSUSE/SLES (new removals, update checks and updates);
  yum-only, non-RPM and unknown systems are refused with the reason. Checked in real
  Fedora, Rocky, openSUSE Tumbleweed and Leap, CentOS 7 and Ubuntu containers (`linux`
  workflow).
- **P0 post-operation verification (D79).** After removals and updates every source is
  read again; only confirmed results leave My Software or Updates. Anything not
  confirmed is kept, and Activity History says what was found.
- **P1 deduplication (D80).** Merging keeps two entries unless they can only be one
  install: no merging across Scoop, dpkg, rpm, Snap or Flatpak; no merging of different
  publishers or package IDs from one source; truncated or unversioned entries only on a
  unique match.
- **P1 AI (D81).** Optional local models through Ollama; billing spelled out with the
  worst-case cost; user, home folder and computer names removed before the preview;
  configurable answer limit (`ai.max_output_tokens`, default 4096).

| Phase | Status |
|-------|--------|
| 1. Go bootstrap and TUI foundation | done |
| 2. Inventory: read-only scanning, SQLite, My Software, Activity History | done |
| 3. Updates: check, review, install through the package managers | done |
| 4. Smart Analysis and Dependencies | done |
| 5. Cleanup: remove software and tidy caches, with a fresh dependency gate | done |
| 6. Discover Apps: curated catalogue, local suggestions, install with preview | done |
| 7. Release packaging, winget, catalogue check, AI explanations | done; v0.1.0 draft release created |

## What exists now

| Area | Status | Notes |
|------|--------|-------|
| Configuration | done | YAML with defaults, per-field validation, atomic save, backup of a malformed original |
| Logging | done | slog JSON lines to a file only |
| Platform paths | done | Config, log and data dirs; build-tagged; `ATLAS_HOME` override |
| TUI shell | done | Sidebar or three-tier tab bar, status bar, help, scrolling with indicators |
| Simple / Advanced mode, themes | done | Persisted; Advanced adds package IDs, paths, source IDs, technical errors |
| **Inventory scanners** | done | Windows: registry Uninstall keys, winget, Chocolatey, Scoop. Linux: dpkg, RPM, Snap, Flatpak. All read-only |
| **Merging** | done | One entry only when the sources can report the same copy (Windows entries, winget, Chocolatey) and nothing contradicts it; truncated or unversioned entries only on a unique match; when unsure, two entries (D80) |
| **Apps vs system components** | done | Heuristics per source; apt auto-installed packages and libraries are components; hidden by default |
| **Scan service** | done | Parallel, per-source timeouts, panic-safe, cancellable; complete/partial/failed/nothing/cancelled |
| **SQLite store** | done | Pure-Go driver, versioned migrations, latest list + scan summaries + activity |
| **Scan My System** | done | Preview → confirm → live per-source progress → status line → saved; `x` stops |
| **My Software** | done | Searchable list, apps/everything filter, details, data-sized columns, empty and error states |
| **Activity History** | done | Every scan and clear recorded with outcome; details in Advanced mode |
| **Home** | done | Real counts, last scan time, partial-scan warning, recent activity, state-aware recommendations |
| **Clear saved list** | done | Settings → preview → confirm; never touches installed software; recorded |
| **Update checks** | done | winget, Chocolatey, Scoop (Windows); APT, DNF or Zypper (by distribution), Snap, Flatpak (Linux). Read-only, parallel, timeouts, cancellable |
| **Updates screen** | done | Apps listed, system components behind `f`, held packages shown but never selected; select, update selected, update all |
| **Installing updates** | done | Preview lists every change, permission prompts, rollback per manager, exact commands (Advanced). Then ATLAS hands the terminal to each package manager |
| **Elevation** | done | sudo on Linux (password asked by sudo), Windows installers may show UAC, Chocolatey and global Scoop blocked with an explanation unless ATLAS runs as administrator |
| **Results** | done | Per-step outcome, exit codes and restart notices in Activity History; updates leave the list only once the computer shows the new version (D79); Home suggests a rescan |
| **Smart Analysis** | done | Runtimes past or near end of support (Python, Node.js, .NET, Visual C++), same app from two tools, several versions, apps not opened in over a year (Windows, positive evidence only), runtimes nothing needs, updates waiting, stale data. Each finding says why it matters, what you can do, and its source |
| **What ATLAS checked** | done | Every check listed with its result, including "none found" and "not checked" with the reason |
| **Dependencies** | done | Runtimes grouped by version with support status; direct and indirect dependents from dpkg, RPM, Chocolatey and Scoop records; "not recorded" where Windows keeps none |
| **Analysis storage** | done | Schema v3; latest analysis kept as one document; cleared with the software list; marked stale after a newer scan |
| **Removing software** | done | From My Software (`r`) or Cleanup suggestions. Every plan re-reads dependency records; blocked items say why |
| **Safety gate** | done | Blocks: anything other installed software needs; anything whose dependency records couldn't be read completely (fails closed, D77); dpkg Essential/required; DNF protected list; tools ATLAS or the OS rely on; runtimes and components whose users can't be determined; software owned by more than one package manager; RPM removals where the distribution's tool isn't DNF or Zypper |
| **Things to tidy** | done | Exact, read-only discovery: APT and DNF caches, packages APT or DNF marks unneeded, old snap revisions, old Scoop versions and downloads; sizes shown |
| **Destructive confirmation** | done | Removal and clean-up previews need `r`; Enter alone never confirms a deletion |
| **Run and record** | done | Same terminal handover as updates; removed software leaves My Software only when no source lists it any more (D79); Smart Analysis marked stale; Activity History records everything |
| **AI explanations** | done | Claude (Anthropic API, billed, worst case shown) or Ollama (local, free); identifying names removed before the preview; answer limit configurable; advice only (D81) |
| **Discover Apps** | done | 30 curated tools in 7 categories, embedded and validated at build time, dated 2026-11; suggestions only from local data with a reason ("because you have Git"); installed and not-installable tools clearly marked with the reason |
| **Installing** | done | Picks the best available manager (winget, Scoop, Chocolatey; APT, DNF, Flatpak, Snap); blocks Chocolatey without admin, APT without package lists, Flatpak without Flathub; adds a missing Scoop bucket as a visible step; classic snaps explained |
| Windows verification tooling | done | `atlas -selftest` (read-only report), Windows CI, `tools/conharness` (drives ATLAS in a real Windows console) |

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
- Success is never reported from an exit code alone: removals and updates count only
  once the computer confirms them; otherwise the records are kept.
- Nothing is sent to an AI unless the user turned it on and confirmed a preview of the
  exact text; AI answers never run anything.
- Analysis never infers from missing data: no usage record means no claim about usage,
  and no dependency record means "treat it as needed".
