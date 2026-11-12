# ATLAS — Test Status

Last run: 2026-11-12, end of Phase 2, Go 1.24.7 (`GOTOOLCHAIN=local`) on linux/amd64.

## Unit tests (`go test ./...`) — all passing, 85 tests in 10 packages

| Package | Tests | Covers |
|---------|-------|--------|
| internal/config | 10 | defaults, missing/malformed/partial files, enum warnings, save round-trip, backup |
| internal/logging | 2 | JSON lines, levels, append |
| internal/platform | 3 | `ATLAS_HOME` override for config, log and data dirs; default dir names |
| internal/database | 8 | migration and idempotent reopen; refuses newer schema; empty db; save round-trip; latest scan replaces list; clear keeps activity; activity order and limit; persistence across reopen |
| internal/inventory | 20 | winget table (spinner noise, truncated names and IDs, ARP/MSIX IDs, wide characters, localized header, missing Available column, garbage, non-zero exit with table); registry rules (system components, child updates, KB updates, date and path cleanup, runtime classification); Scoop and Chocolatey folders; dpkg status with auto-installed and removed packages; RPM; Snap; Flatpak; missing program is "unavailable"; scanner statuses incl. timeout and panic; overall status; cancellation; merge rules; service saves complete scans, keeps the old list on failure, records clear; activity texts |
| internal/tui/nav | 4 | order, titles, shortcuts, availability, wrap, jumps |
| internal/tui/layout | 6 | breakpoints, sidebar, tab bar, short terminals, bounds |
| internal/tui/theme | 2 | selection, palette completeness |
| internal/tui/components | 6 | tab-bar tiers, shortcuts always visible, scroll window, empty/error states, human time, truncate/pad with wide characters |
| internal/tui | 24 | Phase 1 shell behaviour plus: scan preview → confirm → saved → shown on Home, My Software and Activity History; search captures `q`, digits and `m`; search, filter and details; Advanced reveals package IDs and technical errors; scan cancellation saves nothing and is recorded; partial scan explained; clear needs confirmation and is recorded; previews-off skips only read-only actions; database unavailable explained in every screen; empty My Software invites a scan; data survives restart; long lists scroll within the terminal; long previews scroll instead of clipping |

## Checks

| Check | Result |
|-------|--------|
| `gofmt -l .` | clean |
| `go vet ./...` (linux) | clean |
| `GOOS=windows GOARCH=amd64 go vet ./...` | clean |
| `go test -count=1 ./...` | pass |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~13.5 MB) |
| `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (~12.8 MB) |
| Linux build, `atlas -version` | pass |

## End-to-end run of the real binary (pseudo-terminal)

Started `atlas` with `ATLAS_HOME` in a scratch folder at 120×30 and drove it by
keyboard: Home rendered; `m` switched to Advanced and saved `mode: advanced`;
Enter opened the scan preview; Enter ran a real read-only scan of this Ubuntu
container (dpkg: 837 packages, 190 apps and 647 components, in 7 ms; RPM, Snap
and Flatpak reported "not on this computer"); `2` showed the list with package
IDs; `q` exited with code 0. The database, config and JSON log were all written.

**Resolves the Phase 1 limitation.** Keys had seemed not to arrive because
Bubble Tea's package init asks the terminal for its background colour and reads
stdin for up to 5 seconds waiting for the reply. The test harness now answers
that query as a real terminal does, and input works.

## Manual rendering review

Home, My Software (Simple, Advanced, search, details, all), Activity History
(Simple, Advanced), Settings and the clear preview were rendered from a real scan
at 120×30, 80×24 and 60×18. Every view fit its terminal exactly.

## Not verified in this environment

- The Windows registry, winget, Chocolatey and Scoop scanners on a real Windows
  machine. Their parsing is unit-tested and the code vets and builds for
  windows/amd64 and windows/arm64, but no Windows host is available here.
- winget output under non-English Windows locales beyond the French header fixture.
- Colours on Windows Terminal, legacy conhost and light terminal backgrounds.
