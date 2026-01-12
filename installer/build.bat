@echo off
echo Building rigrun installer...
echo.

:: Check for Inno Setup
where iscc >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo ERROR: Inno Setup not found!
    echo.
    echo Please install Inno Setup from: https://jrsoftware.org/isdl.php
    echo Or install via: winget install JRSoftware.InnoSetup
    echo.
    pause
    exit /b 1
)

:: Build Rust binary first
echo [1/2] Building rigrun binary...
cd /d %~dp0..
cargo build --release
if %ERRORLEVEL% neq 0 (
    echo ERROR: Cargo build failed!
    pause
    exit /b 1
)

:: Build installer
echo.
echo [2/2] Building installer...
cd /d %~dp0
iscc rigrun-setup.iss
if %ERRORLEVEL% neq 0 (
    echo ERROR: Installer build failed!
    pause
    exit /b 1
)

echo.
echo ========================================
echo SUCCESS! Installer created:
echo   installer\output\rigrun-0.1.0-setup.exe
echo ========================================
echo.
pause
