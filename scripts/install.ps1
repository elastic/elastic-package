#Requires -Version 5.1
# Install the latest release of elastic-package for Windows.
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/elastic/elastic-package/main/scripts/install.ps1 | iex
#
# To install to a custom directory, set INSTALL_DIR before piping:
#   $env:INSTALL_DIR = "$env:USERPROFILE\.local\bin"
#   irm https://raw.githubusercontent.com/elastic/elastic-package/main/scripts/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$Repo       = 'elastic/elastic-package'
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\elastic-package" }

# Detect true OS architecture.
# $env:PROCESSOR_ARCHITEW6432 is only set when running a 32-bit process on a 64-bit OS,
# and in that case it holds the real OS arch — so check it first.
$OsArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$Arch = switch ($OsArch) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    'x86'   { '386' }
    default {
        Write-Error "Unsupported architecture: $OsArch. Please download manually from https://github.com/$Repo/releases/latest"
        exit 1
    }
}

# Fetch latest release version from GitHub API
Write-Host "Fetching latest elastic-package release..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
$Version = $Release.tag_name -replace '^v', ''

if (-not $Version) {
    Write-Error "Failed to determine the latest release version."
    exit 1
}

# Check currently installed version, skip if already up to date
$CurrentVersion = ''
$ExistingBinary = Join-Path $InstallDir 'elastic-package.exe'
if (Test-Path $ExistingBinary) {
    try {
        $VersionOutput = & $ExistingBinary version 2>$null
        $Match = [regex]::Match($VersionOutput, '\d+\.\d+\.\d+')
        if ($Match.Success) { $CurrentVersion = $Match.Value }
    } catch {}
}

if ($CurrentVersion -eq $Version) {
    Write-Host "elastic-package v$Version is already installed and up to date."
    exit 0
}

if ($CurrentVersion) {
    Write-Host "Upgrading elastic-package v$CurrentVersion -> v$Version (windows/$Arch)..."
} else {
    Write-Host "Installing elastic-package v$Version (windows/$Arch)..."
}

$Filename = "elastic-package_${Version}_windows_${Arch}.zip"
$Url      = "https://github.com/$Repo/releases/download/v$Version/$Filename"

$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    $ZipPath = Join-Path $TmpDir $Filename
    Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
    Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force

    $Binary = Join-Path $TmpDir 'elastic-package.exe'
    if (-not (Test-Path $Binary)) {
        Write-Error "Extraction failed: elastic-package.exe not found in archive."
        exit 1
    }

    # Ensure install directory exists
    if (-not (Test-Path $InstallDir)) {
        Write-Host "Creating install directory $InstallDir..."
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    Copy-Item -Path $Binary -Destination $InstallDir -Force

    # Add install directory to the user PATH if not already present
    $UserPath = [System.Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($UserPath -notlike "*$InstallDir*") {
        [System.Environment]::SetEnvironmentVariable('PATH', "$UserPath;$InstallDir", 'User')
        Write-Host "Added $InstallDir to your PATH (restart your shell for this to take effect)."
    }

    Write-Host ""
    Write-Host "elastic-package v$Version installed to $InstallDir\elastic-package.exe"
    Write-Host ""

    # Verify — update PATH for the current session so the check works immediately
    $env:PATH = "$env:PATH;$InstallDir"
    & (Join-Path $InstallDir 'elastic-package.exe') version
} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
