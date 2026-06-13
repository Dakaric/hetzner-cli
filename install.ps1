#Requires -Version 5
# Build and install the hetzner CLI on Windows (PowerShell), then run onboarding.
$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is required to build from source: https://go.dev/dl/  (or download a prebuilt hetzner.exe from the GitHub Releases page and put it on your PATH)."
    exit 1
}

Write-Host "Building hetzner.exe..."
go build -o hetzner.exe .

$binDir = Join-Path $env:LOCALAPPDATA 'Programs\hetzner'
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
Copy-Item -Force 'hetzner.exe' (Join-Path $binDir 'hetzner.exe')
Write-Host "Installed: $binDir\hetzner.exe"

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
Write-Host "Onboarding: paste your Hetzner Cloud API token when prompted."
& (Join-Path $binDir 'hetzner.exe') login
