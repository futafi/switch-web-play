#!/bin/bash
set -e

echo "=== Starting D-Bus ==="
mkdir -p /run/dbus
dbus-daemon --system --nofork &
sleep 1

echo "=== Unblocking Bluetooth (rfkill) ==="
rfkill unblock bluetooth 2>/dev/null || true

echo "=== Starting BlueZ (compat, no plugins) ==="
BLUETOOTHD=$(command -v bluetoothd || echo /usr/libexec/bluetooth/bluetoothd)
"$BLUETOOTHD" --compat --noplugin=* &
sleep 1

# Marker file so `nuxbt check` passes
mkdir -p /run/systemd/system/bluetooth.service.d
cat > /run/systemd/system/bluetooth.service.d/nuxbt.conf <<'CONF'
[Service]
ExecStart=
ExecStart=/usr/libexec/bluetooth/bluetoothd --compat --noplugin=*
CONF

echo "=== Granting socket capabilities to Python ==="
PYTHON_PATH=$(readlink -f "$(which python3)")
setcap 'cap_net_raw,cap_net_admin,cap_net_bind_service+eip' "$PYTHON_PATH" 2>/dev/null || true

echo "=== Waiting for BlueZ on D-Bus ==="
for i in $(seq 1 10); do
    if bluetoothctl list 2>/dev/null | grep -q Controller; then
        echo "BlueZ is ready"
        break
    fi
    echo "  waiting... ($i)"
    sleep 1
done

echo "=== Bluetooth adapter ==="
bluetoothctl show 2>/dev/null || echo "(no adapter detected)"
echo "========================="

exec "$@"
