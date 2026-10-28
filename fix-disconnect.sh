#!/bin/sh
set -eu
backup="${1:?backup directory required}/legacy-disconnect"
mkdir -p "$backup"
# Only the three previously diagnosed legacy services. Never stop network_mgr,
# qmi_svr, the SIM driver, httpd compatibility service, or charging services.
for unit in net_check.service thttpd.service webserver.service; do
    systemctl stop "$unit" 2>/dev/null || true
    systemctl disable "$unit" 2>/dev/null || true
    path="/etc/systemd/system/$unit"
    if [ -f "$path" ] && [ ! -L "$path" ]; then
        mv "$path" "$backup/$unit"
    fi
    if [ ! -e "$path" ] && [ ! -L "$path" ]; then ln -s /dev/null "$path"; fi
done
for name in net_check.sh thttpd webserver; do
    path="/usr/bin/$name"
    [ ! -f "$path" ] || mv "$path" "$backup/$name"
done
systemctl daemon-reload
for unit in net_check.service thttpd.service webserver.service; do
    systemctl reset-failed "$unit" 2>/dev/null || true
    if systemctl is-active --quiet "$unit"; then echo "Legacy service still active: $unit"; exit 1; fi
done
echo "Legacy disconnect sources disabled and backed up: $backup"
