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
- **Updates** checks winget, Chocolatey and Scoop on Windows, and APT, DNF, Snap
  and Flatpak on Linux. Install everything with one key, or pick what to update.
- **Activity History** records every scan and change, so nothing happens silently.
- Simple and Advanced modes, dark and light themes, and help on every screen.

Installing updates is the only change ATLAS makes, and only after you confirm a
preview listing every change. ATLAS then hands the terminal to the package
manager, so you see its real output and answer its own prompts, such as your sudo
password or a Windows permission dialog. ATLAS never answers for you, and it
never removes software.

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
| `c`, `u`, `space`, `enter` | In Updates: check, update all, select, update selected |
| `x` | Stop a running scan or update check |
| `Ctrl+C` | While updates run: stop; nothing further starts |
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
