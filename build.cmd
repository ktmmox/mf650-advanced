@echo off
setlocal
cd /d "%~dp0"
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags="-s -w" -o airlink-console .
pause
