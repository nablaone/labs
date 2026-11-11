"""Pull camera frames from the board over the USB console (`cam dump`)
into a directory, as JPEGs -- calibration data that looks like what the
detector will actually see. Runs on the host (needs only pyserial).

usage: capture.py PORT OUT_DIR [N] [INTERVAL_S]

Prints a countdown before each frame so you can move your hand around;
mix frames with and without hands.
"""
import base64
import pathlib
import re
import sys
import time

import serial

FRAME = re.compile(rb"-----BEGIN FRAME (\d+)x(\d+) (\w+) (\d+)-----\s*\n(.*?)-----END FRAME-----", re.S)


def command(s, cmd, timeout=15):
    s.reset_input_buffer()
    s.write(cmd.encode() + b"\r")
    buf, t = b"", time.time()
    while time.time() - t < timeout:
        buf += s.read(65536)
        if buf.rstrip().endswith(b"hand>") and len(buf) > len(cmd) + 8:
            break
    return buf


def main():
    port, out = sys.argv[1], pathlib.Path(sys.argv[2])
    n = int(sys.argv[3]) if len(sys.argv) > 3 else 20
    interval = float(sys.argv[4]) if len(sys.argv) > 4 else 1.5
    out.mkdir(parents=True, exist_ok=True)
    s = serial.Serial(port, 115200, timeout=0.1)
    s.write(b"\r")  # enter CLI mode (silences logs)
    time.sleep(0.5)
    s.read(65536)
    command(s, "cam fmt jpeg")
    command(s, "cam res qvga")  # what the detector sees by default
    stamp = time.strftime("%Y%m%d-%H%M%S")
    for i in range(n):
        time.sleep(interval)
        m = FRAME.search(command(s, "cam dump"))
        if not m or m.group(3) != b"jpeg":
            print(f"  frame {i}: no JPEG in reply, skipped")
            continue
        path = out / f"{stamp}-{i:03d}.jpg"
        path.write_bytes(base64.b64decode(b"".join(m.group(5).split())))
        print(f"  {path} ({m.group(1).decode()}x{m.group(2).decode()})")
    s.write(b"exit\r")


if __name__ == "__main__":
    main()
