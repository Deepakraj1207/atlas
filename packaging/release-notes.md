ATLAS shows what is installed on your computer and helps you keep it tidy and up to
date, in plain language. Nothing happens without a preview you confirm, and every
action is recorded in Activity History.

## What's new in 0.2.2

**AI Tools: your MCP servers, plugins and skills in one place.** Press **0** for a new
section that lists everything set up in your AI apps, from each app's own settings:

| App | What ATLAS lists |
|---|---|
| Claude Code | MCP servers (yours and per project), plugins, skills |
| Claude Desktop | MCP servers, including the Microsoft Store version |
| Cursor, VS Code, Windsurf | MCP servers |
| Gemini CLI | MCP servers and extensions |
| Codex | MCP servers and skills |

You can manage them too. Each change is previewed first and recorded in Activity
History:
- **Remove an MCP server** you don't use. Only that entry changes, and a backup of the
  settings file is kept next to it.
- **Turn a Claude Code plugin on or off.**
- **Remove a skill.** Its folder is moved into ATLAS's own folder, not deleted, so you
  can put it back.

Where ATLAS can't make a change safely, for example a settings file with comments, Codex
or Gemini extensions, it tells you how to do it instead. An MCP server's keys and
environment are never read or shown.

**A calmer look.** The Dashboard opens with a large logo and three framed panels: what
you can do, your computer, and the one next step that matters most. Every screen now has
a one-line title instead of an intro paragraph, messages sit in framed panels, and lists
are separated by thin lines. Keys at the bottom read `[enter] select`, and the help screen
uses a few words per key.

## What's new in 0.2.1

**Enter works on empty screens.** Smart Analysis looked broken before its first run:
it only answered to the `a` key, while the Dashboard teaches Enter. Now, when a screen
has nothing to show yet, Enter does what it suggests: Smart Analysis and Dependencies
analyze (or scan first), My Software scans, Updates checks, Cleanup looks for things to
tidy. Each still shows its preview first, and the letter keys work as before.

**Nothing goes unrecorded, even if the window is closed.** If ATLAS (or the terminal
window) is closed while a package manager is updating, removing or installing, Activity
History now shows that run as **Unfinished**, with what was started, and the Dashboard
tells you to scan again. Before, a run cut short this way left no trace.

**Updates you can read.** Long version numbers are shown in full instead of being cut
off where "Installed" and "New" looked identical. On narrow terminals the selected
update is spelled out above the list.

**Easier to find your way.**
- The Dashboard's next steps start with what needs doing (updates waiting, findings),
  not general tips.
- In Smart Analysis, Enter on a finding opens the place where you act on it (Updates,
  Cleanup or Dependencies).
- "Find unused applications" shows the unused apps, or says plainly that none were
  found, or that it can't check on this system and why.
- On terminals narrower than 105 columns the tabs now name every section instead of
  showing only numbers.
- Long messages continue on the next line instead of ending in "…".

**Clearer wording.** When ATLAS refuses to remove something, the preview says so and
nothing else. Turning on AI explanations without a key says at once what is missing.
Plus smaller fixes: "1 needs attention", no "0 worth knowing", and "Welcome!" only on a
first run.

## What's new in 0.2.0

**A new look.** ATLAS opens on a Dashboard: a "What would you like to do?" list of
the common jobs, your computer at a glance (Windows or Linux version, memory, disk), and
the real numbers of apps, updates, runtimes and things to clean up. The status bar shows
what ATLAS is doing and the figures that matter. Press Esc to move through the sections
with the arrow keys, and `/` to search your software from anywhere. Every key from 0.1.0
still works.

**Safer clean-up.** ATLAS now refuses to remove something when it can't read the
dependency records completely, for example when a package manager fails or takes too
long. Before, it could treat "couldn't check" as "nothing needs it".

**Results are checked.** After removing or updating software, ATLAS looks at the
computer again. Only changes it can confirm leave My Software or Updates; anything it
can't confirm stays listed, and Activity History says why.

**openSUSE support.** RPM-based systems now use their own tool: DNF on Fedora, RHEL and
similar, Zypper on openSUSE and SLES. Systems ATLAS can't handle safely are refused with
the reason, instead of guessing.

**Fewer wrong merges.** When ATLAS can't be sure two entries are the same install (a
portable copy beside an installed one, two editions, the same app from two Linux package
managers), it now shows both instead of hiding one.

**AI explanations** (optional, off by default):
- You can now use a model running on your own computer through Ollama: free, and
  nothing goes over the internet.
- With Claude, the preview says plainly that questions are billed to your Anthropic API
  account, and shows the most one question can cost.
- Your user name, home folder and computer name are removed before anything is sent.
- `ai.max_output_tokens` limits how long an answer can be.

## Install

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/Deepakraj1207/atlas/main/install.ps1 | iex
```

**Linux:**

```sh
curl -fsSL https://raw.githubusercontent.com/Deepakraj1207/atlas/main/install.sh | sh
```

These download this release's file for your computer, check it against `SHA256SUMS`,
and install `atlas` for your user only. No administrator rights or sudo.

**Or by hand:**

**Windows:** download `atlas_<version>_windows_amd64.zip` (or `_arm64`), unzip it, and
run `atlas.exe` in Windows Terminal.

**Ubuntu / Debian:** `sudo apt install ./atlas-lifecycle_<version>_amd64.deb`

**Fedora / RHEL:** `sudo dnf install ./atlas-lifecycle-<version>-1.x86_64.rpm`

**openSUSE:** `sudo zypper install ./atlas-lifecycle-<version>-1.x86_64.rpm`

**Any Linux:** unpack `atlas_<version>_linux_amd64.tar.gz` and run `./atlas`.

The command is `atlas` everywhere. Check a download with `SHA256SUMS`.

## Update

- **Installed with the one-line command:** run the same command again.
- **.deb / .rpm:** download the new file and install it the same way; it replaces the
  old version and keeps your settings and saved data.
- **zip / tarball:** replace the `atlas` program with the new one.

ATLAS never updates itself or checks for new versions in the background.

## Notes

- The Windows build isn't code-signed yet, so SmartScreen may warn the first time you
  run an `atlas.exe` downloaded with a web browser.
- The Linux package is called `atlas-lifecycle` because Fedora already has a package
  named `atlas` (a maths library).
- `atlas -selftest` checks ATLAS against your computer without changing anything.
