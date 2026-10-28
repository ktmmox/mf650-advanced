@echo off
setlocal
cd /d "%~dp0"
set "ADB=adb"
if exist "platform-tools\adb.exe" set "ADB=%~dp0platform-tools\adb.exe"
"%ADB%" connect 192.168.100.1:7628
"%ADB%" -s 192.168.100.1:7628 shell "sh /data/airlink-deploy/deploy-device.sh 1 && systemctl stop airlink-console.service && /data/airlink/airlink-console --battery-release && systemctl disable airlink-console.service"
if errorlevel 1 (echo Rollback failed. Check the message above.) else (echo Original UI restored. Disconnect fixes remain enabled.)
pause
