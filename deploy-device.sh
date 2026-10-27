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
if [ ! -f /www/system_status_data.asp ] || [ ! -f /www/Advanced_Settings_Content.asp ]; then
    [ -s "$base/native/www/system_status_data.asp" ] && [ -s "$base/native/www/Advanced_Settings_Content.asp" ] || { echo 'Unsupported firmware: native settings missing'; exit 1; }
    mkdir -p /www
    if [ "$mode" = 1 ]; then cp -a "$base/native/www/." /www/; fi
    [ -f /www/index.asp ] || cp "$backup/index.asp" /www/index.asp
fi
[ -s "$stage/native-runtime.tar.gz" ] && [ -s "$stage/native-httpd.conf" ] || { echo "Native runtime payload missing"; exit 1; }
command -v systemctl >/dev/null
command -v ubus >/dev/null
[ -x /usr/bin/atcmd ]
mkdir -p "$base" "$backup"
# Keep the original homepage backup across subsequent deployments.
if [ ! -s "$backup/index.asp" ]; then
    if grep -q 'target.port=' /www/index.asp; then
        echo 'Original homepage backup is missing. Restore it before installing.'; exit 1
    fi
    cp /www/index.asp "$backup/index.asp"
fi
# Snapshot exactly what this deployment changes, before stopping the service.
previous="$backup/deploy-$(date +%Y%m%d-%H%M%S)-$$"
mkdir -p "$previous"
cp /www/index.asp "$previous/index.asp"
[ ! -f "$base/airlink-console" ] || cp "$base/airlink-console" "$previous/airlink-console"
[ ! -f "$service" ] || cp "$service" "$previous/service"
[ ! -f "$override" ] || cp "$override" "$previous/native-httpd.conf"
was_active=0
was_enabled=0
systemctl is-active --quiet airlink-console.service && was_active=1
systemctl is-enabled --quiet airlink-console.service && was_enabled=1
restore() {
    code=$?
    trap - EXIT
    if [ "$code" != 0 ]; then
        echo 'Deployment failed; restoring previous entry and service.'
        systemctl stop airlink-console.service || true
        cp "$previous/index.asp" /www/index.asp || true
        if [ -f "$previous/airlink-console" ]; then cp "$previous/airlink-console" "$base/airlink-console"; chmod 755 "$base/airlink-console"; fi
        if [ -f "$previous/service" ]; then cp "$previous/service" "$service"; else rm -f "$service"; fi
        if [ -f "$previous/native-httpd.conf" ]; then mkdir -p "$(dirname "$override")"; cp "$previous/native-httpd.conf" "$override"; else rm -f "$override"; fi
        systemctl daemon-reload || true
        systemctl restart httpd.service || true
        if [ "$was_enabled" = 1 ]; then systemctl enable airlink-console.service || true; else systemctl disable airlink-console.service || true; fi
        [ "$was_active" != 1 ] || systemctl start airlink-console.service || true
        echo "Recovery snapshot: $previous"
    fi
    exit "$code"
}
trap restore EXIT
# Keep an independent copy of all native ASP templates and the matching engine.
# Existing runtime stays in place so a failed app upgrade cannot lose its interfaces.
if [ ! -s "$base/native/httpd" ] || [ ! -s "$base/native/www/system_status_data.asp" ]; then
    mkdir -p "$base/native.new"
    tar -xzf "$stage/native-runtime.tar.gz" -C "$base/native.new"
    [ -s "$base/native.new/httpd" ] && [ -s "$base/native.new/www/start_apply.htm" ]
    chmod 755 "$base/native.new/httpd"
    if [ -d "$base/native" ]; then mv "$base/native" "$previous/native-incomplete"; fi
    mv "$base/native.new" "$base/native"
fi
if [ "$mode" = 2 ]; then
    cp "$stage/default-index.asp" "$base/native/www/index.asp"
    mkdir -p "$(dirname "$override")"
    cp "$stage/native-httpd.conf" "$override"
else
    if [ ! -x /usr/bin/httpd ]; then cp "$base/native/httpd" /usr/bin/httpd; chmod 755 /usr/bin/httpd; fi
    rm -f "$override"
fi
systemctl daemon-reload
systemctl restart httpd.service
native_ready=0
for attempt in 1 2 3 4 5 6 7 8; do
    if [ -n "$(wget -qO- http://127.0.0.1/system_status_data.asp 2>/dev/null)" ]; then native_ready=1; break; fi
    sleep 1
done
[ "$native_ready" = 1 ] || { echo "Native runtime startup failed"; exit 1; }
# Read-only interface checks; never submit settings while deploying.
for endpoint in system_status_data.asp Advanced_Settings_Content.asp Advanced_Wireless2g_Content.asp Advanced_Wireless_Content.asp sms_in.asp; do
    [ -n "$(wget -qO- "http://127.0.0.1/$endpoint")" ] || { echo "Native interface check failed: $endpoint"; exit 1; }
done
cp "$stage/airlink-console" "$base/airlink-console.new"
chmod 755 "$base/airlink-console.new"
systemctl stop airlink-console.service || true
mv "$base/airlink-console.new" "$base/airlink-console"
cp "$stage/airlink-console.service" "$service"
systemctl daemon-reload
systemctl enable airlink-console.service
systemctl start airlink-console.service
ready=0
for attempt in 1 2 3 4 5; do
    if wget -qO- http://192.168.100.1:8780/healthz | grep -q '"status":"ok"'; then ready=1; break; fi
    sleep 1
done
[ "$ready" = 1 ] || { echo 'New console health check failed'; exit 1; }
# Replace the entry only after the new service passes its health check.
if [ "$mode" = 1 ]; then
    cp "$backup/index.asp" /www/index.asp.new
else
    cp "$stage/default-index.asp" /www/index.asp.new
fi
mv /www/index.asp.new /www/index.asp
printf '%s\n' "$mode" > "$base/deployment-mode"
sync
trap - EXIT
echo "Deployment successful. Mode: $mode"
echo "Recovery snapshot: $previous"
