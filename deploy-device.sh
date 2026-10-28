#!/bin/sh
set -eu
mode="${1:-}"
case "$mode" in 1|2) ;; *) echo 'Invalid deployment option'; exit 1;; esac
stage=/data/airlink-deploy
base=/data/airlink
backup=/data/airlink-backup
service=/etc/systemd/system/airlink-console.service
override=/etc/systemd/system/httpd.service.d/50-airlink-native.conf
[ "$(id -u)" = 0 ] || { echo 'Root ADB required'; exit 1; }
for name in airlink-console airlink-console.service native-runtime.tar.gz native-httpd.conf loopback-bind.so fix-disconnect.sh; do
    [ -s "$stage/$name" ] || { echo "Missing payload: $name"; exit 1; }
done
command -v systemctl >/dev/null
command -v ubus >/dev/null
[ -x /usr/bin/atcmd ]
# Exported native runtime must already exist before retiring the original UI.
[ -s /www/system_status_data.asp ] || [ -s "$base/native/www/system_status_data.asp" ] || { echo 'Unsupported firmware'; exit 1; }
mkdir -p "$base" "$backup"
previous="$backup/deploy-$(date +%Y%m%d-%H%M%S)-$$"
mkdir -p "$previous"
[ ! -f "$service" ] || cp "$service" "$previous/service"
[ ! -f "$override" ] || cp "$override" "$previous/native-httpd.conf"
[ ! -f "$base/airlink-console" ] || cp "$base/airlink-console" "$previous/airlink-console"
[ ! -e /usr/bin/httpd ] || cp /usr/bin/httpd "$previous/httpd-original"
[ ! -f "$base/native/loopback-bind.so" ] || cp "$base/native/loopback-bind.so" "$previous/loopback-bind.so"
[ ! -f /www/index.asp ] || cp /www/index.asp "$previous/index.asp"
[ ! -f "$base/native/www/index.asp" ] || cp "$base/native/www/index.asp" "$previous/native-index.asp"
was_active=0; was_enabled=0
systemctl is-active --quiet airlink-console && was_active=1
systemctl is-enabled --quiet airlink-console && was_enabled=1
restore() {
    result=$?
    trap - EXIT
    if [ "$result" != 0 ]; then
        echo 'Deployment failed; restoring previous runtime.'
        systemctl stop airlink-console httpd || true
        if [ -d "$previous/www" ]; then mkdir -p /www; cp -a "$previous/www/." /www/; fi
        [ ! -f "$previous/index.asp" ] || cp "$previous/index.asp" /www/index.asp
        [ ! -f "$previous/httpd-original" ] || cp "$previous/httpd-original" /usr/bin/httpd
        [ ! -f "$previous/loopback-bind.so" ] || cp "$previous/loopback-bind.so" "$base/native/loopback-bind.so"
        [ ! -f "$previous/native-index.asp" ] || cp "$previous/native-index.asp" "$base/native/www/index.asp"
        if [ -f "$previous/service" ]; then cp "$previous/service" "$service"; else rm -f "$service"; fi
        if [ -f "$previous/native-httpd.conf" ]; then mkdir -p "$(dirname "$override")"; cp "$previous/native-httpd.conf" "$override"; else rm -f "$override"; fi
        if [ -f "$previous/airlink-console" ]; then cp "$previous/airlink-console" "$base/airlink-console"; chmod 755 "$base/airlink-console"; fi
        systemctl daemon-reload || true
        systemctl restart httpd || true
        if [ "$was_enabled" = 1 ]; then systemctl enable airlink-console || true; else systemctl disable airlink-console || true; fi
        [ "$was_active" != 1 ] || systemctl start airlink-console || true
        echo "Recovery snapshot: $previous"
    fi
    exit "$result"
}
trap restore EXIT
if [ ! -s "$base/native/httpd" ] || [ ! -s "$base/native/www/system_status_data.asp" ]; then
    mkdir -p "$base/native.new"
    tar -xzf "$stage/native-runtime.tar.gz" -C "$base/native.new"
    [ -s "$base/native.new/httpd" ] && [ -s "$base/native.new/www/start_apply.htm" ]
    chmod 755 "$base/native.new/httpd"
    [ ! -d "$base/native" ] || mv "$base/native" "$previous/native-incomplete"
    mv "$base/native.new" "$base/native"
fi
# Always preserve an actual original homepage; never save a redirect as original.
if [ ! -s "$backup/index.asp" ]; then
    original=/www/index.asp
    [ -s "$original" ] || original="$base/native/www/index.asp"
    [ -s "$original" ] && ! grep -q 'target.port=' "$original" || { echo 'Original homepage backup unavailable'; exit 1; }
    cp "$original" "$backup/index.asp"
fi
systemctl stop airlink-console httpd
if [ "$mode" = 2 ]; then
    cp "$stage/loopback-bind.so" "$base/native/loopback-bind.so"
    chmod 755 "$base/native/loopback-bind.so"
    mkdir -p "$(dirname "$override")"
    cp "$stage/native-httpd.conf" "$override"
    listen=192.168.100.1:80
else
    mkdir -p /www
    cp -a "$base/native/www/." /www/
    cp "$backup/index.asp" /www/index.asp
    if [ ! -x /usr/bin/httpd ]; then cp "$base/native/httpd" /usr/bin/httpd; chmod 755 /usr/bin/httpd; fi
    rm -f "$override"
    listen=192.168.100.1:8780
fi
cp "$stage/airlink-console" "$base/airlink-console.new"
chmod 755 "$base/airlink-console.new"
mv "$base/airlink-console.new" "$base/airlink-console"
cp "$stage/airlink-console.service" "$service"
printf '\nEnvironment=CONSOLE_LISTEN=%s\n' "$listen" > "$previous/listen-environment"
# Insert the listen setting in [Service], not in [Install].
sed "/^WorkingDirectory=/a Environment=CONSOLE_LISTEN=$listen" "$stage/airlink-console.service" > "$service"
systemctl daemon-reload
systemctl restart httpd
native_ready=0
for i in 1 2 3 4 5 6 7 8; do
    if [ -n "$(wget -qO- http://127.0.0.1/system_status_data.asp 2>/dev/null)" ]; then native_ready=1; break; fi
    sleep 1
done
[ "$native_ready" = 1 ] || { echo 'Native interface startup failed'; exit 1; }
for endpoint in Advanced_Settings_Content.asp Advanced_Wireless2g_Content.asp Advanced_Wireless_Content.asp sms_in.asp; do
    [ -n "$(wget -qO- "http://127.0.0.1/$endpoint")" ] || { echo "Interface check failed: $endpoint"; exit 1; }
done
systemctl enable airlink-console
systemctl start airlink-console
ready=0
for i in 1 2 3 4 5 6 7 8; do
    if wget -qO- "http://$listen/healthz" 2>/dev/null | grep -q '"status":"ok"'; then ready=1; break; fi
    sleep 1
done
[ "$ready" = 1 ] || { echo 'New console health check failed'; exit 1; }
if [ "$mode" = 2 ]; then
    # Remove original UI and engine from their active locations; retain rollback copies.
    systemctl stop httpd
    [ ! -d /www ] || mv /www "$previous/www"
    mkdir -p /www
    [ ! -f /usr/bin/httpd ] || mv /usr/bin/httpd "$previous/httpd-retired"
    systemctl start httpd
    sleep 2
    [ -n "$(wget -qO- http://127.0.0.1/system_status_data.asp)" ]
fi
sh "$stage/fix-disconnect.sh" "$previous"
printf '%s\n' "$mode" > "$base/deployment-mode"
sync
trap - EXIT
echo "Deployment successful. Mode: $mode, console: http://$listen/"
echo "Recovery snapshot: $previous"
