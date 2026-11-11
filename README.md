# ATLAS

**Application & Tool Lifecycle Analysis System.** A keyboard-first terminal app
that makes the software on your computer easy to understand and manage. Built
for Windows 11 first; Linux stays supported.

Phase 1 ships the application shell: navigation, Home, Settings, Simple and
Advanced modes, dark and light themes, a help overlay and honest empty states
for everything that is not built yet. It never installs, updates or removes
anything.

## Build and run

```bash
go build -o atlas ./cmd/atlas
./atlas
```

On Windows:

```powershell
go build -o atlas.exe ./cmd/atlas
.\atlas.exe
```

Flags: `-config <path>` to use a specific settings file, `-version` to print the
version. Setting `ATLAS_HOME` keeps settings and logs in one folder (portable use).

## Keys

| Key | Action |
|-----|--------|
| `?` | Help for the current screen |
| `tab` / `shift+tab`, `1`–`9` | Move between sections |
| arrows, `enter` | Move within a screen, choose an item |
| `m` | Simple / Advanced mode (remembered) |
| `t` | Dark / light theme (remembered) |
| `pgup` / `pgdn` | Scroll when a screen does not fit |
| `q` | Quit |

## Where files live

| | Windows | Linux |
|-|---------|-------|
| Settings | `%AppData%\atlas\config.yaml` | `~/.config/atlas/config.yaml` |
| Log | `%LocalAppData%\atlas\logs\atlas.log` | `~/.local/state/atlas/logs/atlas.log` |

## Development

```bash
gofmt -l . && go vet ./... && go test ./...
GOOS=windows GOARCH=amd64 go build ./...
```

See `PROJECT_STATE.md`, `IMPLEMENTATION_PLAN.md`, `DECISIONS.md` and
`TEST_STATUS.md` for status, plan, decisions and test coverage.
