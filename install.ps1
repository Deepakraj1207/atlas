# Install ATLAS on Windows. In PowerShell:
#
#   irm https://raw.githubusercontent.com/Deepakraj1207/atlas/main/install.ps1 | iex
#
# It downloads the latest release for this computer from GitHub, checks it
# against the release's SHA256SUMS, puts atlas.exe in
# %LOCALAPPDATA%\Programs\ATLAS and adds that folder to your user PATH. No
# administrator rights are needed and nothing else is changed. Run it again
# to update.
#
# Options (set before running, e.g. $env:ATLAS_VERSION = '0.2.0'):
#   ATLAS_VERSION       install that version instead of the latest
#   ATLAS_INSTALL_DIR   install into that folder instead
#   ATLAS_UNINSTALL=1   remove ATLAS and its PATH entry
#
# Everything runs inside one script block: nothing happens if the download is
# cut short, no variables are left in your session, and an error never closes
# your PowerShell window.

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is much faster without it
    $repo = 'Deepakraj1207/atlas'

    function Say([string]$text) { Write-Host $text }
    function Fail([string]$text) { Write-Host "atlas install: $text" -ForegroundColor Red; throw 'atlas-install-stop' }

    # The user PATH as stored (unexpanded, so entries like %USERPROFILE%\bin survive).
    function Get-UserPath {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
        if ($null -eq $key) { return '' }
        try { return [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames) }
        finally { $key.Close() }
    }
    function Set-UserPath([string]$value) {
        $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey('Environment')
        try { $key.SetValue('Path', $value, [Microsoft.Win32.RegistryValueKind]::ExpandString) }
        finally { $key.Close() }
        # Tell Windows the environment changed, so new terminals see it.
        try {
            if (-not ('AtlasInstall.Env' -as [type])) {
                Add-Type -Namespace AtlasInstall -Name Env -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
            }
            $result = [UIntPtr]::Zero
            [void][AtlasInstall.Env]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result)
        } catch { } # Without the broadcast, PATH still applies after signing in again.
    }
    function Split-PathList([string]$list) { @($list -split ';' | Where-Object { $_ -ne '' }) }
    function Same-Dir([string]$a, [string]$b) {
        [Environment]::ExpandEnvironmentVariables($a).TrimEnd('\') -ieq [Environment]::ExpandEnvironmentVariables($b).TrimEnd('\')
    }

    try {
        $dir = if ($env:ATLAS_INSTALL_DIR) { $env:ATLAS_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\ATLAS' }
        $exe = Join-Path $dir 'atlas.exe'

        if ($env:ATLAS_UNINSTALL -eq '1') {
            if (Test-Path $exe) {
                Remove-Item $exe -Force
                Remove-Item (Join-Path $dir 'atlas.exe.old') -Force -ErrorAction SilentlyContinue
                if (-not (Get-ChildItem $dir -Force)) { Remove-Item $dir -Force }
                Say "Removed $exe."
            } else {
                Say "Nothing to remove: $exe isn't there."
            }
            # Remove only our entry; every other entry stays exactly as it was.
            $raw = Get-UserPath
            $parts = $raw -split ';'
            $kept = @($parts | Where-Object { $_ -eq '' -or -not (Same-Dir $_ $dir) })
            if ($kept.Count -ne $parts.Count) {
                Set-UserPath ($kept -join ';')
                Say "Removed $dir from your PATH."
            }
            Say "Your settings, saved data and logs are kept (%APPDATA%\atlas and %LOCALAPPDATA%\atlas); delete those folders too if you want them gone."
            return
        }

        $cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
        switch ($cpu) {
            'AMD64' { $arch = 'amd64' }
            'ARM64' { $arch = 'arm64' }
            default { Fail "there is no ATLAS build for this processor ($cpu); builds exist for x64 and ARM64." }
        }

        # Windows PowerShell 5.1 may not offer TLS 1.2 by default; GitHub requires it.
        try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

        $version = "$env:ATLAS_VERSION".TrimStart('v')
        if (-not $version) {
            try {
                $release = Invoke-RestMethod -UseBasicParsing -Uri "https://api.github.com/repos/$repo/releases/latest"
                $version = "$($release.tag_name)".TrimStart('v')
            } catch { }
            if (-not $version) { Fail "couldn't find the latest release on GitHub. Check your internet connection, or set `$env:ATLAS_VERSION." }
        }

        $name = "atlas_${version}_windows_${arch}"
        $base = if ($env:ATLAS_DOWNLOAD_BASE) { $env:ATLAS_DOWNLOAD_BASE } else { "https://github.com/$repo/releases/download/v$version" }
        $tmp = Join-Path ([IO.Path]::GetTempPath()) ("atlas-install-" + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $tmp | Out-Null
        try {
            Say "Downloading ATLAS $version for Windows ($arch)..."
            $zip = Join-Path $tmp "$name.zip"
            $sums = Join-Path $tmp 'SHA256SUMS'
            try { Invoke-WebRequest -UseBasicParsing -Uri "$base/$name.zip" -OutFile $zip } catch { Fail "couldn't download $base/$name.zip ($($_.Exception.Message))" }
            try { Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $sums } catch { Fail "couldn't download the checksums ($base/SHA256SUMS)" }

            $want = $null
            foreach ($line in Get-Content $sums) {
                $parts = $line -split '\s+', 2
                if ($parts.Count -eq 2 -and $parts[1].TrimStart('*') -eq "$name.zip") { $want = $parts[0].ToLowerInvariant() }
            }
            if (-not $want) { Fail "SHA256SUMS has no entry for $name.zip" }
            $got = (Get-FileHash -Algorithm SHA256 -Path $zip).Hash.ToLowerInvariant()
            if ($got -ne $want) { Fail "the download doesn't match its checksum, so it wasn't installed (expected $want, got $got)" }
            Say 'Checksum OK.'

            $unpacked = Join-Path $tmp 'unpacked'
            Expand-Archive -Path $zip -DestinationPath $unpacked -Force
            $new = Join-Path $unpacked 'atlas.exe'
            if (-not (Test-Path $new)) { Fail "the download doesn't contain atlas.exe" }

            New-Item -ItemType Directory -Path $dir -Force | Out-Null
            # Windows can't overwrite a running program but can rename it, so an
            # open ATLAS keeps working and the new version is used next time.
            $old = Join-Path $dir 'atlas.exe.old'
            Remove-Item $old -Force -ErrorAction SilentlyContinue
            if (Test-Path $exe) {
                try { Move-Item $exe $old -Force } catch { Fail "couldn't replace $exe. Close ATLAS and try again." }
            }
            Copy-Item $new $exe -Force
            Remove-Item $old -Force -ErrorAction SilentlyContinue
        } finally {
            Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
        }

        $installed = $exe
        try { $installed = (& $exe -version) -join ' ' } catch { }
        Say "Installed: $installed"
        Say "Location:  $exe"

        # Append our entry and leave the rest of the user PATH exactly as it is.
        $raw = Get-UserPath
        if (-not (Split-PathList $raw | Where-Object { Same-Dir $_ $dir })) {
            $updated = if ($raw -eq '') { $dir } elseif ($raw.EndsWith(';')) { "$raw$dir;" } else { "$raw;$dir" }
            Set-UserPath $updated
            Say "Added $dir to your PATH."
        }
        if (-not (Split-PathList $env:Path | Where-Object { Same-Dir $_ $dir })) {
            $env:Path = ($env:Path.TrimEnd(';') + ';' + $dir)
        }
        $found = (Get-Command atlas -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1).Source
        if ($found -and -not (Same-Dir (Split-Path $found) $dir)) {
            Say ''
            Say "Note: another atlas comes first on your PATH ($found)."
            Say "Remove that one, or run $exe directly."
        }
        Say ''
        Say 'Start ATLAS by typing:  atlas'
        Say 'To update later, run the same install command again.'
    } catch {
        if ("$_" -ne 'atlas-install-stop') { Write-Host "atlas install: $_" -ForegroundColor Red }
    }
}
