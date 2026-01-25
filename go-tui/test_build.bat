@echo off
cd /d C:\rigrun\go-tui
echo Building rigrun...
go build ./... 2>&1
echo Exit code: %ERRORLEVEL%
pause
