ATLAS shows what is installed on your computer and helps you keep it tidy and up to
date, in plain language. Nothing happens without a preview you confirm, and every
action is recorded in Activity History.

## What's new in 0.2.1

**Enter works on empty screens.** Smart Analysis looked broken before its first run:
it only answered to the `a` key, while the Dashboard teaches Enter. Now, when a screen
has nothing to show yet, Enter does what it suggests: Smart Analysis and Dependencies
analyze (or scan first), My Software scans, Updates checks, Cleanup looks for things to
tidy. Each still shows its preview first, and the letter keys work as before.

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
