@echo off
setlocal
adb connect 192.168.100.1:7628
adb -s 192.168.100.1:7628 shell "mkdir -p /www; if [ ! -f /www/system_status_data.asp ] && [ -d /data/airlink/native/www ]; then cp -a /data/airlink/native/www/. /www/; fi; if [ ! -x /usr/bin/httpd ] && [ -x /data/airlink/native/httpd ]; then cp /data/airlink/native/httpd /usr/bin/httpd; chmod 755 /usr/bin/httpd; fi; test -f /data/airlink-backup/index.asp && cp /data/airlink-backup/index.asp /www/index.asp; rm -f /etc/systemd/system/httpd.service.d/50-airlink-native.conf; systemctl daemon-reload; systemctl restart httpd.service; systemctl stop airlink-console.service; /data/airlink/airlink-console --battery-release; systemctl disable airlink-console.service"
echo Original console is still available: http://192.168.100.1
pause
