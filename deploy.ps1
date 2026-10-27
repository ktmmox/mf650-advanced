$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
Set-Location -LiteralPath $PSScriptRoot
Write-Host "`n  5G MIFI 一键部署" -ForegroundColor Cyan
Write-Host '  1. 双后台兼容：保留原机后台，新版使用 8780 端口'
Write-Host '  2. 替换原机后台：默认入口使用新版，原生接口独立保存'
Write-Host '  0. 退出'
$mode = Read-Host '请选择 1 或 2'
if ($mode -eq '0') { exit 0 }
if ($mode -notin @('1','2')) { throw '选项无效，未执行部署。' }
$device = Read-Host 'ADB 地址或设备序列号（回车使用 192.168.100.1:7628）'
if ([string]::IsNullOrWhiteSpace($device)) { $device = '192.168.100.1:7628' }
if ($device -notmatch '^[A-Za-z0-9_.:-]+$') { throw '设备地址格式无效。' }
$adbCommand = Get-Command adb.exe -ErrorAction SilentlyContinue
$adb = if (Test-Path -LiteralPath "$PSScriptRoot\platform-tools\adb.exe") { "$PSScriptRoot\platform-tools\adb.exe" } elseif ($adbCommand) { $adbCommand.Source } else { throw '未找到 ADB。请安装官方 Android Platform Tools 并加入 PATH，或放入本目录 platform-tools 文件夹。' }
foreach ($file in @('airlink-console','airlink-console.service','default-index.asp','deploy-device.sh','native-httpd.conf','prepare-runtime.ps1')) {
    if (!(Test-Path -LiteralPath (Join-Path $PSScriptRoot $file))) { throw "缺少文件：$file，请完整解压部署包。" }
}
$manifestPath = Join-Path $PSScriptRoot 'SHA256.json'
if (!(Test-Path -LiteralPath $manifestPath)) { throw '缺少 SHA256.json，请重新完整解压部署包。' }
$manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
foreach ($entry in $manifest.PSObject.Properties) {
    $checkFile = Join-Path $PSScriptRoot $entry.Name
    if (!(Test-Path -LiteralPath $checkFile) -or (Get-FileHash -LiteralPath $checkFile -Algorithm SHA256).Hash -ne $entry.Value) { throw "文件校验失败：$($entry.Name)，未执行上传。" }
}
function Invoke-Adb {
    param([string[]]$AdbArguments)
    & $adb @AdbArguments
    if ($LASTEXITCODE -ne 0) { throw "ADB 操作失败（退出码 $LASTEXITCODE）。部署未完成。" }
}
if ($device -match ':\d+$') { Invoke-Adb -AdbArguments @('connect',$device) }
Invoke-Adb -AdbArguments @('-s',$device,'get-state')
$uid = & $adb -s $device shell id -u
if ($LASTEXITCODE -ne 0 -or ($uid -join '').Trim() -ne '0') { throw '需要设备已开启 root ADB，当前未获得 root 权限。' }
if (!(Test-Path -LiteralPath (Join-Path $PSScriptRoot 'native-runtime.tar.gz'))) {
    Write-Host '正在从本机保存匹配的原生接口环境…'
    & (Join-Path $PSScriptRoot 'prepare-runtime.ps1') -Device $device -AdbPath $adb
}
Invoke-Adb -AdbArguments @('-s',$device,'shell','mkdir -p /data/airlink-deploy')
foreach ($file in @('airlink-console','airlink-console.service','default-index.asp','deploy-device.sh','native-runtime.tar.gz','native-httpd.conf')) {
    Invoke-Adb -AdbArguments @('-s',$device,'push',(Join-Path $PSScriptRoot $file),"/data/airlink-deploy/$file")
}
Invoke-Adb -AdbArguments @('-s',$device,'shell',"sh /data/airlink-deploy/deploy-device.sh $mode")
Write-Host "`n部署完成。" -ForegroundColor Green
if ($mode -eq '1') { Write-Host '原后台：http://192.168.100.1/   新版：http://192.168.100.1:8780/' } else { Write-Host '默认后台：http://192.168.100.1/（自动进入新版）' }
Write-Host '若设备管理 IP 不同，请使用实际管理 IP。用户名和密码沿用设备当前管理员账号。'
