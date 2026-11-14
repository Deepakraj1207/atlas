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
- **D67. winget's read of a Windows entry is joined regardless of version.** For
  desktop programs, `winget list` reads Windows' installed-programs entries and shows
  its catalogue's version, so a version mismatch says nothing. The join needs the name to
  belong to exactly one Windows entry and one winget entry, and skips MSIX and Store
  records. Items carry an `Origin` for this; it isn't saved.
- **D68. Scoop metadata is read under both names** (`scoop-manifest.json`, and
  `manifest.json` from older Scoop), and an installed app whose metadata can't be read
  is listed with the version folder's name instead of being skipped.
- **D69. `tools/conharness`** drives ATLAS in a real Windows pseudo-console. It's a
  separate module, so ATLAS doesn't depend on it.

## 2026-11-13 — Phase 7: release and AI explanations

- **D70. MIT licence, version 0.1.0.** Chosen by the user. `atlas -version` prints the
  version, commit and build date; release builds set them with `-ldflags`, other builds
  fall back to the Git details Go records.
- **D71. winget package `Deepakraj1207.ATLAS`, moniker and command `atlas`.** winget
  requires a `Publisher.Name` identifier, so a bare `atlas` (the user's wish) isn't
  possible; the moniker keeps `winget install --moniker atlas` working. A portable zip
  (`InstallerType: zip`, nested portable `atlas.exe`) needs no installer and no admin
  rights. `tools/release` writes the three manifests in the winget-pkgs layout; they
  pass `winget validate` on the user's Windows 11 PC. Submitting them to
  microsoft/winget-pkgs stays a manual step for the user.
- **D72. Linux packages are named `atlas-lifecycle`.** Fedora already ships a package
  called `atlas` (the ATLAS maths library numpy depends on); an .rpm of the same name
  would be treated as its upgrade. The command is still `/usr/bin/atlas`; dpkg and rpm
  refuse to overwrite another package's file. Packages are built with the distributions'
  own `dpkg-deb` and `rpmbuild`, not a third-party packager.
- **D73. Releases are drafts.** A `v*` tag builds, tests and checks every file (the zip
  runs and validates with winget; the .deb and .rpm install, run and remove on Ubuntu and
  Fedora) and creates a draft GitHub release. A person publishes it. `winget-check`
  then installs the published release from its manifests on a clean Windows machine.
- **D74. The catalogue check is manual.** `tools/catalogcheck` looks each Discover
  package ID up with its own package manager (Flathub through its public API) and
  reports found / missing / not checked; ATLAS never does this at runtime.
- **D75. Go 1.27.4.** Go 1.24 is out of support, and the official Anthropic Go SDK
  (v2) needs Go 1.25 or later. `GOTOOLCHAIN=local` still applies once 1.27.4 is
  installed. Other dependencies were not upgraded.
- **D76. AI explanations are off by default and always previewed.** When turned on in
  Settings, `e` on a Smart Analysis finding or an app in My Software opens a preview of
  the exact text that would be sent: names, versions, publisher, package ID and ATLAS's
  own wording, never install paths (which can contain the user's name) or the rest of
  the software list. Sending is listed under "What will change", so the preview appears
  even with read-only previews turned off. It uses the official Anthropic Go SDK,
  `claude-opus-5-5` (overridable as `ai.model`) at low effort, with Anthropic's
  server-side refusal fallback. The key comes from `ANTHROPIC_API_KEY`
  (or an `ant auth login` profile); ATLAS never stores it. Answers are labelled as written
  by AI, `x` stops a question, and every question (answered, failed or stopped) is
  recorded in Activity History with what was sent.

## 2026-11-14 — Engineering audit remediation

- **D77. Removal fails closed when dependency records can't be verified.** `PlanRemoval`
  used to discard each dependency source's status, so a source that failed or timed out
  looked like one that keeps no records, and an ordinary app was allowed through. Now every
  dependency source declares the inventory source it covers (`analysis.DependencySource`),
  its outcome is kept, and an item is refused when any source it came from that keeps
  records was unavailable, failed, timed out, was stopped, came back empty, or doesn't list
  the package. Every such source must agree nothing else needs the item. Items from sources
  that keep no records (Windows' installed-programs list, winget, Snap, Flatpak) keep the
  existing rule: apps allowed, runtimes and components refused. The same work fixed Scoop
  matching: its graph uses bare app names while inventory uses `bucket/app`, so Scoop
  dependents were never seen (`analysis.GraphKey`).
- **D78. RPM packages are changed with the distribution's own tool.** RPM used to be
  mapped straight to DNF for removals, updates, installs and clean-up, so openSUSE got
  `sudo dnf remove …`. `platform.RouteRPM` reads `/etc/os-release` (`ID`, `ID_LIKE`):
  Fedora, RHEL and relatives (Rocky, Alma, CentOS Stream, Amazon Linux) use DNF;
  openSUSE and SLES use Zypper, even when DNF is also installed. It goes by the
  distribution, not by whichever tool is on PATH. It refuses, with the reason shown,
  on yum-only systems, when the native tool is missing, on non-RPM distributions (an
  `rpm` binary on Ubuntu is never used to change packages) and on unrecognised
  systems. Zypper support: removal (`zypper remove`), update checks
  (`zypper --non-interactive --no-refresh list-updates`, read by column name) and
  updates (`zypper update`), never with `-y`/`--non-interactive`/`--clean-deps` on a
  changing command. DNF's protected list and tidiers apply only where DNF is the tool.
  Discover offers RPM installs only through DNF, since the catalogue has no Zypper
  names; openSUSE falls back to Flatpak rather than guessing names. The self-test
  report shows the detected distribution and route.
- **D79. Removals and updates are verified on the computer before they count.**
  `RecordResults` used to trust exit codes: a removal deleted the saved record and an
  update left the updates list as soon as the package manager exited 0. Now inventory
  takes a read-only snapshot of every source afterwards (`inventory.Service.Snapshot`,
  unmerged, not saved). A removal is confirmed only when no source lists the item any
  more (by package ID or name) and every source it came from was read. An update is
  confirmed when its manager's source shows the offered version, or a version different
  from before. Otherwise the saved record or update stays, and the activity becomes
  "…, but not all confirmed" with what was found ("Still installed: listed by …",
  "Not updated: still version …", "Couldn't check: … Their records are kept"). Without a
  verifier nothing is confirmed. The status line reports what was confirmed, not what
  exited 0. winget's "< 3.10.3" ("older than") is now kept by the scanner, because
  stripping it would have let a failed update look like it reached 3.10.3. Discover
  installs are not verified yet; the status already tells the user to scan again.

- **D80. Merging keeps two entries whenever it can't be sure they're one install.** A wrong
  merge hides an install, and removal then follows the surviving entry's source. Rules
  now: (1) only sources that can report the same copy merge: the Windows installer
  sources (Windows entries, winget, Chocolatey) with each other, and any source with
  itself. Scoop apps are portable copies and dpkg, rpm, Snap and Flatpak each install
  their own copy, so they never merge across managers (this was already intended for
  Scoop but the first pass ignored it). (2) Within one source, different package IDs or
  different publishers (ignoring "Inc", "Corporation", punctuation…) are different
  software. Publishers aren't compared across sources: Chocolatey reports authors.
  (3) A weak entry, with a name winget cut short or no version, joins only when exactly
  one entry fits it, whatever the order. The Chocolatey/winget-to-Windows join and
  winget's read of a Windows entry also need a one-to-one fit, so a record matching
  both the x64 and x86 editions stays on its own. (4) When a weak entry from a
  higher-priority source joins, it stays the primary record. The cost is an occasional
  duplicate line, which is visible and harmless; a false merge isn't.

- **D81. AI explanations: local option, explicit billing, minimised data, a cost cap.**
  Still off by default, previewed every time (even with read-only previews turned
  off), recorded in Activity History, and advisory only: the explainer port has no way
  to run anything. Changes:
  (1) **Ollama** as a second provider (`ai.provider: ollama`, `ai.model`,
  `ai.ollama_url`), through Ollama's documented `/api/chat` HTTP API with the standard
  library: no new dependency. It connects straight to the address shown (no proxy, no
  redirects). A loopback address is described as "doesn't leave your computer" and
  free; any other host is named as such, never called local. Ollama is never picked
  automatically and a bad address never falls back to Anthropic.
  (2) **Billing** is spelled out in the preview's "What will change" and in Settings:
  billed to the user's Anthropic API account by the token, not covered by a Claude.ai
  subscription, with the worst case for one question at the default model's published
  price (US$4/US$20 per million tokens, Nov 2026: at most about US$0.09 with the default
  limit). No price is invented for other models.
  (3) **Minimised data.** `ExplainService.Prepare` replaces the home folder with `~`
  and the user and computer names (whole words, 3+ characters, not generic names like
  root) with `[user]` / `[computer]`. It caps each line at 300 characters and the
  question at 2,000. The preview shows the prepared text and that exact text is sent.
  (4) **`ai.max_output_tokens`** (default 4096, allowed 1024–16000) replaces the
  hard-coded 16000. The floor exists because Claude's reasoning counts towards the
  limit and can't be turned off on the default model; the ceiling keeps a request
  without streaming under the SDK's timeout. An answer that hits the limit is kept and
  marked "cut short"; one with no text says how to raise the limit.

## 2026-11-14 — Interface redesign

- **D82. A quieter, modern terminal look, with the same features and keys.** Inspired by
  current AI command-line tools (no copied branding or text). Same Bubble Tea, Bubbles and
  Lip Gloss; no new dependency.
  - **Look.** Dark-first palette with one accent (periwinkle) and status colours used only
    for status. Text weight and dim/bright carry the hierarchy, and boxes are kept for the
    few places that need them. Only symbols already confirmed to render in Windows Terminal
    and conhost are used (`◆ › ● ○ • │ ─`), no emoji, because those are double width.
  - **Shell.** A one-line header (`◆ ATLAS`, version, a status dot with plain words, the
    computer, the mode) over a thin rule. A borderless sidebar with `›` on the current
    section. A footer without a box: `● message` with real figures on the right (apps,
    updates, memory in use), then key hints.
  - **Focus.** Esc is "back": it closes what a screen has open (a search, the details
    view), then moves focus to the section list, where ↑ ↓ move and Enter, → or Esc
    return. Digits, Tab and `/` work everywhere, as before. A screen with nothing to show
    yet does its suggested action on Enter (scan, analyze, check, look for things to tidy),
    always through its preview (0.2.1, after "Smart analysis not working" on Windows). Focus starts on the screen,
    so every existing key keeps its meaning. A screen keeps Esc for itself only while
    `HandlesBack` says it has something open.
  - **Dashboard** (Home, renamed in the interface only). A "What would you like to do?"
    list (scan, check for updates, analyze, find unused apps, clean up, discover,
    search); the first three still preview first. The computer at a glance comes from
    the new read-only `platform.ReadSystemInfo`: Windows name from the registry (build
    22000 and later is Windows 11), memory from `GlobalMemoryStatusEx`, disk from
    `GetDiskFreeSpaceEx`, and on Linux `/etc/os-release`, `/proc/meminfo` and statfs.
    Figures are real or shown as "—" (not known yet), never 0 by default. The health
    words ("Not scanned yet", "Last scan had gaps", "N need attention", "System ready")
    only state what the saved data shows. On wide terminals the actions and the glance
    sit side by side, so the important part fits Windows Terminal's default 120×30.
  - Memory use is read again every 15 seconds; nothing else polls.
  - **Big logo.** ATLAS in its own three-row block letters, drawn only with `█ ▀ ▄`
    (single width in every Windows console font, legacy conhost included), with a
    gradient from the accent colour to the info colour. It sits beside the welcome on wide
    terminals, so it costs no rows at 120×30. It goes above the welcome on narrow but tall
    terminals and is left out on small ones. (A six-row version for large windows was
    tried and removed: the owner preferred the three-row logo at every size.)

## 2026-11-14 — Installing ATLAS

- **D83. One-line installers instead of winget.** The owner chose a single command per
  system over a winget package, which needs Microsoft's review before anyone can use it.
  `install.ps1` (`irm …/install.ps1 | iex`, Windows PowerShell 5.1 and 7) and
  `install.sh` (`curl -fsSL …/install.sh | sh`, POSIX sh, curl or wget) do the same
  thing:
  - **Download.** They fetch the latest release, or `ATLAS_VERSION`, for the processor
    (x64 or ARM64).
  - **Checksum.** They check the download against the release's `SHA256SUMS` and refuse
    on any mismatch, leaving an existing install untouched.
  - **Per-user install.** They install for the current user only, with no sudo or
    administrator rights:
    - Linux: `~/.local/bin`. If that folder isn't on PATH, the installer prints the line
      to add rather than editing shell files.
    - Windows: `%LOCALAPPDATA%\Programs\ATLAS`, added to the user PATH. The PATH is
      read and written unexpanded, our entry is appended, and uninstall removes only that
      entry, so the PATH is restored byte for byte.
  - **Update and uninstall.** Running again updates; on Windows a running `atlas.exe` is
    renamed aside, so it keeps working. `ATLAS_UNINSTALL=1` removes the program and,
    on Windows, its PATH entry. Settings and data are always kept.
  - **No partial runs.** All work happens inside one function or script block, so a
    cut-off download runs nothing. On Windows, an error never closes the user's
    PowerShell window.

  winget support for the user's *other* software (scanning, updates, removal, Discover)
  is unchanged. `tools/release` (winget manifests) and the `winget-check` workflow are
  removed; `install-check` replaces it.

