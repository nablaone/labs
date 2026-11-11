"""Shared bits of the model pipeline: paths, anchors, pre/post-processing.

The pre/post-processing here is the reference the firmware's C code is
checked against, so it is written out explicitly rather than borrowed
from OpenCV's demo. It follows the OpenCV zoo's mp_palmdet.py
(letterbox, /255, SSD anchor decode, sigmoid, NMS).
"""
import os
import pathlib

import numpy as np

ROOT = pathlib.Path(__file__).resolve().parent.parent  # model/
BUILD = ROOT / "build"
FETCH = BUILD / "fetch"
DATA = ROOT / "data"

# Model input size (square). The zoo model is 192; surgery makes the graph
# resolution-independent, so smaller sizes are a pipeline parameter.
RES = int(os.environ.get("RES", "96"))
OUT = BUILD / f"r{RES}"

ORIGINAL_ONNX = FETCH / "palm_detection_mediapipe_2023feb.onnx"
PALM_DIR = FETCH / "palm_detection_20230125"

# MediaPipe palm detector SSD config: 4 output layers with strides
# 8, 16, 16, 16 and 2 anchors per cell each; layers that share a stride are
# merged, so stride 8 -> 2 anchors/cell, stride 16 -> 6 anchors/cell.
# fixed_anchor_size: every anchor is 1x1, only the centre matters.
ANCHOR_LAYOUT = ((8, 2), (16, 6))


def anchors(res=RES):
    """(N, 2) anchor centres in [0, 1], in the model's output order:
    stride-8 grid first, row-major (y outer, x inner), then stride 16."""
    out = []
    for stride, per_cell in ANCHOR_LAYOUT:
        f = res // stride
        for y in range(f):
            for x in range(f):
                out.extend([((x + 0.5) / f, (y + 0.5) / f)] * per_cell)
    return np.asarray(out, dtype=np.float32)


def num_anchors(res=RES):
    return sum((res // s) ** 2 * k for s, k in ANCHOR_LAYOUT)


def load_rgb(path):
    import cv2
    img = cv2.imread(str(path), cv2.IMREAD_COLOR)
    if img is None:
        raise FileNotFoundError(path)
    return cv2.cvtColor(img, cv2.COLOR_BGR2RGB)


def letterbox(rgb, res=RES):
    """Keep aspect ratio, scale the long side to `res`, pad with black,
    centred (the odd pixel goes right/bottom). Returns float32 HWC in
    [0, 1] plus what decode() needs to map back to source pixels."""
    import cv2
    h, w = rgb.shape[:2]
    ratio = min(res / w, res / h)
    nw, nh = int(round(w * ratio)), int(round(h * ratio))
    resized = cv2.resize(rgb, (nw, nh), interpolation=cv2.INTER_LINEAR)
    left, top = (res - nw) // 2, (res - nh) // 2
    canvas = np.zeros((res, res, 3), dtype=np.uint8)
    canvas[top:top + nh, left:left + nw] = resized
    return canvas.astype(np.float32) / 255.0, dict(ratio=ratio, left=left, top=top)


def sigmoid(x):
    return 1.0 / (1.0 + np.exp(-x.astype(np.float64)))


def decode(boxes, scores, lb, res=RES, score_thr=0.5, nms_thr=0.3):
    """boxes (N, 4) raw regressor output [dcx, dcy, w, h] in input pixels,
    scores (N,) logits. Returns a list of (score, x1, y1, x2, y2) in
    source-image pixels after threshold + NMS, best first."""
    a = anchors(res)
    p = sigmoid(scores)
    keep = np.where(p >= score_thr)[0]
    if keep.size == 0:
        return []
    b = boxes[keep].astype(np.float64) / res
    cxy = b[:, 0:2] + a[keep]
    wh = b[:, 2:4]
    xy1 = (cxy - wh / 2) * res
    xy2 = (cxy + wh / 2) * res
    # letterbox space -> source pixels
    off = np.array([lb["left"], lb["top"]])
    xy1 = (xy1 - off) / lb["ratio"]
    xy2 = (xy2 - off) / lb["ratio"]
    dets = np.concatenate([p[keep, None], xy1, xy2], axis=1)
    return nms(dets, nms_thr)


def iou(a, b):
    ix1, iy1 = max(a[0], b[0]), max(a[1], b[1])
    ix2, iy2 = min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix2 - ix1) * max(0.0, iy2 - iy1)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def nms(dets, thr):
    order = np.argsort(-dets[:, 0])
    kept = []
    for i in order:
        if all(iou(dets[i, 1:], dets[j, 1:]) <= thr for j in kept):
            kept.append(i)
    return [tuple(float(v) for v in dets[i]) for i in kept]


def palm_samples():
    """Zoo benchmark images with their ground-truth box (x1 y1 x2 y2)."""
    for jpg in sorted(PALM_DIR.glob("palm*.jpg")):
        gt = [float(v) for v in jpg.with_suffix(".txt").read_text().split()]
        yield jpg, gt[0:4]
