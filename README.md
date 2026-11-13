# ATLAS

**Application & Tool Lifecycle Analysis System.** A keyboard-first terminal app
that makes the software on your computer easy to understand and manage. Built
for Windows 11 first; Linux stays supported.

## Install

```powershell
winget install Deepakraj1207.ATLAS     # Windows, once the package is accepted
```

Or download a release from GitHub: a zip for Windows (x64 or ARM64), a `.deb`
(Ubuntu/Debian) or `.rpm` (Fedora/RHEL) named `atlas-lifecycle`, or a tarball for any
Linux. The command is `atlas` everywhere. `atlas -selftest` checks ATLAS against your
computer without changing anything.

## What it does

- **Scan My System** finds installed software without changing anything. On
  Windows it reads installed programs, winget, Chocolatey and Scoop. On Linux it
  reads dpkg/apt, RPM, Snap and Flatpak.
- **My Software** lists what was found, with search, details, and a filter that
  hides libraries and runtimes unless you ask for them.
- **Updates** checks winget, Chocolatey and Scoop on Windows, and APT, DNF, Snap
  and Flatpak on Linux. Install everything with one key, or pick what to update.
- **Smart Analysis** explains what's worth knowing in plain language: runtimes that no
  longer get security fixes, apps installed twice, old versions, and apps you haven't
  opened in a year (Windows). It also lists what it checked and what it couldn't.
- **Dependencies** shows which runtimes (Python, Node.js, Java, .NET, Visual C++) other
  software needs, from your package managers' own records.
- **Cleanup** removes software you don't need, from My Software or from Smart Analysis
  suggestions, and clears package caches and old versions. Before every removal ATLAS
  checks again what depends on it, and it refuses to remove anything other software
  or your system needs.
- **Discover Apps** suggests free, well-known tools based on what you already have,
  explains why, and installs them through your package manager after a preview. No
  internet lookups are made to build suggestions.
- **AI explanations** (optional, off by default): press `e` on a finding or an app to
  ask Claude, Anthropic's AI, what it is and whether it matters. You see exactly what
  will be sent first; it needs your own Anthropic API key in `ANTHROPIC_API_KEY`.
- **Activity History** records every scan and change, so nothing happens silently.
- Simple and Advanced modes, dark and light themes, and help on every screen.

ATLAS only changes your system by installing updates or tools you picked, removing
software, or clearing caches, and only after you confirm a preview listing every change. Removals need the
`r` key; Enter alone never deletes anything. ATLAS then hands the terminal to the package
manager, so you see its real output and answer its own prompts, such as your sudo
password or a Windows permission dialog. ATLAS never answers for you, and never
uses flags that skip questions or delete your settings.

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
version, and `-selftest [-report file.md]` to check ATLAS against this computer and
write a report. The self-test is read-only and doesn't touch your ATLAS data.
Setting `ATLAS_HOME` keeps settings and logs in one folder (portable use).

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
| `a` | In Smart Analysis and Dependencies: analyze (again) |
| `r` | In My Software: remove the selected item (with a preview) |
| `c`, `space`, `enter` | In Cleanup: look for things to tidy, select, review |
| `r` | In a removal or clean-up preview: go ahead |
| `←/→`, `space`, `enter` | In Discover Apps: category, select, install (with a preview) |
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

ATLAS needs Go 1.27.4 (the version in `go.mod`; Go 1.24 is no longer supported and
the Anthropic SDK needs 1.25 or later). With it installed, use the local toolchain so
dependency updates can't raise the Go version:

```bash
export GOTOOLCHAIN=local
gofmt -l . && go vet ./... && go test ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/atlas
```

See `PROJECT_STATE.md`, `IMPLEMENTATION_PLAN.md`, `DECISIONS.md` and
`TEST_STATUS.md` for status, plan, decisions and test coverage.
