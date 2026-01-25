#!/usr/bin/env pwsh
# Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# rigrun installer for Windows
# Usage: irm https://rigrun.dev/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$repo = "jeranaias/rigrun"
$name = "rigrun"

Write-Host ""
Write-Host "  rigrun installer" -ForegroundColor Cyan
Write-Host "  Your GPU first. Cloud when needed." -ForegroundColor DarkGray
Write-Host ""

# Detect OS and architecture
$osVersion = [System.Environment]::OSVersion.Platform
if ($osVersion -ne [System.PlatformID]::Win32NT) {
    Write-Host "  Error: This installer is for Windows only." -ForegroundColor Red
    Write-Host "  For Mac/Linux/WSL, use: curl -fsSL https://rigrun.dev/install.sh | sh" -ForegroundColor DarkGray
    exit 1
}

$arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
switch ($arch) {
    "AMD64" {
        $arch = "x86_64"
        Write-Host "  Detected: Windows x64" -ForegroundColor DarkGray
    }
    "ARM64" {
        $arch = "aarch64"
        Write-Host "  Detected: Windows ARM64" -ForegroundColor DarkGray
    }
    default {
        Write-Host "  Error: Unsupported architecture: $arch" -ForegroundColor Red
        exit 1
    }
}

$target = "$arch-pc-windows-msvc"
$ext = ".exe"

# Get latest release
Write-Host "[1/4] Finding latest release..." -ForegroundColor Yellow
try {
    # Use TLS 1.2
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ "User-Agent" = "rigrun-installer" } -UseBasicParsing
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
$tempDir = Join-Path $env:TEMP "rigrun-install-$(Get-Random)"
$zipPath = Join-Path $tempDir "$name.zip"

Write-Host "[2/4] Downloading $assetName..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

try {
    # Use TLS 1.2
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
    $ProgressPreference = 'Continue'
} catch {
    Write-Host "      Error: Failed to download from $downloadUrl" -ForegroundColor Red
    Write-Host "      $_" -ForegroundColor Red
    exit 1
}

# Extract
Write-Host "[3/4] Extracting..." -ForegroundColor Yellow
try {
    Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force
} catch {
    Write-Host "      Error: Failed to extract archive" -ForegroundColor Red
    Write-Host "      $_" -ForegroundColor Red
    exit 1
}

# Install
$installDir = Join-Path $env:LOCALAPPDATA "Programs\rigrun"
Write-Host "[4/4] Installing to $installDir..." -ForegroundColor Yellow

try {
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
} catch {
    Write-Host "      Error: Failed to create installation directory" -ForegroundColor Red
    Write-Host "      $_" -ForegroundColor Red
    exit 1
}

$exePath = Get-ChildItem -Path $tempDir -Filter "rigrun.exe" -Recurse | Select-Object -First 1
if ($exePath) {
    try {
        # Remove existing installation if present
        $targetPath = Join-Path $installDir "rigrun.exe"
        if (Test-Path $targetPath) {
            Remove-Item $targetPath -Force
        }
        Copy-Item -Path $exePath.FullName -Destination $targetPath -Force
    } catch {
        Write-Host "      Error: Failed to copy rigrun.exe" -ForegroundColor Red
        Write-Host "      $_" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "      Error: rigrun.exe not found in archive" -ForegroundColor Red
    exit 1
}

# Verify installation
$installedPath = Join-Path $installDir "rigrun.exe"
if (-not (Test-Path $installedPath)) {
    Write-Host "      Error: Installation failed - rigrun.exe not found at $installedPath" -ForegroundColor Red
    exit 1
}

# Add to PATH
$pathUpdated = $false
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$installDir*") {
    try {
        Write-Host "      Adding to PATH..." -ForegroundColor Yellow
        $newPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        $env:PATH = "$env:PATH;$installDir"
        $pathUpdated = $true
    } catch {
        Write-Host "      Warning: Failed to add to PATH automatically" -ForegroundColor Yellow
        Write-Host "      Add manually: $installDir" -ForegroundColor DarkGray
    }
}

# Cleanup
try {
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
} catch {
    # Ignore cleanup errors
}

# Check for Ollama
Write-Host ""
if (-not (Get-Command ollama -ErrorAction SilentlyContinue)) {
    Write-Host "  ⚠ Ollama not found (required for local inference)" -ForegroundColor Yellow
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

# Only show PATH note if we updated it
if ($pathUpdated) {
    Write-Host "  Note: Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
    Write-Host ""
}
