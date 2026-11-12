# ATLAS — Test Status

Last run: 2026-11-12, end of Phase 4, Go 1.24.7 (`GOTOOLCHAIN=local`) on linux/amd64.

## Unit tests (`go test ./...`) — all passing, 131 tests in 13 packages

| Package | Tests | Covers |
|---------|-------|--------|
| internal/analysis | 13 | support dates incl. month-precision ends and the 6-month window; runtime detection for 18 real name formats and 8 look-alikes that must not match; dpkg graph with Provides and alternatives, direct and indirect dependents; RPM requires-to-provides; Chocolatey nested `<dependency>`; Scoop string or list `depends`; UserAssist ROT13, value decoding, zero timestamps, known-folder paths, folder-boundary matching; support rules (attention vs distro info, ending soon, old VC++ grouped); duplicates vs versions incl. transitional snap stub, distro names and components; unused needs positive evidence; dependents and "may no longer be needed"; freshness; service refuses without a scan, saves, records, partial when a source fails; cancel before start is "cancelled", not an error |
| internal/config | 10 | defaults, missing/malformed/partial files, enum warnings, save round-trip, backup |
| internal/core | 3 | command rendering, exit codes, status helpers |
| internal/database | 11 | migrations incl. upgrading a v1 database; scans; activity; update checks; analysis round-trip, latest only, cleared with the software list |
| internal/inventory | 21 | Phase 2 parsers and rules; plain-language source errors |
| internal/updates | 12 | Phase 3 managers, planning, terminal job, recording |
| internal/logging, platform | 5 | file logging; directories |
| internal/tui/nav, layout, theme, components | 18 | navigation, responsive layout, themes, shared components |
| internal/tui | 38 | earlier phases plus: analysis needs a scan (screens and preview); full flow from Home to Smart Analysis, What ATLAS checked, Dependencies and Activity History; real dependency graph with direct and indirect dependents and the Advanced data-source line; stale analysis note and clearing with the list; analysis can be stopped and excludes scans; group heading stays visible on a 60×18 terminal |

### Race detector

`go test -race -count=5` over the tui, inventory, updates and analysis packages (all
that use goroutines) reports no data races and no failures. An earlier race-detector run
found a real timing bug (D46), which is now fixed and has a regression test.

## Checks

| Check | Result |
|-------|--------|
| `gofmt -l .` | clean |
| `go vet ./...` (linux) | clean |
| `GOOS=windows GOARCH=amd64 go vet ./...` | clean |
| `go test -count=1 ./...` | pass |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~14.2 MB) |
| `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~13.6 MB) |
| Linux build | pass |

## End-to-end runs in a pseudo-terminal

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
