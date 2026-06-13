#Requires -Version 5
# hetzner CLI installer for Windows — downloads the prebuilt hetzner.exe and puts it on your PATH.
# No Go, no git clone required.
#
#   irm https://raw.githubusercontent.com/Dakaric/hetzner-cli/main/get.ps1 | iex
#
# Knobs (env vars):
#   HETZNER_BIN_DIR   where to install   (default: %LOCALAPPDATA%\Programs\hetzner)
#   HETZNER_VERSION   tag to install     (default: latest, e.g. v0.1.0)
$ErrorActionPreference = 'Stop'

$repo    = 'Dakaric/hetzner-cli'
$binDir  = if ($env:HETZNER_BIN_DIR) { $env:HETZNER_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\hetzner' }
$version = if ($env:HETZNER_VERSION) { $env:HETZNER_VERSION } else { 'latest' }

# Only an amd64 build is published; on Windows-on-ARM it runs via x64 emulation.
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
    Write-Warning 'No native arm64 build — installing the amd64 binary (runs under x64 emulation on Windows 11 ARM).'
}

$asset = 'hetzner_windows_amd64.zip'
$url = if ($version -eq 'latest') {
    "https://github.com/$repo/releases/latest/download/$asset"
} else {
    "https://github.com/$repo/releases/download/$version/$asset"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("hetzner-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    Write-Host "Downloading $asset ($version)..."
    $zip = Join-Path $tmp $asset
    Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing

    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    $exe = Join-Path $tmp 'hetzner.exe'
    if (-not (Test-Path $exe)) { throw "archive did not contain hetzner.exe" }

    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    Copy-Item -Force $exe (Join-Path $binDir 'hetzner.exe')
    Write-Host "Installed: $binDir\hetzner.exe"
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$binDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$binDir", 'User')
    Write-Host "Added $binDir to your user PATH. Restart your shell to pick it up."
}

# 'hetzner ssh' / 'hetzner exec' need the OpenSSH client (ssh.exe).
if (-not (Get-Command ssh -ErrorAction SilentlyContinue)) {
    Write-Warning "ssh.exe not found. For 'hetzner ssh'/'exec', enable the OpenSSH Client: Settings > Apps > Optional Features > OpenSSH Client."
}

Write-Host ""
Write-Host "Next: get a token (Console > your project > Security > API tokens > Generate, Read & Write), then:"
Write-Host "  hetzner login     # paste the token; it is validated and saved"
Write-Host "  hetzner status    # confirm it works"
