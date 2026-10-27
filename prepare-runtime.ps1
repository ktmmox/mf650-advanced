param([string]$Device,[string]$AdbPath)
$ErrorActionPreference = 'Stop'
$script = @'
set -eu
stage=/data/airlink-export-native
mkdir -p "$stage/source"
if [ -x /data/airlink/native/httpd ] && [ -s /data/airlink/native/www/system_status_data.asp ]; then
    cp -a /data/airlink/native/www "$stage/source/"
    cp /data/airlink/native/httpd "$stage/source/httpd"
else
    test -x /usr/bin/httpd
    test -s /www/system_status_data.asp
    cp -a /www "$stage/source/"
    cp /usr/bin/httpd "$stage/source/httpd"
fi
tar -czf "$stage/native-runtime.tar.gz" -C "$stage/source" .
'@
$tempScript = Join-Path $PSScriptRoot 'export-native.tmp.sh'
try {
    [IO.File]::WriteAllText($tempScript, $script.Replace("`r`n","`n"), [Text.UTF8Encoding]::new($false))
    & $AdbPath -s $Device push $tempScript /data/airlink-export-native.sh
    if ($LASTEXITCODE -ne 0) { throw '接口导出脚本上传失败' }
    & $AdbPath -s $Device shell sh /data/airlink-export-native.sh
    if ($LASTEXITCODE -ne 0) { throw '接口运行环境导出失败，未部署后台' }
    & $AdbPath -s $Device pull /data/airlink-export-native/native-runtime.tar.gz (Join-Path $PSScriptRoot 'native-runtime.tar.gz')
    if ($LASTEXITCODE -ne 0) { throw '接口运行环境下载失败' }
} finally {
    Remove-Item -LiteralPath $tempScript -ErrorAction SilentlyContinue
}
