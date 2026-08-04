#!/bin/sh
# One-shot capture of systemd-udevd's debug log around a CAN adapter
# hotplug event -- for diagnosing why a udev rule's RUN key isn't taking
# effect (e.g. scripts/setup-socketcan.sh's slcand rule) when `udevadm
# test` shows the rule matching and queuing the right command, but real
# hotplug produces nothing (no process, no interface).
#
# Usage: run this, then physically unplug/replug the adapter within the
# capture window. Bumps udev's log level to debug for the duration only,
# and restores it to info afterward regardless of how the script exits.
#
# Must be run as root (see `make debug-udev-can` in the top-level
# Makefile, which invokes this via sudo). No sudo calls in here on
# purpose, matching scripts/setup-socketcan.sh.

set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "debug-udev-can.sh must be run as root (try: make debug-udev-can)" >&2
    exit 1
fi

CAPTURE_SECONDS=${1:-15}

udevadm control --log-priority=debug
trap 'udevadm control --log-priority=info' EXIT

journalctl -u systemd-udevd -f &
JPID=$!

echo "Watching systemd-udevd at debug level for ${CAPTURE_SECONDS}s -- unplug and replug the CAN adapter now." >&2
sleep "$CAPTURE_SECONDS"

kill "$JPID" 2>/dev/null || true
wait "$JPID" 2>/dev/null || true
