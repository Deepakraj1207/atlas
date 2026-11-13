# ATLAS — Test Status

Last run: 2026-11-12, end of Phase 6, Go 1.24.7 (`GOTOOLCHAIN=local`) on linux/amd64.

## Unit tests (`go test ./...`) — all passing, 163 tests in 16 packages

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

- **An interactive Windows 11 desktop.** The plan is in `docs/WINDOWS_TEST_PLAN.md`:
  UAC prompts from installers, Windows Terminal and conhost rendering, Ctrl+C during a
  real install, and a real install and removal of `jq`. The user's PC
  (DESKTOP-41IVS1K) was offline in Desktop Commander during this session.

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
