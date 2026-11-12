"""Shared bits: paths, anchors, pre-processing, decoding, drawing.

Everything model-specific that is *not* inside the network lives here,
written out explicitly (no OpenCV demo code), so it can be read top to
bottom. It follows the OpenCV zoo's reference wrapper mp_palmdet.py:
letterbox -> /255 -> network -> SSD anchor decode -> sigmoid -> NMS.
"""
import pathlib

import numpy as np

ROOT = pathlib.Path(__file__).resolve().parent.parent
BUILD = ROOT / "build"
FETCH = BUILD / "fetch"

ORIGINAL_ONNX = FETCH / "palm_detection_mediapipe_2023feb.onnx"
SURGERED_ONNX = BUILD / "palm_surgered.onnx"
COREML_FP16 = BUILD / "palm_fp16.mlpackage"
COREML_FP32 = BUILD / "palm_fp32.mlpackage"
PALM_DIR = FETCH / "palm_detection_20230125"

# The zoo model has a fixed input: 1 x 192 x 192 x 3, NHWC, RGB in [0, 1].
RES = 192

# Output row layout of the 18-wide `boxes` tensor, all in input pixels,
# relative to the anchor centre:
#   [0:2]  box centre offset (dx, dy)
#   [2:4]  box size (w, h)
#   [4:18] 7 keypoints (x, y): wrist, then the base of each finger
NUM_KEYPOINTS = 7

# MediaPipe palm detector SSD config: 4 output layers with strides
# 8, 16, 16, 16 and 2 anchors per cell each. Layers sharing a stride are
# merged, so stride 8 -> 2 anchors/cell, stride 16 -> 6 anchors/cell.
# fixed_anchor_size: every anchor is 1x1, only its centre matters.
ANCHOR_LAYOUT = ((8, 2), (16, 6))


def anchors(res=RES):
    """(N, 2) anchor centres in [0, 1], in the model's output order:
    stride-8 grid first, row-major (y outer, x inner), then stride 16.
    At 192: 24*24*2 + 12*12*6 = 2016 anchors."""
    out = []
    for stride, per_cell in ANCHOR_LAYOUT:
        f = res // stride
        for y in range(f):
            for x in range(f):
                out.extend([((x + 0.5) / f, (y + 0.5) / f)] * per_cell)
    return np.asarray(out, dtype=np.float32)


ANCHORS = anchors()


def letterbox(rgb, res=RES):
    """Keep the aspect ratio, scale the long side to `res`, pad with black,
    centred. Returns the network input (1, res, res, 3) float32 in [0, 1]
    plus what decode() needs to map boxes back to source pixels."""
    import cv2
    h, w = rgb.shape[:2]
    ratio = min(res / w, res / h)
    nw, nh = int(round(w * ratio)), int(round(h * ratio))
    resized = cv2.resize(rgb, (nw, nh), interpolation=cv2.INTER_LINEAR)
    left, top = (res - nw) // 2, (res - nh) // 2
    canvas = np.zeros((res, res, 3), dtype=np.uint8)
    canvas[top:top + nh, left:left + nw] = resized
    x = canvas.astype(np.float32)[None] / 255.0
    return x, dict(ratio=ratio, left=left, top=top)


def sigmoid(x):
    return 1.0 / (1.0 + np.exp(-x.astype(np.float64)))


def decode(boxes, scores, lb, score_thr=0.5, nms_thr=0.3):
    """Raw network outputs -> detections in source-image pixels.

    boxes (2016, 18), scores (2016,) logits. Returns a list of dicts
    {score, box: (x1, y1, x2, y2), keypoints: (7, 2)}, best first."""
    p = sigmoid(scores)
    keep = np.where(p >= score_thr)[0]
    if keep.size == 0:
        return []
    raw = boxes[keep].astype(np.float64) / RES          # input pixels -> [0, 1]
    centre = ANCHORS[keep]                              # (k, 2)
    cxy = raw[:, 0:2] + centre
    wh = raw[:, 2:4]
    kps = raw[:, 4:].reshape(-1, NUM_KEYPOINTS, 2) + centre[:, None, :]

    def to_source(xy):                                  # [0,1] letterbox -> pixels
        return (xy * RES - [lb["left"], lb["top"]]) / lb["ratio"]

    xy1, xy2 = to_source(cxy - wh / 2), to_source(cxy + wh / 2)
    kps = to_source(kps)
    order = nms(np.concatenate([xy1, xy2], axis=1), p[keep], nms_thr)
    return [dict(score=float(p[keep][i]), box=tuple(float(v) for v in (*xy1[i], *xy2[i])),
                 keypoints=kps[i]) for i in order]


def iou(a, b):
    ix1, iy1 = max(a[0], b[0]), max(a[1], b[1])
    ix2, iy2 = min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix2 - ix1) * max(0.0, iy2 - iy1)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def nms(boxes, scores, thr):
    """Greedy non-maximum suppression; returns kept indices, best first."""
    kept = []
    for i in np.argsort(-scores):
        if all(iou(boxes[i], boxes[j]) <= thr for j in kept):
            kept.append(i)
    return kept


def draw(bgr, dets, colour=(0, 255, 0)):
    import cv2
    for d in dets:
        x1, y1, x2, y2 = (int(round(v)) for v in d["box"])
        cv2.rectangle(bgr, (x1, y1), (x2, y2), colour, 2)
        cv2.putText(bgr, f"{d['score']:.2f}", (x1, max(0, y1 - 6)),
                    cv2.FONT_HERSHEY_SIMPLEX, 0.6, colour, 2)
        for x, y in d["keypoints"]:
            cv2.circle(bgr, (int(round(x)), int(round(y))), 4, (0, 0, 255), -1)
    return bgr


def palm_samples():
    """Zoo benchmark images with their ground-truth box (x1 y1 x2 y2)."""
    for jpg in sorted(PALM_DIR.glob("palm*.jpg")):
        gt = [float(v) for v in jpg.with_suffix(".txt").read_text().split()]
        yield jpg, gt[0:4]


def md_table(header, rows):
    out = ["| " + " | ".join(header) + " |", "|" + "---|" * len(header)]
    out += ["| " + " | ".join(str(c) for c in r) + " |" for r in rows]
    return "\n".join(out)


def load_rgb(path):
    import cv2
    img = cv2.imread(str(path), cv2.IMREAD_COLOR)
    if img is None:
        raise FileNotFoundError(path)
    return cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
