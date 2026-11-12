# ATLAS

**Application & Tool Lifecycle Analysis System.** A keyboard-first terminal app
that makes the software on your computer easy to understand and manage. Built
for Windows 11 first; Linux stays supported.

What works today:

- **Scan My System** finds installed software without changing anything. On
  Windows it reads installed programs, winget, Chocolatey and Scoop. On Linux it
  reads dpkg/apt, RPM, Snap and Flatpak.
- **My Software** lists what was found, with search, details, and a filter that
  hides libraries and runtimes unless you ask for them.
- **Activity History** records every scan and change, so nothing happens silently.
- Simple and Advanced modes, dark and light themes, and help on every screen.

ATLAS never installs, updates or removes software yet. Every action shows a
preview of what it will do first.

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
| `/`, `f`, `s` | In My Software: search, show system components, scan |
| `x` | Stop a running scan |
| `q` | Quit |

## Where files live

| | Windows | Linux |
|-|---------|-------|
| Settings | `%AppData%\atlas\config.yaml` | `~/.config/atlas/config.yaml` |
| Database | `%LocalAppData%\atlas\atlas.db` | `~/.local/share/atlas/atlas.db` |
| Log | `%LocalAppData%\atlas\logs\atlas.log` | `~/.local/state/atlas/logs/atlas.log` |

## Development

Use the local Go toolchain so dependency updates can't raise the Go version:

```bash
export GOTOOLCHAIN=local
gofmt -l . && go vet ./... && go test ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas
```

See `PROJECT_STATE.md`, `IMPLEMENTATION_PLAN.md`, `DECISIONS.md` and
`TEST_STATUS.md` for status, plan, decisions and test coverage.
