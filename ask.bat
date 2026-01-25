@echo off
REM Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
REM SPDX-License-Identifier: AGPL-3.0-or-later
REM
REM ask.bat - Ultra-simple rigrun query helper for Windows
REM Usage: ask "your question here"

if "%~1"=="" (
    echo Usage: ask "your question here"
    exit /b 1
)

curl -sX POST localhost:8787/v1/chat/completions -H "Content-Type: application/json" -d "{\"model\":\"local\",\"messages\":[{\"role\":\"user\",\"content\":\"%~1\"}]}" 2>nul
