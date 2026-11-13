# Windows 11 test plan

What CI can't check: an interactive Windows 11 desktop, with real permission
prompts, real terminals and your own installed software. Allow about 30 minutes.

Everything below is either read-only or changes only things you choose. Steps that
install or remove something use **jq** (a tiny command-line tool) or **7-Zip**, so
nothing important is at risk.

## 0. Get the build

Either download `atlas-windows-amd64` from the latest **windows** workflow run
(Actions tab on GitHub), or build it yourself with Go 1.27.4:

```powershell
$env:GOTOOLCHAIN = "local"
go build -o atlas.exe ./cmd/atlas
```

Put `atlas.exe` in a folder of your choice, for example `C:\Tools\atlas`.

## 1. Self-test (read-only, 1–2 minutes)

In **Windows Terminal** (PowerShell), in the folder with `atlas.exe`:

```powershell
.\atlas.exe -selftest -report selftest-win11.md
```

It changes nothing and doesn't touch your ATLAS data. Send `selftest-win11.md` back.
It lists program names and versions found on your PC, so read it first if that matters to you.

## 1b. Automatic interface checks (read-only, about 2 minutes)

`tools/conharness` runs ATLAS in a real Windows console (ConPTY, as Windows Terminal
does), presses keys from a script and checks the screens. The two scripts cover most
of section 2 without changing anything; ATLAS uses a scratch data folder.

```powershell
Push-Location tools\conharness; go build -o ..\..\conharness.exe .; Pop-Location
$env:ATLAS_HOME = "$env:TEMP\atlas-check"
.\conharness.exe -strict -script tools\conharness\scripts\ui-basics.txt -out screens-ui.txt -- .\atlas.exe
.\conharness.exe -strict -script tools\conharness\scripts\readonly-flow.txt -out screens-flow.txt -- .\atlas.exe
```

Exit code 0 means every check passed; the screens are in the `-out` files. CI runs the
same scripts on every push.

## 2. Interactive checks

Run `.\atlas.exe` in Windows Terminal. For each row, note **OK** or what you saw.

| # | Do this | Expect |
|---|---------|--------|
| 2.1 | Start ATLAS | Home appears at once; no strange characters; the sidebar shows 9 sections |
| 2.2 | Press `t`, then `t` again | Light theme, then dark again; colours stay readable |
| 2.3 | Resize the window narrow, then very small | Tabs replace the sidebar; very small shows "ATLAS needs a terminal of at least…" |
| 2.4 | Press `?`, then Esc | Help opens and closes |
| 2.5 | Home → Enter on **Scan My System** → Enter | Preview, then live progress; "Scan complete"; the count of apps looks right |
| 2.6 | `2` (My Software), `/`, type `7-zip` (or any app you know), Enter | It's listed with the right version; Enter shows details, including Publisher |
| 2.7 | `m` (Advanced mode) | Package IDs appear (e.g. `7zip.7zip` if winget knows it) |
| 2.8 | Home → **Check for Updates** → Enter | winget is checked. If you've never accepted winget's source agreements, it should say "Windows Package Manager is waiting for you to accept its source agreements" with how to do it, **not** hang. Run `winget list` once, accept, and check again |
| 2.9 | `3` (Updates) | Updates listed with installed → new versions; held apps marked |
| 2.10 | Home → **Analyze My Software** → Enter | Smart Analysis lists findings. Under "What ATLAS checked", "Apps not opened in over a year" is **checked** (Windows records usage) |
| 2.11 | `5` (Dependencies) | Visual C++ and .NET runtimes say "not recorded … treat it as needed" |
| 2.12 | `6` (Cleanup), `c`, Enter | Search runs; results or "Nothing to tidy" |
| 2.13 | `7` (Discover), find **jq**, Enter | Preview shows `winget install --id jqlang.jq --exact --source winget` |
| 2.14 | Enter in that preview | The terminal is handed to winget; you see winget's own output; "Press Enter to return to ATLAS"; back in ATLAS, "Installed 1" |
| 2.15 | Home → Scan again → `2`, find jq, `r` | Removal preview; **Enter does nothing** except say "press r"; Esc cancels |
| 2.16 | `r` again, then `r` in the preview | winget removes jq in the terminal; back in ATLAS, jq is gone from My Software |
| 2.17 | `7`, find **7-Zip** (if not installed), Enter, Enter | A Windows permission (UAC) prompt may appear **from the installer**; ATLAS never asks for it itself. Approve or decline: both outcomes are recorded correctly |
| 2.18 | During an install or update, press Ctrl+C | The installer stops; ATLAS stays open, says "Stopped", and starts nothing else |
| 2.19 | `8` (Activity History), `m` | Every action above is listed; Advanced shows commands and exit codes |
| 2.20 | `q` | ATLAS exits; the window is left clean |

## 3. Older console (optional)

Press Win+R, run `conhost.exe`, `cd` to the folder and run `.\atlas.exe`. Repeat
2.1–2.4. Report any boxes, question marks or broken lines.

## 4. Administrator (optional)

Right-click Windows Terminal → **Run as administrator**, run ATLAS, and open Updates
or Discover. Chocolatey items (if you use Chocolatey) are no longer blocked. The Home
diagnostics in Advanced mode show "Administrator yes".

## 5. Let Claude run it instead

Start **Desktop Commander** on the PC (it's registered as `DESKTOP-41IVS1K`) and say
so in the session. Claude can then run section 1 and the read-only parts of section 2
directly. Anything that installs, removes or needs a UAC answer still needs you.
