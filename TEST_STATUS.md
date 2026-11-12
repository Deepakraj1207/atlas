# ATLAS — Test Status

Last run: 2026-11-12, end of Phase 5, Go 1.24.7 (`GOTOOLCHAIN=local`) on linux/amd64.

## Unit tests (`go test ./...`) — all passing, 149 tests in 15 packages

| Package | Tests | Covers |
|---------|-------|--------|
| internal/cleanup | 12 | gate blocks what other software needs, naming it; allows when dependents are removed in the same plan; blocks Essential, required, protected names, kernels, unknown-dependency runtimes and components; DNF protected.d; remover choice on Windows (winget by ID or exact name, Scoop, Chocolatey needs admin, ambiguous ownership blocked); no forbidden flags in any remover and every removal has a rollback; APT cache and autoremove (simulation only); DNF cache and unneeded; snap disabled revisions only; Scoop old versions and cache; find, run, record and drop removed software; sizes |
| internal/handover | 3 | multi-command steps, blocked, failed, missing program, restart codes, wording; Ctrl+C stops remaining steps; verbs and summaries follow the step action |
| internal/analysis | 13 | Phase 4 coverage, plus Essential/required marking in the dpkg graph |
| internal/database | 12 | earlier coverage, plus exact-match deletion of removed software |
| internal/updates | 10 | Phase 3 managers, planning, recording (job tests moved to handover) |
| internal/inventory, core, config, logging, platform | 39 | earlier phases |
| internal/tui/nav, layout, theme, components | 18 | navigation, layout, themes, components |
| internal/tui | 42 | earlier phases plus: removal from My Software opens a preview with dependency, rollback and never-purge text; Enter refuses, Esc runs nothing, `r` runs exactly the planned command, the item leaves My Software and the run is recorded; a blocked removal names what needs it and can't be forced with `r` or Enter; Cleanup shows analysis suggestions and, after a previewed read-only search, things to tidy with sizes; tidying runs only the listed command, reports space freed and leaves the list; Enter still confirms read-only previews and `r` doesn't |

### Race detector

`go test -race` over every package that uses goroutines (tui, inventory, updates,
analysis, cleanup, handover) reports no data races and no failures. An earlier race-detector run
found a real timing bug (D46), which is now fixed and has a regression test.

## Checks

| Check | Result |
|-------|--------|
| `gofmt -l .` | clean |
| `go vet ./...` (linux) | clean |
| `GOOS=windows GOARCH=amd64 go vet ./...` | clean |
| `go test -count=1 ./...` | pass |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~14.6 MB) |
| `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~13.9 MB) |
| Linux build | pass |

## End-to-end runs in a pseudo-terminal

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
