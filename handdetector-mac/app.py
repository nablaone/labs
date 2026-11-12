"""Live palm detection on the Mac camera.

The same chain as pipeline/detect.py, once per camera frame, with the
time of every stage on screen. Switch engines while it runs to see the
hardware make the difference.

    uv run python app.py                    # default: coreml-ane
    uv run python app.py -e onnx-cpu
    uv run python app.py --source clip.mp4  # a video file instead of the camera

Keys: 1..7 switch engine (list on screen), m mirror on/off, q or Esc quit.

The first run asks for camera access for the terminal app (System
Settings -> Privacy & Security -> Camera).
"""
import argparse
import sys
import time

sys.path.insert(0, __import__("os").path.join(__import__("os").path.dirname(__file__), "pipeline"))

import cv2  # noqa: E402

from common import decode, draw, letterbox  # noqa: E402
from engines import ENGINES, make  # noqa: E402

HOTKEYS = ["coreml-ane", "coreml-gpu", "coreml-cpu", "onnx-cpu",
           "onnx-coreml-all", "torch-mps", "trap-ane"]


def overlay(bgr, lines):
    # Size the text from the frame height, so it reads the same on 720p and 4K.
    s = bgr.shape[0] / 720 * 1.1
    thick = max(2, round(2 * s))
    step = round(40 * s)
    y = step
    for text in lines:
        cv2.putText(bgr, text, (round(16 * s), y), cv2.FONT_HERSHEY_SIMPLEX, s,
                    (0, 0, 0), thick * 3, cv2.LINE_AA)
        cv2.putText(bgr, text, (round(16 * s), y), cv2.FONT_HERSHEY_SIMPLEX, s,
                    (255, 255, 255), thick, cv2.LINE_AA)
        y += step


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-e", "--engine", default="coreml-ane", choices=list(ENGINES))
    ap.add_argument("--camera", type=int, default=0)
    ap.add_argument("--source", help="video or image file instead of the camera")
    ap.add_argument("--frames", type=int, default=0, help="stop after N frames (0 = run until q)")
    ap.add_argument("--headless", action="store_true", help="no window, print timings")
    args = ap.parse_args()

    still = cv2.imread(args.source) if args.source else None    # an image: reuse it
    cap = (cv2.VideoCapture(args.source) if args.source
           else cv2.VideoCapture(args.camera, cv2.CAP_AVFOUNDATION))
    if still is None and not cap.isOpened():
        raise SystemExit("cannot open the camera (check System Settings -> Privacy -> Camera)")

    engines = {}

    def get(name):
        if name not in engines:
            print(f"loading {name} ...", flush=True)
            engines[name] = make(name)
        return engines[name]

    engine = get(args.engine)
    mirror = args.source is None
    fps, n = 0.0, 0
    while True:
        t0 = time.perf_counter()
        ok, bgr = (True, still.copy()) if still is not None else cap.read()
        if not ok:
            if args.source:                     # loop a file
                cap.set(cv2.CAP_PROP_POS_FRAMES, 0)
                ok, bgr = cap.read()
            if not ok:
                if n == 0:
                    raise SystemExit("the camera gives no frames: allow camera access for this "
                                     "terminal in System Settings -> Privacy & Security -> Camera")
                break
        if mirror:
            bgr = cv2.flip(bgr, 1)
        t1 = time.perf_counter()
        x, lb = letterbox(cv2.cvtColor(bgr, cv2.COLOR_BGR2RGB))   # 1. pre-process
        t2 = time.perf_counter()
        boxes, scores = engine.infer(x)                            # 2. network
        t3 = time.perf_counter()
        dets = decode(boxes, scores, lb)                           # 3. post-process
        t4 = time.perf_counter()

        ms = [(b - a) * 1e3 for a, b in ((t0, t1), (t1, t2), (t2, t3), (t3, t4))]
        fps = 0.9 * fps + 0.1 / max(t4 - t0, 1e-6) if n else 1 / max(t4 - t0, 1e-6)
        n += 1
        status = (f"{engine.name}: grab {ms[0]:.1f}  pre {ms[1]:.1f}  "
                  f"infer {ms[2]:.2f}  post {ms[3]:.1f} ms  ({fps:.0f} fps)")

        if args.headless:
            if n % 30 == 0 or n == args.frames:
                print(f"{status}  hands {len(dets)}", flush=True)
        else:
            draw(bgr, dets)
            overlay(bgr, [status, ENGINES[engine.name][0],
                          "  ".join(f"{i + 1}:{e}" for i, e in enumerate(HOTKEYS)),
                          "m: mirror   q: quit"])
            cv2.imshow("palm detector", bgr)
            key = cv2.waitKey(1) & 0xFF
            if key in (ord("q"), 27):
                break
            if key == ord("m"):
                mirror = not mirror
            if ord("1") <= key < ord("1") + len(HOTKEYS):
                engine = get(HOTKEYS[key - ord("1")])
        if args.frames and n >= args.frames:
            break
    cap.release()
    cv2.destroyAllWindows()


if __name__ == "__main__":
    main()
