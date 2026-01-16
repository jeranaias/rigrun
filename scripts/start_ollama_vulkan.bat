@echo off
REM ============================================
REM Ollama Vulkan Startup Script for RDNA 4 GPUs
REM ============================================
REM
REM This script starts Ollama with Vulkan GPU backend enabled.
REM Required for AMD RDNA 4 GPUs (RX 9070, 9070 XT, 9060 series)
REM which are not yet supported by ROCm/HIP on Windows.
REM
REM The Vulkan backend provides:
REM - Full GPU acceleration on RDNA 4 (RX 9070 XT = 16GB VRAM)
REM - fp16 and bf16 support
REM - Cooperative matrix operations (KHR_coopmat)
REM - Multi-GPU support (can use iGPU shared memory for larger models)
REM
REM Performance tested on RX 9070 XT:
REM - 3B model: ~2-3 seconds
REM - 14B model: ~10-15 seconds
REM - 32B model: Works via iGPU shared memory
REM
REM Usage: Double-click this file or run from command line
REM ============================================

echo Starting Ollama with Vulkan GPU backend...
echo.

REM Set Vulkan as the GPU backend
set OLLAMA_VULKAN=1

REM Optional: Enable debug logging (uncomment if needed)
REM set OLLAMA_DEBUG=INFO

REM Start Ollama server
"%LOCALAPPDATA%\Programs\Ollama\ollama.exe" serve
