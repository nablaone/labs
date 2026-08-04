.PHONY: setup-socketcan debug-udev-can

# Persistent SocketCAN setup for this host's gs_usb-backed adapter
# (CANable2). Script itself has no sudo calls -- this target is the one
# place that elevates, so the script stays usable as-is in any root-only
# context too.
setup-socketcan:
	sudo ./scripts/setup-socketcan.sh

# Capture systemd-udevd's debug log around a CAN adapter hotplug --
# diagnoses a udev RUN rule (e.g. the slcand one above) that matches under
# `udevadm test` but doesn't actually fire on a real hotplug.
# CAPTURE_SECONDS overrides the default 15s window, e.g.:
#   make debug-udev-can CAPTURE_SECONDS=30
debug-udev-can:
	sudo ./scripts/debug-udev-can.sh $(CAPTURE_SECONDS)
