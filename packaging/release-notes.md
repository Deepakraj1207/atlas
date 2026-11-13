ATLAS shows what is installed on your computer and helps you keep it tidy and up to
date, in plain language. Nothing happens without a preview you confirm, and every
action is recorded in Activity History.

## Install

**Windows** (once the winget package is accepted):

```powershell
winget install Deepakraj1207.ATLAS
```

Or download `atlas_<version>_windows_amd64.zip` (or `_arm64`), unzip it, and run
`atlas.exe` in Windows Terminal.

**Ubuntu / Debian:** `sudo apt install ./atlas-lifecycle_<version>_amd64.deb`

**Fedora / RHEL:** `sudo dnf install ./atlas-lifecycle-<version>-1.x86_64.rpm`

**Any Linux:** unpack `atlas_<version>_linux_amd64.tar.gz` and run `./atlas`.

The command is `atlas` everywhere. Check a download with `SHA256SUMS`.

## Notes

- The Windows build isn't code-signed yet, so SmartScreen may warn the first time you
  run a downloaded `atlas.exe`. Installing through winget avoids the warning.
- The Linux package is called `atlas-lifecycle` because Fedora already has a package
  named `atlas` (a maths library).
- `atlas -selftest` checks ATLAS against your computer without changing anything.
