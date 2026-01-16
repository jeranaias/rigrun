@echo off
REM ============================================
REM Set OLLAMA_VULKAN=1 Permanently (System-wide)
REM ============================================
REM
REM This script sets OLLAMA_VULKAN=1 as a system environment
REM variable so Ollama always uses Vulkan GPU backend.
REM
REM Required for AMD RDNA 4 GPUs (RX 9070, 9070 XT, 9060 series)
REM
REM REQUIRES: Run as Administrator
REM ============================================

echo This script will set OLLAMA_VULKAN=1 permanently.
echo.
echo After running this, Ollama will ALWAYS use Vulkan GPU.
echo You can remove it later with: setx OLLAMA_VULKAN "" /M
echo.
pause

setx OLLAMA_VULKAN 1 /M

if %errorlevel%==0 (
    echo.
    echo SUCCESS! OLLAMA_VULKAN=1 has been set system-wide.
    echo.
    echo Please restart any running Ollama instances.
    echo The change will take effect for new command prompts.
) else (
    echo.
    echo ERROR: Failed to set environment variable.
    echo Please run this script as Administrator:
    echo   Right-click ^> Run as administrator
)

pause
