#!/usr/bin/env pwsh
# rigrun installer for Windows
# Usage: irm https://rigrun.dev/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$repo = "rigrun/rigrun"
$name = "rigrun"

Write-Host ""
Write-Host "  rigrun installer" -ForegroundColor Cyan
Write-Host "  Your GPU first. Cloud when needed." -ForegroundColor DarkGray
Write-Host ""

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "x86_64" } else { "i686" }
$target = "$arch-pc-windows-msvc"
$ext = ".exe"

# Get latest release
Write-Host "[1/4] Finding latest release..." -ForegroundColor Yellow
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ "User-Agent" = "rigrun-installer" }
    $version = $release.tag_name
    Write-Host "      Found $version" -ForegroundColor Green
} catch {
    Write-Host "      Could not fetch latest release. Using cargo install instead..." -ForegroundColor Yellow
    Write-Host ""
    if (-not (Get-Command cargo -ErrorAction SilentlyContinue)) {
        Write-Host "      Error: Rust/Cargo is not installed." -ForegroundColor Red
        Write-Host ""
        Write-Host "      To install Rust, visit: https://rustup.rs" -ForegroundColor Yellow
        Write-Host "      After installing Rust, restart your terminal and run this installer again." -ForegroundColor Yellow
        Write-Host ""
        exit 1
    }
    Write-Host "Running: cargo install rigrun" -ForegroundColor Cyan
    cargo install rigrun
    Write-Host ""
    Write-Host "Done! Run 'rigrun' to start." -ForegroundColor Green
    exit 0
}

# Find the right asset
$assetName = "$name-$target.zip"
$asset = $release.assets | Where-Object { $_.name -eq $assetName }

if (-not $asset) {
    # Try alternative naming
    $assetName = "$name-$version-$target.zip"
    $asset = $release.assets | Where-Object { $_.name -eq $assetName }
}

if (-not $asset) {
    Write-Host "      No pre-built binary for $target. Using cargo install..." -ForegroundColor Yellow
    if (-not (Get-Command cargo -ErrorAction SilentlyContinue)) {
        Write-Host "      Error: Rust/Cargo is not installed." -ForegroundColor Red
        Write-Host ""
        Write-Host "      To install Rust, visit: https://rustup.rs" -ForegroundColor Yellow
        Write-Host "      After installing Rust, restart your terminal and run this installer again." -ForegroundColor Yellow
        Write-Host ""
        exit 1
    }
    cargo install rigrun
    exit 0
}

# Download
$downloadUrl = $asset.browser_download_url
$tempDir = Join-Path $env:TEMP "rigrun-install"
$zipPath = Join-Path $tempDir "$name.zip"

Write-Host "[2/4] Downloading $assetName..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing

# Extract
Write-Host "[3/4] Extracting..." -ForegroundColor Yellow
Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

# Install
$installDir = Join-Path $env:USERPROFILE ".rigrun\bin"
Write-Host "[4/4] Installing to $installDir..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$exePath = Get-ChildItem -Path $tempDir -Filter "rigrun.exe" -Recurse | Select-Object -First 1
if ($exePath) {
    Copy-Item -Path $exePath.FullName -Destination (Join-Path $installDir "rigrun.exe") -Force
} else {
    Write-Host "      Error: rigrun.exe not found in archive" -ForegroundColor Red
    exit 1
}

# Add to PATH
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$installDir*") {
    Write-Host "      Adding to PATH..." -ForegroundColor Yellow
    [Environment]::SetEnvironmentVariable("PATH", "$userPath;$installDir", "User")
    $env:PATH = "$env:PATH;$installDir"
}

# Cleanup
Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue

# Check for Ollama
Write-Host ""
if (-not (Get-Command ollama -ErrorAction SilentlyContinue)) {
    Write-Host "  [!] Ollama not found (required for local inference)" -ForegroundColor Yellow
    Write-Host ""
    $install = Read-Host "  Install Ollama now? (Y/n)"
    if ($install -ne 'n' -and $install -ne 'N') {
        Write-Host "  Downloading Ollama installer..." -ForegroundColor Yellow
        $ollamaUrl = "https://ollama.com/download/OllamaSetup.exe"
        $ollamaPath = Join-Path $env:TEMP "OllamaSetup.exe"
        Invoke-WebRequest -Uri $ollamaUrl -OutFile $ollamaPath -UseBasicParsing
        Write-Host "  Running Ollama installer..." -ForegroundColor Yellow
        Start-Process -FilePath $ollamaPath -Wait
        Remove-Item $ollamaPath -ErrorAction SilentlyContinue
        Write-Host "  Ollama installed!" -ForegroundColor Green
    } else {
        Write-Host "  Install Ollama later: https://ollama.com/download" -ForegroundColor DarkGray
    }
}

Write-Host ""
Write-Host "  Done!" -ForegroundColor Green
Write-Host ""
Write-Host "  Installed: $installDir\rigrun.exe" -ForegroundColor DarkGray
Write-Host ""
Write-Host "  Get started:" -ForegroundColor Cyan
Write-Host "    rigrun              # Start the server"
Write-Host "    rigrun status       # Check GPU and stats"
Write-Host "    rigrun models       # See available models"
Write-Host ""
Write-Host "  Note: Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
Write-Host ""
