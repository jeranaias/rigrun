# check_size.ps1 - Build release binary and report size
# Usage: .\scripts\check_size.ps1
#
# This script builds the release binary with size optimizations and reports
# the final binary size. Target: <15 MB (ideally <5 MB)

param(
    [switch]$SkipBuild,
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"

# Change to project root
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptDir
Set-Location $projectRoot

Write-Host "=== rigrun Binary Size Check ===" -ForegroundColor Cyan
Write-Host ""

if (-not $SkipBuild) {
    Write-Host "Building release binary with size optimizations..." -ForegroundColor Yellow
    Write-Host "(opt-level=z, lto=true, codegen-units=1, panic=abort, strip=true)"
    Write-Host ""

    if ($Verbose) {
        cargo build --release
    } else {
        cargo build --release 2>&1 | Out-Null
    }

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }

    Write-Host "Build complete!" -ForegroundColor Green
    Write-Host ""
}

$binaryPath = "target\release\rigrun.exe"

if (-not (Test-Path $binaryPath)) {
    Write-Host "Binary not found at: $binaryPath" -ForegroundColor Red
    Write-Host "Run without -SkipBuild to build first."
    exit 1
}

$fileInfo = Get-Item $binaryPath
$sizeBytes = $fileInfo.Length
$sizeMB = $sizeBytes / 1MB
$sizeKB = $sizeBytes / 1KB

Write-Host "Binary: $binaryPath" -ForegroundColor White
Write-Host "Size:   $([math]::Round($sizeMB, 2)) MB ($([math]::Round($sizeKB, 0)) KB)" -ForegroundColor White
Write-Host ""

# Size targets
$targetMax = 15
$targetIdeal = 5

if ($sizeMB -lt $targetIdeal) {
    Write-Host "Status: EXCELLENT - Under ${targetIdeal}MB target!" -ForegroundColor Green
} elseif ($sizeMB -lt $targetMax) {
    Write-Host "Status: GOOD - Under ${targetMax}MB target" -ForegroundColor Yellow
} else {
    Write-Host "Status: NEEDS WORK - Over ${targetMax}MB target" -ForegroundColor Red
}

Write-Host ""
Write-Host "=== Optimization Tips ===" -ForegroundColor Cyan
Write-Host "If size is too large, consider:"
Write-Host "  - Reducing tokio features (currently 'full')"
Write-Host "  - Using 'time' crate instead of 'chrono'"
Write-Host "  - Using ureq instead of reqwest (no async needed for HTTP client)"
Write-Host "  - Running UPX compression: upx --best target\release\rigrun.exe"
