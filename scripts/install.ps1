# domjudge-tool installer (Windows)
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/ntub/domjudge-tool/main/scripts/install.ps1 | iex
#
# Usage (cmd.exe):
#   powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/ntub/domjudge-tool/main/scripts/install.ps1 | iex"
#
# Environment variables:
#   DOMJUDGE_TOOL_VERSION        Release tag to install (e.g. v1.8.0; default: latest)
#   DOMJUDGE_TOOL_INSTALL_DIR    Target directory for binary (default: %LOCALAPPDATA%\Programs\domjudge-tool\bin)
#   DOMJUDGE_TOOL_NO_MODIFY_PATH Set to 1 to skip modifying User PATH
#
# Integrity note:
#   Binaries are not OS-code-signed; integrity comes from the SHA-256 checksum in
#   checksums.txt and optional GitHub build provenance verification:
#     gh attestation verify <archive> --repo ntub/domjudge-tool
#   Downloads via PowerShell / Invoke-WebRequest are not quarantined by Windows SmartScreen.

$ErrorActionPreference = 'Stop'

# Enable TLS 1.2 on Windows PowerShell 5.1
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {
    # Ignore if TLS 1.2 is already the default or unsupported
}

# Architecture detection (support 32-bit PowerShell on 64-bit OS via PROCESSOR_ARCHITEW6432)
$rawArch = $env:PROCESSOR_ARCHITEW6432
if ([string]::IsNullOrEmpty($rawArch)) {
    $rawArch = $env:PROCESSOR_ARCHITECTURE
}
if ([string]::IsNullOrEmpty($rawArch)) {
    Write-Error "Unable to detect processor architecture."
    exit 1
}

switch ($rawArch.ToUpperInvariant()) {
    "AMD64" { $targetArch = "amd64" }
    "ARM64" { $targetArch = "arm64" }
    Default {
        Write-Error "Unsupported Windows architecture: $rawArch. Only AMD64 and ARM64 are supported."
        exit 1
    }
}

$archiveName = "domjudge-tool_windows_${targetArch}.zip"
$checksumsName = "checksums.txt"

if (-not [string]::IsNullOrEmpty($env:DOMJUDGE_TOOL_VERSION)) {
    $baseUrl = "https://github.com/ntub/domjudge-tool/releases/download/$($env:DOMJUDGE_TOOL_VERSION)"
} else {
    $baseUrl = "https://github.com/ntub/domjudge-tool/releases/latest/download"
}

$installDir = $env:DOMJUDGE_TOOL_INSTALL_DIR
if ([string]::IsNullOrEmpty($installDir)) {
    $installDir = Join-Path $env:LOCALAPPDATA "Programs\domjudge-tool\bin"
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("domjudge-tool-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    $archivePath = Join-Path $tempDir $archiveName
    $checksumsPath = Join-Path $tempDir $checksumsName

    Write-Host "Downloading $archiveName..."
    Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$archiveName" -OutFile $archivePath

    Write-Host "Downloading $checksumsName..."
    Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$checksumsName" -OutFile $checksumsPath

    # Verify SHA-256 against checksums.txt
    $checksumContent = Get-Content -LiteralPath $checksumsPath
    $matchedLine = $checksumContent | Where-Object { $_ -match "\s+\*?$([regex]::Escape($archiveName))\s*$" } | Select-Object -First 1
    if (-not $matchedLine) {
        Write-Error "$archiveName not found in $checksumsName"
        exit 1
    }

    $tokens = -split $matchedLine.Trim()
    $expectedHash = $tokens[0].ToLowerInvariant()
    if ($expectedHash.Length -ne 64) {
        Write-Error "Invalid SHA-256 hash length in $checksumsName for $archiveName"
        exit 1
    }

    $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
        Write-Error "SHA-256 checksum mismatch for ${archiveName}`n  expected: $expectedHash`n  actual:   $actualHash"
        exit 1
    }
    Write-Host "Verified SHA-256 checksum ($actualHash)."

    # Extract archive
    $extractDir = Join-Path $tempDir "extracted"
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir -Force

    $sourceExe = Join-Path $extractDir "domjudge-tool.exe"
    if (-not (Test-Path -LiteralPath $sourceExe)) {
        Write-Error "domjudge-tool.exe not found in archive $archiveName"
        exit 1
    }

    # Install binary atomically
    if (-not (Test-Path -LiteralPath $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    $tempExe = Join-Path $installDir (".domjudge-tool.tmp." + $PID + ".exe")
    Copy-Item -LiteralPath $sourceExe -Destination $tempExe -Force
    $targetExe = Join-Path $installDir "domjudge-tool.exe"
    Move-Item -LiteralPath $tempExe -Destination $targetExe -Force

    try {
        $installedVersion = & $targetExe version
        Write-Host "Installed $installedVersion to $targetExe"
    } catch {
        Write-Host "Installed domjudge-tool to $targetExe"
    }

    # PATH management
    $normalizedInstall = $installDir.TrimEnd('\', '/')
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $userPaths = if ($userPath) { $userPath -split ';' } else { @() }
    $inUserPath = $false
    foreach ($p in $userPaths) {
        if ($p.Trim().TrimEnd('\', '/') -ieq $normalizedInstall) {
            $inUserPath = $true
            break
        }
    }

    if ($env:DOMJUDGE_TOOL_NO_MODIFY_PATH -eq '1') {
        Write-Host "DOMJUDGE_TOOL_NO_MODIFY_PATH=1 set; skipping PATH modification."
        Write-Host "To use domjudge-tool in your current session, run:"
        Write-Host "  `$env:Path = `"$installDir;`$env:Path`""
    } elseif (-not $inUserPath) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $installDir } else { "$userPath;$installDir" }
        [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')
        Write-Host "Added $installDir to User PATH."
        Write-Host "Restart your terminal or open a new window for PATH changes to take effect."
        # Update current session PATH if not present
        $sessionPaths = $env:Path -split ';'
        $inSessionPath = $false
        foreach ($p in $sessionPaths) {
            if ($p.Trim().TrimEnd('\', '/') -ieq $normalizedInstall) {
                $inSessionPath = $true
                break
            }
        }
        if (-not $inSessionPath) {
            $env:Path = "$installDir;$env:Path"
        }
    } else {
        Write-Host "$installDir is already in User PATH."
    }
} finally {
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
