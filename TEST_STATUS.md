# ATLAS — Test Status

Last run: 2026-11-12, end of Phase 3, Go 1.24.7 (`GOTOOLCHAIN=local`) on linux/amd64.

## Unit tests (`go test ./...`) — all passing, 111 tests in 12 packages

| Package | Tests | Covers |
|---------|-------|--------|
| internal/config | 10 | defaults, missing/malformed/partial files, enum warnings, save round-trip, backup |
| internal/core | 3 | command rendering and quoting; exit-code interpretation incl. restart codes; status and count helpers |
| internal/logging | 2 | JSON lines, levels, append |
| internal/platform | 3 | `ATLAS_HOME` override for config, log and data dirs; default dir names |
| internal/database | 10 | migrations incl. upgrading a Phase 2 (v1) database in place; scans; activity; update check round-trip, held/component/scope fields, removal of applied updates, pruning old checks |
| internal/inventory | 21 | all Phase 2 parsers and rules; plain-language source errors |
| internal/updates | 12 | winget upgrade tables (truncated IDs blocked, explicit-target table held, no-updates messages, no agreement flags); Chocolatey exit code 2, pins, admin blocking, restart codes; Scoop bucket comparison and holds, global blocking; APT parsing, holds, missing package lists, sudo vs root, no `-y`; DNF exit 100; Snap; Flatpak; service enrichment from saved list; planning skips held and keeps manager order; terminal job (multi-command steps, blocked, failed, missing program, restart, output wording); Ctrl+C stops remaining steps; recording removes only applied updates; activity outcomes |
| internal/tui/nav | 4 | order, titles, shortcuts, availability, wrap, jumps |
| internal/tui/layout | 6 | breakpoints, sidebar, tab bar, short terminals, bounds |
| internal/tui/theme | 2 | selection, palette completeness |
| internal/tui/components | 6 | tab-bar tiers, scroll window, states, human time, truncate/pad |
| internal/tui | 32 | Phase 1 and 2 behaviour plus: check preview → check → Home and Updates show counts (held excluded); update-all preview lists every change, sudo and rollback, runs nothing before Enter, Esc cancels; confirmed run executes exactly the planned command and records it; Home suggests a rescan; selecting one update runs only it; held updates can't be selected; failure reported with exit code in Advanced history; blocked steps explained and never run; check stoppable and exclusive with scans; never-checked and up-to-date states; components hidden but included in update all; failed check explained on Updates screen |

## Checks

| Check | Result |
|-------|--------|
| `gofmt -l .` | clean |
| `go vet ./...` (linux) | clean |
| `GOOS=windows GOARCH=amd64 go vet ./...` | clean |
| `go test -count=1 ./...` | pass |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~13.8 MB) |
| `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~13.2 MB) |
| Linux build | pass |

## End-to-end runs in a pseudo-terminal

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

- Real update runs of winget, Chocolatey, Scoop, APT, DNF, Snap or Flatpak. No Windows
  host is available, and this container has no APT lists, so nothing was updated for
  real. Parsing and planning are unit-tested with output fixtures, and the handover
  itself is verified end to end.
- winget's behaviour when a source agreement is pending during `winget upgrade`
  (expected: winget prompts in the terminal).
- Colours on Windows Terminal, legacy conhost and light terminal backgrounds.
