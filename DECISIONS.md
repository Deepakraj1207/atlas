# ATLAS — Decisions

Short, dated records of choices that shape the code.

## 2026-11-11 — Phase 1

- **D1. Module path** `github.com/deepakraj1207/atlas`; everything under
  `internal/` except `cmd/`. Nothing is importable by third parties yet.
- **D2. Library versions** Bubble Tea v1.3.x, Lip Gloss v1.1.x, yaml.v3. The v2
  Charm line is still pre-release; v1 is stable on Windows. Bubbles is not yet
  imported (no list/input components in Phase 1) and will be added in Phase 2.
- **D3. Explicit themes, not terminal-adaptive colours.** Users choose dark or
  light and the choice is persisted. Adaptive colours would silently override it.
- **D4. Help and action preview replace the content area** instead of
  compositing over it. True z-layering needs ANSI-aware cell compositing; not
  worth it in Phase 1. Header, navigation and footer stay visible so context is
  never lost.
- **D5. Config is never written silently.** Loading never creates or rewrites
  the file. Saving happens only when the user changes a setting and the status
  bar reports success or failure. If the existing file was malformed it is
  backed up to `config.yaml.bak` before being replaced. Writes are atomic
  (temp file + rename).
- **D6. Malformed config degrades to defaults and is surfaced** in the status
  bar on startup and on the Settings screen (raw error only in Advanced mode).
  Unknown values such as `theme: purple` fall back per field with a warning
  rather than failing the whole file.
- **D7. Logging** is JSON lines via slog, appended to `atlas.log`. No rotation
  yet. Nothing is ever written to stdout/stderr while the TUI runs; if the log
  file cannot be opened, a discard logger is used and Settings says so.
- **D8. Paths**: config in `os.UserConfigDir()/atlas`; logs in
  `%LocalAppData%\atlas\logs` on Windows and `$XDG_STATE_HOME` or
  `~/.local/state/atlas/logs` elsewhere. `ATLAS_HOME` overrides both.
- **D9. Action preview is a core type** (`core.ActionPlan`) so every future
  consequential action must state what happens, why, what changes, whether it
  is safe, and whether it is reversible. Technical detail is a separate field
  shown only in Advanced mode.
- **D10. Phase 1 quick actions open the preview and stop.** Each plan is marked
  unavailable, names the phase that delivers it, and says nothing has changed.
  No fake progress, no fake results.
- **D11. No sample inventory data anywhere.** Home figures render as "not yet
  scanned" / "never". Empty states say what the section will do and which phase
  delivers it.
- **D12. Section navigation is global; in-screen movement is local.** Tab,
  Shift+Tab and digits always change section; arrows and Enter go to the
  current screen. Avoids a separate "focus the sidebar" mode.
- **D13. Root-level scrolling with visible indicators.** Screens render at full
  height; the root clips to the terminal and swaps the first/last line for
  "more above/below" markers. PgUp/PgDn scroll. Nothing is cut off silently.
- **D14. Tab bar density tiers** (full names, short names, digits + current
  name) chosen by measured width so all nine shortcuts are always visible.
- **D15. Tests drive the model through `Update` with real key messages** and
  resolve returned commands synchronously, so persistence is asserted against
  the real config file. No snapshot tests of rendered output; only presence of
  key phrases, to avoid churn when styling changes.

## 2026-11-12 — Phase 2

- **D16. SQLite via modernc.org/sqlite v1.46.1.** Pure Go, so Windows builds keep
  `CGO_ENABLED=0`. v1.46.2 and later require Go 1.25; the toolchain here is
  1.24.7, so the version is pinned. Always build with `GOTOOLCHAIN=local` to
  stop `go get` from silently raising the go directive.
- **D17. Only the latest software list is kept.** Each saved scan replaces the
  list; scan summaries and activity are kept. "What changed since last scan"
  can add a second snapshot later without a schema rewrite.
- **D18. Failed, cancelled and "nothing to scan" results never replace the saved
  list.** A partial scan does replace it, and is flagged on Home, in My Software
  and in Activity History.
- **D19. Read files instead of running commands where possible.** dpkg status,
  apt extended_states, Scoop manifests and Chocolatey .nuspec files are read
  directly. That is faster and avoids version differences such as
  `choco list` changing meaning in Chocolatey v2.
- **D20. winget runs with `--disable-interactivity` and never accepts source
  agreements on the user's behalf.** If winget needs attention, the scan reports
  it as a source that couldn't be read.
- **D21. winget table columns come from header positions measured in terminal
  cells.** This handles localized headers and wide characters. Truncated IDs and
  local IDs (ARP\, MSIX\) are dropped because they can't be used later.
- **D22. Merge rule:** same normalized name and compatible version (equal, or
  one unknown, or a truncated prefix). The same program at two versions stays
  two entries. Earlier sources win; the registry is first on Windows.
- **D23. Apps vs system components** is a heuristic: libraries, runtimes,
  redistributables, apt auto-installed packages, required/important priority,
  snap bases, flatpak runtimes. Components are hidden by default and never
  deleted from the data. The heuristic is documented and tested.
- **D24. "Preview read-only actions" (`confirm_actions`)** can skip the preview
  only for actions that change nothing. Anything with `WhatChanges` always asks.
- **D25. Screens may opt into `SelfScrolling` and `InputCapturer`.** Lists manage
  their own cursor; the search box receives `q`, digits and `m` as text.
- **D26. Previews and help scroll** with arrows and PgUp/PgDn and show
  "more above/below" markers. Nothing is clipped silently.
- **D27. Saving a scan uses a non-cancellable context.** Once results are in,
  stopping or quitting can't leave a half-written list. SQLite transactions
  make each save atomic either way.

## 2026-11-12 — Phase 3

- **D28. Updates run in the user's terminal.** ATLAS hands the terminal to each package
  manager with Bubble Tea's exec support. The package manager prints its own output
  and asks its own questions. That is the only faithful way to support sudo
  passwords, Windows UAC, winget licence agreements and apt's own confirmation.
  ATLAS records exit codes and durations, not the output itself.
- **D29. ATLAS never answers for the user.** No `-y`, `--accept-package-agreements`
  or `--accept-source-agreements`. Package managers ask, and the user answers.
- **D30. Elevation is explained, never requested by ATLAS for itself.** On Linux,
  privileged commands are wrapped in `sudo` unless ATLAS already runs as root. On
  Windows, winget installers may show UAC. Chocolatey and global Scoop updates are
  blocked, with a reason and the command, unless ATLAS runs as administrator.
- **D31. Held packages are shown but never updated.** That covers winget pins and
  explicit-target packages, `choco pin`, `scoop hold` and `apt-mark hold`.
- **D32. Checks use each manager's existing metadata.** APT and DNF don't refresh
  their lists during a check because that needs root. Scoop compares with local
  buckets. If APT has never downloaded lists, the check fails with an explanation
  instead of reporting "up to date".
- **D33. `core.SourceError`** lets a source fail with a plain message and a technical
  detail, for example "No package information downloaded yet" with "run sudo apt update".
- **D34. Only the latest update check is kept.** Applied updates are removed from it.
  Failed and cancelled checks don't replace it; a failed check is kept in memory for
  the session so the Updates screen can explain what went wrong.
- **D35. Ctrl+C during updates** reaches the package manager as usual. ATLAS catches
  the signal, lets that command stop, starts nothing further, and records "stopped".
- **D36. Scans and update checks are mutually exclusive.** Only one background task
  runs at a time, and `x` stops it.
- **D37. Rollback is stated honestly per manager.** Snap and Scoop can revert. winget,
  Chocolatey, APT and DNF need a specific older version, if the source still has it.

## 2026-11-12 — Phase 4

- **D38. Analysis is a single, explicit action.** "Analyze My Software" reads the extra
  sources, applies the rules and saves the result. Smart Analysis and Dependencies both
  show the latest result. It has a preview and an Activity History entry like
  everything else, and it requires a saved scan.
- **D39. The analysis result is stored as one JSON document (schema v3).** It's derived
  data, always read and written whole, and its shape will grow. Only the latest is kept,
  and it is removed when the software list is cleared.
- **D40. Built-in support table, dated and sourced.** It covers Python, Node.js, .NET
  and Visual C++, as of 2026-11. Every finding shows its source so users can verify it.
  Month-only dates count as the end of that month. "Ending" means within 183 days.
  The table must be reviewed with each release.
- **D41. Java support isn't judged.** It depends on the vendor (Oracle, Temurin,
  Microsoft, distributions), so a single date would mislead.
- **D42. Distro-packaged runtimes past upstream end-of-life are "worth knowing", not
  "attention".** Distributions often keep patching them. The finding says so and
  points to the distribution's policy.
- **D43. No inference from missing data.**
  - Usage: only Windows UserAssist records with a timestamp count. Absence of a record
    is never "unused". Matching is by install folder, never a drive root.
  - Dependencies: when the source keeps no records (Windows installers), the runtime
    shows "not recorded" and "treat it as needed".
- **D44. Distro package names are compared exactly.** Versions inside names
  (`clang-18`, `python3.12-venv`) mark different packages, not versions of one. Other
  sources use a normalised product key that ignores versions and architecture.
- **D45. Only hard requirements count as dependencies** (dpkg Depends/Pre-Depends,
  RPM requires, Chocolatey dependency, Scoop depends). "Nothing else needs it" says that
  recommendations aren't counted.
- **D46. Database reads inside a cancellable task use an uninterruptible context.**
  Stopping applies to the slow external sources; a cancel at the very start must read
  as "stopped", never as a save error. This was found by the race detector's timing.

## 2026-11-12 — Phase 5

- **D47. Shared terminal handover.** The update job moved to `internal/handover`
  and serves updates and cleanup. Steps carry an action (update, remove, clean), so
  terminal output and summaries use the right verb.
- **D48. Destructive previews need `r`.** For removals and clean-ups, Enter only
  explains how to go ahead. A deliberate, different key stops muscle memory from
  confirming a deletion. Read-only previews still use Enter, and `r` doesn't confirm them.
- **D49. Fresh dependency check for every removal plan.** dpkg, RPM, Chocolatey and
  Scoop records are re-read at plan time, never taken from the saved analysis.
  Dependents that are part of the same plan don't block it.
- **D50. What ATLAS will never remove:**
  - packages the system marks Essential or required, or that DNF protects;
  - tools ATLAS or the OS rely on (sudo, package managers, winget/App Installer,
    kernels, bootloaders, Edge and WebView2, ssh server);
  - runtimes and system components with no dependency records;
  - software known to more than one Windows package manager, since ATLAS can't
    know which one owns it.
  Each block names its reason. Safety reasons are reported before "which tool" reasons.
- **D51. Program-only entries on Windows are removed with `winget uninstall --name
  <exact> --exact`.** That runs the program's own uninstaller, as Windows Settings
  would. Entries with a winget ID use `--id`.
- **D52. No flags that skip questions or delete data.** No `-y`, `--purge`, `--force`,
  `--delete-data` or silent switches. APT, DNF, Flatpak and Chocolatey ask for themselves.
  Settings and app data are kept: Scoop persist, ~/.var/app, APT config files, and
  Snap's 31-day snapshots.
- **D53. Clean-ups list their exact targets first.** APT autoremove targets come from
  `apt-get -s autoremove` (simulation, no root) and DNF's from `repoquery --unneeded`.
  At run time the package manager recomputes and shows its list before asking. That
  is stated in the preview.
- **D54. After removals, the saved list drops removed items, and Smart Analysis is
  marked stale** (latest removal is newer than the analysis).

## 2026-11-12 — Phase 6

- **D55. The catalogue is embedded YAML (`internal/discover/catalog.yaml`), dated and
  validated strictly at load.** A bad entry fails the test suite, not a user's session.
  Only free, widely used tools are listed, and only package IDs believed correct as of
  2026-11. Where unsure (for example GIMP's winget ID after 3.0, or 7-Zip on DNF), the
  package is left out rather than guessed.
- **D56. Every Windows tool must have a winget package**, the default on Windows 11.
  Linux prefers the distribution's own manager, then Flatpak, then Snap.
- **D57. Suggestions are local and explained.** They come only from the saved software
  list: "because you have X", or "useful for almost everyone" for a few essentials.
  Installed tools are never suggested. No network calls are made.
- **D58. Recognising installed tools** uses product keys ("+" becomes "plus", so
  Notepad++ is not Notepad) and package IDs per manager. Version stripping keeps leading
  digits ("7-Zip"); this was also fixed in Smart Analysis.
- **D59. Managers that can't install here are blocked during assessment, not at the
  last step**: Chocolatey without admin, APT without package lists, Flatpak without
  Flathub. That lets the next manager be chosen, and every block is explained.
- **D60. A missing Scoop bucket is added as a visible step in the preview.** Classic
  snaps say they get full system access.
- **D61. "Find New Tools" on Home opens Discover directly**, because opening a screen
  changes nothing. Installing still needs a preview. Install previews use Enter, like
  updates: installing isn't a deletion.
- **D62. Installed tools are remembered for the session** until the next scan adds them
  to My Software, so Discover doesn't offer them again.

## 2026-11-13 — Windows verification

- **D63. `atlas -selftest`** runs every read-only part of ATLAS against the real
  computer and writes a Markdown report. It uses a throwaway database in a temporary
  folder, so the user's ATLAS data is never touched, and it plans but never runs
  removals and installs. It's how ATLAS is verified on machines no one can drive
  interactively.
- **D64. Windows CI on windows-2025 and windows-2022.** Windows-only tests use the real
  registry, console processes and UserAssist. CI installs Scoop so those paths see real
  data, and publishes the tested `atlas.exe`.
- **D65. Programs installed through Chocolatey or winget appear twice on Windows** (the
  manager's record and Windows' own entry). They are joined when the product key matches
  and the versions are numerically equivalent (26.05 = 26.5.0). Different versions are
  not joined, because the evidence is ambiguous, and are reported as "listed twice …
  record may be out of date". Scoop apps are portable and never joined this way.
- **D66. Windows HRESULT exit codes are shown in hex,** and when a tool prints its error
  to stdout (winget does), that message is used. winget's pending source agreements
  (0x8A150046) get a plain explanation and instructions; ATLAS still never accepts them.
