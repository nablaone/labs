#!/bin/sh
# Sets up persistent SocketCAN on this host for both kinds of CAN-USB
# adapter firmware, since which one's plugged in can change over time:
#
#   - candleLight/gs_usb firmware -- adapter shows up as a native `can*`
#     netdevice directly, kernel-side.
#   - slcan firmware -- adapter shows up as a plain ttyACM serial port;
#     needs `slcand` (userspace) to bridge it into SocketCAN as a `can*`
#     netdevice.
#
# The CANable2 actually plugged into this host (16d0:117e) was confirmed
# on 2026-08-04 to be running slcan/CDC-ACM firmware (its USB interfaces
# bind to the cdc_acm kernel driver, not gs_usb) -- so the slcan path is
# the one that matters today. The gs_usb path is kept for when/if this or
# another adapter gets reflashed to candleLight firmware.
#
# slcand is NOT launched directly from a udev RUN key. Confirmed via
# `make debug-udev-can` (2026-08-04): a real hotplug generates more than
# one uevent for the same tty (this box also runs ModemManager for a WWAN
# modem, which -- even with ID_MM_DEVICE_IGNORE below -- still touches new
# tty devices briefly), and systemd-udevd kills any RUN command still
# running when a later uevent for the same device supersedes it
# ("terminated by signal TERM" / "Input/output error" in the debug log).
# RUN+= is only meant for quick synchronous actions; a daemon has to be
# handed off to systemd proper so it's no longer tracked by the udev
# worker. Instead the udev rule below just tags the device and asks
# systemd to start slcand@<tty>.service, which also means BindsTo=
# handles stopping slcand automatically on unplug -- no more leaked
# processes across replugs, which a plain RUN+= approach can't do either.
#
# Idempotent -- safe to rerun.
#
# Must be run as root (see `make setup-socketcan` in the top-level
# Makefile, which invokes this via sudo). No sudo calls in here on
# purpose, so it also works unmodified inside a root-only context (e.g. a
# container) that has no sudo binary at all.

set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "setup-socketcan.sh must be run as root (try: make setup-socketcan)" >&2
    exit 1
fi

CAN_BITRATE=500000
# slcand encodes speed as an index (0..8), see `slcand -h`: s6 = 500 kbit/s.
CAN_BITRATE_SLCAN_INDEX=6

cat > /etc/modules-load.d/can.conf <<'EOF'
# gs_usb: for candleLight-firmware adapters (native CAN netdev). CANable2's
# 16d0:117e PID isn't in this kernel's gs_usb.ko modalias table (only
# 16d0:0f30, 16d0:10b8, 1cd2:606f, 1209:2323, 1d50:606f are), so hotplug
# autoload never fires for it even when it IS running gs_usb firmware --
# load unconditionally instead.
gs_usb
# slcan: line discipline used by slcand to bridge a serial-firmware
# adapter into SocketCAN. Normally autoloads when slcand attaches, but
# preloading is cheap insurance.
slcan
EOF

cat > /etc/systemd/system/slcand@.service <<EOF
[Unit]
Description=slcand SocketCAN bridge for %i
After=dev-%i.device
BindsTo=dev-%i.device

[Service]
Type=forking
ExecStart=/usr/bin/slcand -o -c -s${CAN_BITRATE_SLCAN_INDEX} /dev/%i can0
# Belt-and-suspenders: confirmed 2026-08-04 that slcand's own -o (open)
# command can silently lose the race when it's launched this soon after
# hotplug (attaching right as the port enumerates, vs. several seconds of
# natural delay in a by-hand test) -- leaving can0 registered but DOWN.
# Bring it up explicitly too; "-" ignores a harmless already-up failure.
ExecStartPost=-/usr/bin/ip link set can0 up type can bitrate ${CAN_BITRATE}
EOF

cat > /etc/udev/rules.d/80-canbus.rules <<EOF
# Keep ModemManager off this adapter. This box's WWAN modem needs MM
# active, but MM's own 80-mm-candidate.rules auto-probes every new tty --
# confirmed by ID_MM_CANDIDATE=1 showing up on the CANable2's port.
# This rule's filename ("80-canbus" < "80-mm-candidate" alphabetically)
# sorts before MM's, so the ENV assignment lands before that rule's own
# ignore-check runs.
SUBSYSTEM=="tty", ATTRS{idVendor}=="16d0", ATTRS{idProduct}=="117e", ENV{ID_MM_DEVICE_IGNORE}="1"

# candleLight/gs_usb firmware: kernel creates the can* netdev directly,
# just bring it up. Scoped to DRIVERS=="gs_usb" so this doesn't also fire
# (and fail, since it's already up) on the can* netdevice slcan.ko creates
# below for slcan-firmware adapters -- that one also matches KERNEL=="can*"
# but has no gs_usb ancestry.
ACTION=="add", SUBSYSTEM=="net", KERNEL=="can*", DRIVERS=="gs_usb", RUN+="/usr/bin/ip link set %k up type can bitrate ${CAN_BITRATE}"

# slcan firmware (this CANable2, confirmed 2026-08-04): shows up as a tty,
# not a can* netdev. Hand off to slcand@.service (see above) instead of
# RUN+= -- see the file header for why a direct RUN+= daemon spawn doesn't
# survive real hotplug here.
ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="16d0", ATTRS{idProduct}=="117e", TAG+="systemd", ENV{SYSTEMD_WANTS}="slcand@%k.service"
EOF

cat > /etc/NetworkManager/conf.d/85-can-unmanaged.conf <<'EOF'
# Keep NetworkManager off any CAN interface, gs_usb-native or
# slcand-bridged -- NM has no CAN-aware connection type and would
# otherwise fight the udev rules / candump.
[keyfile]
unmanaged-devices=driver:gs_usb;driver:slcan
EOF

modprobe gs_usb || true
modprobe slcan || true
systemctl daemon-reload
# daemon-reload alone doesn't restart an already-running instance -- if
# the adapter was already plugged in from a previous run, its
# slcand@<tty>.service is still running the old unit definition. Restart
# any active instances so a rerun picks up service-file changes without
# requiring a physical replug.
systemctl restart 'slcand@*.service' 2>/dev/null || true
udevadm control --reload-rules
systemctl reload NetworkManager
# trigger the new rules against whatever's already plugged in
udevadm trigger --subsystem-match=net --action=add
udevadm trigger --subsystem-match=tty --action=add

sleep 1
ip -details link show type can
