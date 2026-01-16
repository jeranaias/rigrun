@echo off
REM Model Benchmark Script for RX 9070 XT (Vulkan)
REM Tests response time for various model sizes

echo =============================================
echo Model Benchmark - RX 9070 XT with Vulkan
echo =============================================
echo.

set PROMPT="Write a Python function to check if a number is prime. Include docstring and type hints."

echo Testing qwen2.5:14b (baseline)...
echo %TIME%
ollama run qwen2.5:14b %PROMPT%
echo %TIME%
echo.
echo =============================================

echo Testing mistral-small (22B)...
echo %TIME%
ollama run mistral-small %PROMPT%
echo %TIME%
echo.
echo =============================================

echo Testing codestral:22b...
echo %TIME%
ollama run codestral:22b %PROMPT%
echo %TIME%
echo.
echo =============================================

echo Testing gemma2:27b...
echo %TIME%
ollama run gemma2:27b %PROMPT%
echo %TIME%
echo.
echo =============================================

echo Benchmark complete!
pause
