# ATLAS — Test Status

Last run: 2026-11-11, end of Phase 1, Go 1.24.7 on linux/amd64.

## Unit tests (`go test ./...`) — all passing

| Package | Tests | Covers |
|---------|-------|--------|
| internal/config | 10 | defaults; missing file (no write); malformed file → defaults + plain-language error; partial file keeps defaults; explicit `false` honoured; invalid enum values warn per field; newer version warns; save round-trip; save backs up malformed original; toggles/labels |
| internal/logging | 2 | JSON lines, level filtering, file creation and append |
| internal/platform | 2 | `ATLAS_HOME` override; default dirs end in `atlas` / `atlas/logs` |
| internal/tui/nav | 4 | section order, titles and shortcuts; coming-soon metadata complete; next/prev wrap; jump by digit and invalid rejection |
| internal/tui/layout | 6 | breakpoint table; wide sidebar and content sizing; medium/narrow tab bar; header and hints drop on short terminals; too-small never negative; content ≥ 1 for every size |
| internal/tui/theme | 2 | theme selection by name with default; palettes complete |
| internal/tui/components | 4 | tab-bar tier selection; every shortcut visible at 40–100 cols; scroll clamping and indicators; empty/error state wording and Advanced-only details |
| internal/tui | 13 | starts on Home in Simple mode; tab/shift-tab/digit navigation; mode toggle persists to file and changes Home content; theme toggle persists; save failure reported in status bar; malformed config backed up on first save; help overlay opens/closes and swallows navigation; quick action opens preview with all five sections and writes nothing; Settings toggles persist; resize changes breakpoint and too-small message; content scrolls with indicators and resets on navigation; q/ctrl+c quit; view never exceeds terminal height across 6 sizes × 9 sections |

## Checks

| Check | Result |
|-------|--------|
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test -count=1 ./...` | pass (8 packages) |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas` | pass (atlas.exe, ~5.9 MB) |
| `GOOS=windows GOARCH=amd64 go vet ./...` | clean |
| `go build` + `atlas -version` on linux | pass |
| Real binary under a pseudo-terminal | starts, enters alt screen, renders Home at 100×30, writes the JSON log to `ATLAS_HOME/logs/atlas.log`, creates no config file on startup |

## Manual rendering review

Views were rendered (ANSI stripped) at 120×30, 80×24, 60×18 and 100×13 for
Home (Simple and Advanced), help, action preview (Simple and Advanced),
Settings and a coming-soon section. Every render fit its terminal exactly.

## Not verified in this environment

- **Keyboard input through a forked pseudo-terminal.** In this sandbox Bubble
  Tea's input reader receives no key events even for a minimal stock program,
  while plain programs (`cat`) do receive pty input. This is a sandbox
  limitation, not an ATLAS defect; key handling is covered by the model tests.
  Needs a quick manual run on Windows Terminal and a Linux shell.
- Visual check of colours on Windows Terminal, legacy conhost and a light
  terminal background.
