#!/usr/bin/env python3
"""
cantool.py -- sniff/send on a CANable-style dongle via python-can's slcan
backend.

macOS-only for now (no SocketCAN there, so this talks to the dongle
directly over its slcan/CDC-ACM serial port instead) -- Linux already has
the real thing (candump/cansend against SocketCAN, see
../../scripts/setup-socketcan.sh), so no point duplicating that path here.

Same <id_hex>#<data_hex> frame syntax as the esp32-idf app's "can sniff"/
"can send" CLI commands (main/main.c), so a frame copy-pastes cleanly
between the two.

Usage:
    cantool.py sniff                     # print received frames until Ctrl-C
    cantool.py send <id_hex>#<data_hex>  # e.g. cantool.py send 123#DEADBEEF

Port auto-detected (first /dev/cu.usbmodem*); override with the CAN_PORT
env var if more than one dongle is attached. Bitrate fixed at 500000 to
match the ESP32 side and scripts/setup-socketcan.sh.
"""
import glob
import os
import sys

import can

BITRATE = 500000


def find_port():
    port = os.environ.get("CAN_PORT")
    if port:
        return port
    matches = sorted(glob.glob("/dev/cu.usbmodem*"))
    if not matches:
        sys.exit(
            "No CAN dongle found (no /dev/cu.usbmodem* device) -- is it "
            "plugged in? Override with CAN_PORT=/dev/whatever if needed."
        )
    return matches[0]


def open_bus():
    return can.interface.Bus(interface="slcan", channel=find_port(), bitrate=BITRATE)


def parse_frame(frame_str):
    if "#" not in frame_str:
        sys.exit(f"bad frame '{frame_str}', expected <id_hex>#<data_hex>, e.g. 123#DEADBEEF")
    id_str, data_str = frame_str.split("#", 1)
    return int(id_str, 16), bytes.fromhex(data_str)


def cmd_sniff():
    bus = None
    try:
        # open_bus() includes slcan's own brief post-open settle delay --
        # a Ctrl-C landing during that (not just the receive loop below)
        # should still exit cleanly, so it's inside this try too.
        bus = open_bus()
        # flush=True: stdout is only line-buffered when connected to a
        # real terminal -- piped/redirected/backgrounded (logging, `| tee`,
        # this kind of scripted test) it's fully buffered by default, so
        # frames wouldn't show up until the process exits without this.
        print(f"Listening on {bus.channel_info} -- Ctrl-C to stop...", flush=True)
        while True:
            msg = bus.recv(timeout=0.5)
            if msg is not None:
                print(f"{msg.arbitration_id:03X}#{msg.data.hex().upper()}", flush=True)
    except KeyboardInterrupt:
        print()
    finally:
        if bus is not None:
            bus.shutdown()


def cmd_send(frame_str):
    can_id, data = parse_frame(frame_str)
    bus = open_bus()
    try:
        bus.send(can.Message(arbitration_id=can_id, data=data, is_extended_id=False))
    finally:
        bus.shutdown()
    print(f"sent {can_id:03X}#{data.hex().upper()}")


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]
    if cmd == "sniff":
        cmd_sniff()
    elif cmd == "send":
        if len(sys.argv) < 3:
            sys.exit("usage: cantool.py send <id_hex>#<data_hex>")
        cmd_send(sys.argv[2])
    else:
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
