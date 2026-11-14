# ATLAS — Test Status

Last run: 2026-11-13, Phase 7, Go 1.27.4 on linux/amd64.

## Unit tests (`go test ./...`) — all passing, 197 tests in 22 packages (plus Windows-only tests)

Phase 7 added:

| Package | Tests | Covers |
|---------|-------|--------|
| internal/ai | 6 | the request carries exactly the previewed question plus fixed instructions, `claude-opus-5-5`, low effort, the refusal fallback and its beta header, and no thinking override (against a local fake of the Messages API); 401, 402, 404, 429, 529, refusal, empty answer and an unreachable server each give a plain message; credentials detection; every question (answered, failed, stopped) is recorded in Activity History |
| internal/tui (phase 7) | 6 | AI off by default (no hint, nothing sent); no key: explained, nothing sent; preview shows the exact text and appears even with read-only previews off; Esc sends nothing; Enter sends exactly what was shown; the answer is labelled as AI-written; errors explained; Settings toggles and saves; finding questions leave out file paths |
| internal/config | +1 | AI off by default; enabled and model read; a bad model name ignored with a warning; saved |
| tools/release | 2 | winget manifests: layout, identifier, zip + nested portable, alias and moniker, SHA-256 and URLs, release date, licence; refuses a zip without `atlas.exe` at the top, bad versions or dates, missing files |
| tools/catalogcheck | 2 | found / missing / not checked per manager (winget, Scoop bucket files, Chocolatey, Flathub, a manager that can't run); report and exit count |
| cmd/atlas | 1 | version line from build settings or Go's Git details |

Release files, built locally with `packaging/build-release.sh`: both zips (with `atlas.exe`
at the top), both tarballs, both `.deb` and both `.rpm` (the arm64 `.rpm` holds the arm64
binary unchanged), winget manifests and SHA256SUMS, which all verify. The manifests pass
`winget validate` on the user's Windows 11 PC. Not yet done: a live AI request (needs the
user's API key), and installing a published release through winget (after publishing).

| Package | Tests | Covers |
|---------|-------|--------|
| internal/discover | 9 | built-in catalogue validates and every Windows tool has winget; 7 kinds of bad entry rejected; product keys (Notepad++ vs Notepad, 7-Zip); assessment on Windows (installed by name and by package ID, Linux-only hidden, suggestions because of Git and Python, everyone-tools, installed never suggested); on Linux (Windows-only hidden, native first, Flatpak fallback) and before a scan; install plans (exact winget command without agreement flags, already installed blocked, Scoop bucket added visibly, sudo APT without -y, classic snap explained, no Flathub blocked, package notes, APT without lists not used, Chocolatey only as admin); recording |
| internal/tui | 47 | earlier phases plus: Find New Tools opens Discover with no preview; suggestions with reasons, no Linux-only tools on Windows; install previews first, runs exactly the winget command on Enter, records, leaves suggestions; installed tools can't be selected or installed again; multi-select preview, Esc runs nothing; before a scan nothing is installable and `s` offers a scan; Discover fits 6 terminal sizes |
| internal/cleanup | 12 | Phase 5 coverage |
| internal/analysis | 13 | earlier coverage, plus leading digits kept in product keys |
| internal/handover, updates, database, inventory, core, config, logging, platform | 64 | earlier phases, plus install verbs |
| internal/tui/nav, layout, theme, components | 18 | all nine sections available; layout, themes, components |

### Race detector

`go test -race` over every package that uses goroutines (tui, inventory, updates,
analysis, cleanup, handover, discover) reports no data races and no failures. An earlier race-detector run
found a real timing bug (D46), which is now fixed and has a regression test.

## Checks

| Check | Result |
|-------|--------|
| `gofmt -l .` | clean |
| `go vet ./...` (linux) | clean |
| `GOOS=windows GOARCH=amd64 go vet ./...` | clean |
| `go test -count=1 ./...` | pass |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~14.7 MB) |
| `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~14.0 MB) |
| Linux build | pass |

## Real Windows (GitHub Actions: Windows Server 2025 and 2022)

Workflow `.github/workflows/windows.yml` runs `go vet`, the full test suite and
`atlas -selftest` on both images. Scoop is installed first, with `jq` and `fd`, so
its code paths see real data. Both images use the same Windows APIs as Windows 11,
but they aren't an interactive desktop.

Windows-only tests (all pass on both images):

| Test | What it proves |
|------|----------------|
| TestRegistryScanReadsThisComputer | real Uninstall keys read: 59 programs, machine and user scopes, dates as YYYY-MM-DD |
| TestDefaultScannersRunOnWindows | registry, winget, Chocolatey (44 packages) and Scoop scanners run together |
| TestExecRunnerOnWindows | real commands, output and exit codes |
| TestExecStarterExitCodesOnWindows, TestExecStarterRunsCmdShimsFromPath, TestExecStarterMissingProgram | the terminal handover runs `.cmd` shims through PATH (as Scoop and Chocolatey install them) and reports exit codes and missing programs |
| TestUserAssistReadsThisComputer | Windows' usage records decoded with plausible timestamps |
| TestWindowsFolders | settings in %AppData%\atlas, data and logs in %LocalAppData%\atlas |

Problems the first run found, all now fixed and covered by tests:

1. **Chocolatey and Windows listed the same program twice** ("7-Zip (Install)" 26.5.0
   and "7-Zip 26.05 (x64)"). That produced 7–9 false "installed more than once"
   findings per image, and would have planned two removals for one program. They are
   now joined when the product name matches and versions are equivalent. Remaining
   pairs with different versions say "listed twice … record may be out of date".
2. **winget failures showed only "exit code 2316632134".** That is `0x8A150046`:
   source agreements haven't been accepted on the 2025 image. ATLAS now says
   "Windows Package Manager is waiting for you to accept its source agreements" and
   how to do it. It never accepts them itself.
3. Analysis sources always reported "1 found"; they now report packages and records.
4. "Support for Node.js 22 ends in 30 Apr 2027" now reads "ends on".
5. **"Analysis is out of date" could fail to appear on Windows Server 2022.** Two scans
   finished within the same clock tick and got identical timestamps, so a "newer than"
   comparison saw no change. Staleness is now decided by which scan the analysis was
   built from (scan ID) and by the order of activity entries, never by timestamps.
6. **The self-test report listed a combined removal command against each item.** For
   example, `7-Zip: choco uninstall 7zip.install ant apache-httpd`, which reads as though
   removing 7-Zip also removed Ant and Apache. The planner was right to remove the three
   in one step; the report now prints one line per step. Discover's count no longer
   prints Go's raw `map[winget:22]` formatting.

Self-test on the 2025 image after the fixes: 81 apps, 16 components; 11 Chocolatey
updates with friendly names; .NET 10 and Node.js 22 detected; the removal check
routes 7-Zip to `choco uninstall` and blocks .NET and Visual C++ runtimes; Discover
recognises 7-Zip, Firefox, Git, GitHub CLI, Node.js and jq. The only failures are
winget, for the reason above (16 passed, 2 failed: the scan and the update check).

## Engineering audit remediation (2026-11-14)

**P0 — cleanup dependency safety.** Regression tests written first and run against the old
code, where every case below allowed the removal (the step had a remove command):

| Test | Proves |
|------|--------|
| TestGateFailsClosedWhenDependenciesCantBeVerified | an app from dpkg is refused when its dependency source failed, timed out, was unavailable, came back empty, or didn't list the package; no command is planned |
| TestGateStillAllowsAppsWhenEvidenceIsComplete | complete records still allow an app nothing needs, still block one something needs, a failure in an unrelated source doesn't block, winget apps unchanged |
| TestGateMatchesScoopAppsByBareName | Scoop dependents are found (`main/7zip` needed by innounp), unneeded apps allowed, and removing both together allowed |
| TestUnverifiedLabelWithoutName | messages never crash on a source without a label |
| TestGraphKeyAndCoverage (analysis) | graph names per source; every real dependency source declares what it covers |

Verification: gofmt clean; `go vet ./...` (Linux and `GOOS=windows`); `go test ./...` all
pass; `go test -race` on cleanup, analysis, inventory pass; Windows and Linux builds pass.

**P0 — Linux package-manager routing.**

| Test | Proves |
|------|--------|
| TestParseOSRelease, TestRouteRPMPerDistribution (platform) | real os-release files: Fedora 41, RHEL 9, Rocky 9, Amazon Linux 2023 → dnf; openSUSE Tumbleweed, Leap 15.6, SLES 15 → zypper (also with dnf installed); CentOS 7 (yum), missing native tool, Ubuntu/Arch with rpm or dnf present, unknown and missing os-release → refused with a reason |
| TestRPMRemovalFollowsTheDistribution (cleanup) | openSUSE plans `sudo zypper remove`; Fedora `sudo dnf remove`; refused and unconfigured routes block with the reason and no command |
| TestDnfProtectedListOnlyAppliesToDnf (cleanup) | DNF's protected list isn't applied to Zypper systems |
| TestNoAssumeYesPurgeOrDataFlags (cleanup, extended) | Zypper removal has no `-y`, `--non-interactive`, `-n`, `--clean-deps` |
| TestZypper, TestRPMRefusedSaysWhy (updates) | zypper table parsed by column name (also reordered), "No updates found", unknown output is an error, `sudo zypper update` without auto-confirm flags, matched to RPM inventory; refused RPM checks say why |
| TestManagersFollowTheDistribution (updates) | Fedora: apt dnf snap flatpak; openSUSE: apt zypper snap flatpak; CentOS 7: a "not checked" RPM entry; Ubuntu: no RPM manager |
| TestRPMInstallsFollowTheDistribution (discover) | an RPM scan offers DNF installs only where DNF is the distribution's tool |

Real detection here (Ubuntu 24.04 with `rpm` installed): "RPM packages changed with: not
changed: Ubuntu 24.04.5 LTS doesn't manage its software with RPM". The `linux` workflow
runs the self-test inside real fedora:41, rockylinux:9, opensuse/tumbleweed,
opensuse/leap:15.6, centos:7 and ubuntu:24.04 containers and checks each route.

Verification: gofmt clean; `go vet ./...` (Linux and `GOOS=windows`); `go test ./...` all
pass; `go test -race` on cleanup, updates, discover, platform pass; Windows amd64, Linux
amd64 and arm64 builds pass.

**P0 — post-operation verification.** Tests show that before this change an exit code of 0
deleted the saved record or the pending update even when the software was still there.

| Test | Proves |
|------|--------|
| TestVerifyRemoved (inventory) | gone everywhere → confirmed; still listed by winget (package ID) or another copy by name → not done; one of its sources failed, timed out, unavailable or not checked → unknown |
| TestVerifyUpdated (inventory) | new version → confirmed; same version (also written differently) → not done; a different version → confirmed; no longer listed or unreadable → unknown; with no previous version, only the offered version confirms and "< 3.10.3" doesn't |
| TestWingetKeepsOlderThanMarker (inventory) | the scanner keeps "< 3.10.3"; merging still pairs it with the Windows entry |
| TestSnapshotReadsEverySourceWithoutSaving (inventory) | every source read, failed ones contribute nothing, nothing saved as a scan |
| TestRemovalIsVerifiedBeforeTheRecordGoes (cleanup) | exit 0 but still installed → record kept, activity "Removed 1 item, but not all confirmed" with "Still installed: listed by …"; source unreadable → kept, "Couldn't check … records are kept"; no verifier → kept; confirmed → deleted, success |
| TestUpdatesAreVerifiedBeforeLeavingTheList (updates) | still the old version → stays in Updates, partial, "still version 1"; unreadable or no verifier → stays; confirmed → removed, success |
| TestResultStatusReflectsVerification (tui) | status lines say how many were confirmed instead of reporting success from exit codes |
| TestUpdateAllPreviewsThenRunsAndRecords, TestRemoveFromMySoftwareNeedsDeliberateConfirmation (tui, updated) | end to end: "Installed and confirmed 2 updates", "Cleanup finished and checked" |

Verification: gofmt clean; `go vet ./...` (Linux and `GOOS=windows`); `go test ./...` all
pass; `go test -race` on tui, cleanup, updates, inventory pass; Windows and Linux builds pass.
CI on a9604ab: `windows` (run 43570476419) and `linux` (run 43570476398) passed.

**P1 — deduplication robustness.** The scenarios from the audit were written as tests first.
Against the old `merge.go`, 9 cases joined software that may be different installs:

| Test | Proves |
|------|--------|
| TestMergeDoesNotJoinDifferentSoftware | kept apart: same name with different publishers; editions (Community/Professional, 32/64-bit Python); a Scoop portable copy beside an installed one at the same version*; different versions; localized Office installs; two winget package IDs with the same name*; a dpkg and a Snap Firefox*; an rpm and a Flatpak GIMP*; an unversioned Scoop app beside two installed versions*; a Chocolatey record fitting both Python editions*; a truncated winget name fitting both the x64 and x86 Visual C++ entries, in either order* |
| TestMergeStillJoinsTheSameInstall | still joined: winget's read of a Windows entry; a truncated winget name with one candidate; the Chocolatey record of the same install; localized names spelled the same; case and spacing differences; a publisher written two ways ("inc." vs "Inc"); the same Windows entry seen twice |
| TestWeakMatchKeepsTheHigherPrioritySource | a Windows entry without a version stays the primary record (its uninstaller is still used) and is filled in from winget |
| TestNormPublisher | publisher suffixes (Inc, Corporation, Ltd…) and punctuation are ignored, nothing else |
| TestMerge (updated) | its old expectation was the bug: an unversioned Scoop "Python" was joined to whichever of two Windows Pythons came first; now it stays separate |

\* failed on the old code (also the publisher case). Existing merge tests (7-Zip/AWS CLI with
Chocolatey, PyCharm/ONLYOFFICE/Safe Exam Browser read by winget, Edge twice, Tool at three
versions, `< 3.10.3`) pass unchanged.

Verification: gofmt clean; `go vet ./...` (Linux and `GOOS=windows`); `go test ./...` all
pass; `go test -race` on inventory, cleanup, updates pass; Windows amd64 and Linux amd64
builds pass.
CI on 927a99e: `windows` (run 43570907369) and `linux` (run 43570907907) passed.

**P1 — AI privacy and local inference.** Gaps confirmed in the old code: the answer limit
was hard-coded at 16,000 tokens; an answer cut off at that limit was shown as complete;
a user or computer name inside a software name was sent as is; billing was one clause
("may be charged a small amount"); only Anthropic was possible.

| Test | Proves |
|------|--------|
| TestPrepareRemovesIdentifyingDetails (ai) | home folder (either slash, any case) becomes `~`, user name `[user]`, computer name `[computer]`, as whole words only ("Priya.Ramanathan Ltd" kept); generic names (root, short names) untouched; idempotent; long lines and questions shortened visibly to 300 / 2,000 characters |
| TestExplainSendsWhatThePreviewShows (ai, extended) | default `max_tokens` is 4096, not 16000 |
| TestAnswerLimitIsConfigurable (ai) | `WithMaxOutputTokens(2048)` is sent; a `max_tokens` stop keeps the text and marks it cut short; with no text, the error names `ai.max_output_tokens` |
| TestClaudeBillingIsExplicit (ai) | billed to the Anthropic API account by the token, not covered by a Claude.ai subscription, the cap, and at most about US$0.09 at claude-opus-5-5's price; no price invented for other models |
| TestOllamaSendsWhatThePreviewShows (ai) | `POST /api/chat` with the model, the same instructions and the question, `stream: false`, `num_predict` = the limit, and nothing else; `<think>` blocks removed from the answer |
| TestOllamaProblemsArePlainLanguage (ai) | model not pulled → "Run "ollama pull llama3.2" first"; server error; not Ollama; limit reached with and without text; not running → "Is Ollama running?" |
| TestOllamaDoesNotFollowRedirects (ai) | the question never goes to a host other than the one shown |
| TestOllamaProviderAndReady (ai) | localhost, 127.0.0.1 and ::1 are local and free; a LAN host is named and never called local or free; no model → not ready, says to set `ai.model`; only http(s) addresses |
| TestServiceNamesTheProviderAndCutAnswers, TestServiceRecordsEveryQuestion (ai) | Activity History names the provider and host and the answer limit, and notes a cut-short answer; a local model gets 5 minutes |
| TestAIProviderAndLimits (config) | defaults; Ollama settings load and survive saving; unknown provider, wrong model format per provider, limits below 1024 or above 16000, non-http and credential-bearing Ollama addresses → warning and safe value |
| TestNewExplainerFollowsSettings (app) | settings pick the provider and limit; an unusable Ollama address is never ready and never falls back to Anthropic |
| TestAIExplanationPreviewsExactlyWhatIsSent (tui, extended) | the preview shows the prepared text, the destination, "Cost: … billed …", "user name and computer name are replaced", "ATLAS never runs anything it suggests"; the prepared text is what's sent |
| TestLocalAIPreviewSaysNothingLeavesTheComputer (tui) | with Ollama: "It doesn't leave your computer", "Cost: Free", no mention of Anthropic, answer labelled "Ollama's explanation" |
| TestAIExplanationIsOffByDefault, …NeedsAKey, …ErrorIsExplained, TestSettingsTurnsAIOnAndOff (tui) | unchanged and passing: off by default, nothing sent without the preview and Enter |

Not verified here: a real Ollama server and model (none can be installed or downloaded in
this environment). The Ollama adapter is tested over real HTTP against a server that
answers in the documented `/api/chat` format.

Verification: gofmt clean; `go vet ./...` (Linux and `GOOS=windows`); `go test ./...` all
pass; `go test -race` on ai, tui, config pass; Windows amd64, Linux amd64 and arm64 builds
pass.

## Interface redesign (2026-11-14)

| Test | Proves |
|------|--------|
| TestEscMovesFocusToTheSectionsAndBack (tui) | the screen has focus at start (↓ moves the Dashboard list); Esc moves focus to the sections, where ↓ ↓ ↑ moves and keys don't reach the screen; Enter opens; a digit jumps and returns focus; Esc again returns |
| TestNarrowTerminalsMoveThroughTabs (tui) | with the tab bar, ← → move between sections |
| TestEscClosesTheScreensOwnThingsFirst (tui) | `/` from the Dashboard opens My Software's search; Esc clears the search, then closes the details view, and only then moves to the sections |
| TestSearchActionOpensMySoftwareSearch, TestDashboardActionsOpenTheirSections (tui) | each Dashboard action opens its section (Smart Analysis, Cleanup, Discover Apps, the search) without a preview; scan, check and analyze still preview first |
| TestHeaderAndStatusBarShowRealFacts (tui) | header shows `◆ ATLAS`, `v0.1.0`, "Not scanned yet", the computer and the mode; footer shows memory in use; no app or update counts before a scan or check; after a scan "● System ready" and "2 apps" |
| TestUnknownSystemFactsAreLeftOut, TestDashboardFiguresAreRealOrMarkedUnknown (tui) | unknown memory and disk aren't shown; figures are "—" before a scan, real counts after; disk shown as "200 GB free of 500 GB on C:" |
| TestViewNeverExceedsTerminalWidth (tui) | no line wider than the terminal at 8 sizes from 40×10 to 200×50, in every section, with and without section focus |
| TestReadSystemInfoIsHonest, TestGBAndMemUsed, TestParseMeminfo (platform); TestWindowsName (platform, Windows only) | real system facts are consistent (available ≤ total); Windows 11 is recognised by build number; formatting |
| TestStatusMessageWinsOverFigures (components) | found by Windows CI: the right-hand figures cut off "See Activity History (8)." at the end of a status message; now the figures give way whenever the message wouldn't fit in full (fails on the first version, passes now) |
| TestBigLogoWhereThereIsRoom (tui), TestBigLogoShape (components) | the block-letter logo sits beside the welcome at 120×30, above it at 60×40, is left out at 60×20, and stays the same on large windows (160×45); every row is exactly as wide as the logo, uses only `█ ▀ ▄`, in both themes |
| TestDotsDifferByTone (theme) | each status tone has its own colour, and "unknown" (○) differs from "good" (●) even without colour |

Updated for the new interface: help context "Dashboard:", narrow tab "1 Dashboard",
Dashboard content checks read the whole Dashboard (it scrolls at 120×30), the
"Find New Tools" test became TestDashboardActionsOpenTheirSections. The conharness script
`ui-basics.txt` waits for "1 Dashboard".

Real binary in a pseudo-terminal (120×30, true colour): Dashboard and header shown,
Esc then ↓ moves to My Software with the section hints, Enter and `/` open the search,
q exits with 0.

## Real Windows 11 PC (DESKTOP-41IVS1K, Windows 11 Home build 26300, not administrator)

Reached through Desktop Commander. ATLAS was built on the PC with a portable Go in
`%USERPROFILE%\atlas-test` (Go's module mirrors are blocked on that network, so modules
were fetched from their repositories; `go.sum` still verified them). The interface was
driven by `tools/conharness`, which runs ATLAS in a real Windows pseudo-console (ConPTY,
as Windows Terminal does), sends keys and records the screen. ATLAS used a separate data
folder (`ATLAS_HOME`), so the user's own ATLAS data wasn't touched.

- **Self-test: 15 passed, 0 failed.** 67 Windows entries, 158 winget entries, Scoop;
  130 apps after merging; 28 winget updates; 188 UserAssist records; Discover recognised
  10 installed tools. winget's agreements are accepted here, so winget worked (CI can't
  show that).
- **Interface (checks 2.1–2.13 preview only, 2.19, 2.20): all screens work.**
  Box-drawing characters and `›`, `•`, `→`, `✓` render; theme toggle, help, Tab bar at
  72×24, compact layout at 40×12, scan (125 apps in 4.7 s), search, Advanced mode
  (package IDs), update check (13 s), Updates list, analysis, Dependencies, Cleanup
  search, Discover, Activity History; `q` exits with code 0 and leaves the window clean.

Problems found there, all fixed and covered by tests:

1. **Scoop found 1 of 3 apps.** Newer Scoop names its files `scoop-manifest.json` and
   `scoop-install.json`; ATLAS only read the old names and skipped the rest silently.
   So 7-Zip (installed with Scoop) was missing, and Discover offered to install it. All
   four Scoop readers accept both names and a byte-order mark now, and an app whose
   metadata can't be read is still listed.
2. **Five false "listed twice with different versions".** winget list reads desktop
   programs from the same Windows entries but shows its catalogue's version (PyCharm
   262.10315.174 vs 2026.2.2). These are joined now when the name is unique on each side.
   Windows app packages (MSIX) and Store entries are not joined this way.
3. **.NET Native Framework packages were treated as apps,** flagged as "several
   versions" and offered for removal in Cleanup. They're system components now.
4. **"Safe Exam Browser 3.10.3 → 3.10.3".** winget's `< 3.10.3` means "older than its
   catalogue"; the exact version now comes from the saved list, with a note.
5. **"Antigravity 2.29.0 2.29.0".** A version the name already ends with isn't repeated.
6. **The too-small message was cut off at 39×9;** it wraps now. Also "Found 1 things",
   and the Updates hint now says `u` includes the hidden system components.

**Re-checked on the PC after the fixes (commit 60ac12b):** `go test ./...` passes on
Windows 11; self-test 15 passed, 0 failed; both conharness scripts pass in strict mode.
Scoop now finds 3 apps (7-Zip recognised as installed in Discover); no false "listed
twice" findings and no framework findings remain; Safe Exam Browser reads 3.10.2.920 →
3.10.3; "Antigravity 2.29.0 → 2.31.2"; "Press u to update all 28: 25 apps and 3 system
components"; "Found 1 thing". The one remaining note, "Microsoft Edge is listed twice",
is accurate: Windows keeps an older Edge app package (152) beside Edge itself (157).

**Interface checks on CI.** The same two conharness scripts (`tools/conharness/scripts`)
run on windows-2025 and windows-2022 on every push. On 2025, winget hasn't accepted its
agreements, so scans finish "with gaps"; the scripts accept either outcome.

**Changing checks on the PC, with the owner's agreement and the owner at the PC:**

- **jq install and removal (2.13–2.16), driven by conharness.** Discover's preview
  showed `winget install --id jqlang.jq --exact --source winget` with no agreement
  flags; the terminal was handed to winget (download, hash check, "Successfully
  installed"); ATLAS said "Installed 1". After a rescan, the removal preview showed
  `winget uninstall --id jqlang.jq --exact`; Enter only said "press r"; `r` removed it;
  "Cleanup finished"; a rescan found no jq, and winget agrees.
- **UAC (2.17) with Notepad++, three times.** The installer asked Windows for
  permission (consent.exe ran); ATLAS never asked for it itself.
  - Declined: MSI 1602, winget 0x8A15010C. ATLAS then said "Installs didn't work (exit
    code 2316632332)". Fixed: hex codes everywhere, and declining is recorded as
    cancelled by you (Discover says "Install cancelled"). Covered by unit tests; not yet
    re-declined live.
  - Approved: "Installed 1", Discover marks it installed, Activity records the command.
  - Removal approved: "Windows may ask for permission if the uninstaller needs it",
    "Successfully uninstalled", "Cleanup finished". Notepad++ is gone again.
- Also fixed from these runs: each handover starts with a timestamped rule (the second
  handover appeared under the first one's output), "uninstaller" wording for removals,
  and "Scan again to add it" for one app.

Not done: Ctrl+C during a real install (2.18, not agreed), re-declining UAC with the
new build, and a visual check in the legacy conhost window (section 3).

## End-to-end runs in a pseudo-terminal

**Phase 6, real binary.** Scan, then Home's "Find New Tools", which opened Discover Apps
directly. Windows-only tools were hidden. This container has APT with no package lists
and no Flatpak or Snap. Enter on Firefox (not offered through APT) named the managers
that offer it. Enter on btop (offered through APT) said APT has no package information
yet and to run `sudo apt update` first. Nothing ran, and the log has no install entries.

**Phase 5, real binary, real data, nothing removed.**
1. Scan, then My Software, a search for `bc`, and `r`. The real dpkg dependency check
   ran and the preview read "Remove bc" with "Press r to go ahead".
2. Enter was refused with "To go ahead, press r", and Esc cancelled.
3. `python3.12` was blocked, naming the packages that need it.
4. `dpkg -s bc` still reported "install ok installed" afterwards, and the log shows two
   plans and no run. The run itself is covered by the TUI tests and the Phase 3
   handover verification.

Live, read-only planning on this container: `bc` was removable, `bash` was blocked as
something the system relies on, and `python3.12` was blocked as needed by `python3`,
`python3-uno`, `python3.12-dev` and `python3.12-venv`. The APT cache was empty, and APT
reported nothing autoremovable.

**Phase 4, real binary, real data.** Scan, then Analyze My Software (previewed), on this
Ubuntu container. 837 packages were analyzed in about 14 ms. The 7 runtimes found were
Python 3.11, 3.12 and 3.13 and four Java 21 packages. For example, python3.12 is needed
directly by 4 packages and by 38 more through them, from dpkg records. The only finding
was "Updates haven't been checked yet". "Apps not opened in over a year" was reported
as not checked, because Linux keeps no record. Both screens rendered, and q exited with 0.

A first live run surfaced 14 false "several versions" findings for distro packages such
as `clang`/`clang-18` and `python3.11-venv`/`python3.12-venv`. Fixed by comparing distro
package names exactly (D44), with a regression test.

**Real binary, real APT.** Check for Updates opened its preview, then ran. This
container has never downloaded APT package lists, so the check failed with
"No package information downloaded yet … run 'sudo apt update'" rather than
claiming everything was up to date. DNF, Snap and Flatpak reported "not on this
computer". The schema-2 database, settings and log were written, and q exited with 0.

**Terminal handover, scratch harness with a stand-in package manager**, a shell
script that asks "Continue? [Y/n]":
1. Check and Updates listed the update. The apply preview showed it and ran nothing.
2. On Enter the terminal was handed over. The child saw a real TTY, printed its
   prompt, and read the typed answer.
3. "Press Enter to return to ATLAS" appeared. Enter returned to the TUI with
   "Installed 1 update". Activity History had it and the pending list was empty.
4. Ctrl+C during a 30-second step stopped it within 1.5 s. The second step never
   started, ATLAS survived and returned, and "Updates stopped" was recorded with
   both packages listed as not finished.

## Manual rendering review

The Updates list (Simple, Advanced, with a selection, with components shown), the
apply preview (top, bottom, Advanced) and Home were rendered at 120×30, 80×24 and
60×18. Every view fit its terminal. Fixed during review: header misaligned with the
checkbox column, a doubled period in rollback text, and Home counting apps while
the recommendation counted everything.

## Not verified in this environment

- **Changing steps on Windows 11.** The read-only checks ran on the user's PC (see above).
  Still to do there: UAC prompts from installers, Ctrl+C during a real install, a real
  winget install and removal of `jq`, and legacy conhost.

- That every catalogue package ID resolves today. That needs network access to each
  package source. IDs are believed correct as of 2026-11, and a wrong one fails with
  the package manager's own message.
- A real install. The handover path is the one verified in Phase 3.

- A real removal or clean-up run on this machine. It was deliberately not done, because
  removed packages couldn't be reinstalled here (no APT lists). The command path is the
  same handover verified in Phase 3.
- Windows removers (winget by name, Chocolatey, Scoop) on a real Windows machine.

- Windows-only analysis sources on a real Windows machine: the UserAssist registry read,
  and Chocolatey and Scoop dependency folders. Decoding and matching are unit-tested.
- The support table against the projects' pages at release time. Dates are as of 2026-11
  and are shown with their sources.

- Real update runs of winget, Chocolatey, Scoop, APT, DNF, Snap or Flatpak. No Windows
  host is available, and this container has no APT lists, so nothing was updated for
  real. Parsing and planning are unit-tested with output fixtures, and the handover
  itself is verified end to end.
- winget's behaviour when a source agreement is pending during `winget upgrade`
  (expected: winget prompts in the terminal).
- Colours on Windows Terminal, legacy conhost and light terminal backgrounds.
