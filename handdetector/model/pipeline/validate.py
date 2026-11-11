"""Stage 6: compare every step of the pipeline on the same images.

Runs the zoo ONNX (192 only), the surgered ONNX, TFLite float32 and TFLite
int8 on the zoo palm photos plus a few no-hand images, decodes with
common.decode(), and reports per image: best score, IoU of the best box
with the ground truth, and number of detections. Writes
build/rRES/report.md.

Also writes golden vectors for checking the firmware against the host:
golden_input.bin (int8 NHWC input tensor for palm1) and
golden_scores.bin / golden_boxes.bin (the int8 interpreter's raw outputs),
packed together as golden.bin for `make flash-golden` + `det golden`.
"""
import numpy as np

from common import OUT, ORIGINAL_ONNX, RES, decode, iou, letterbox, load_rgb, num_anchors, palm_samples


class OnnxEngine:
    def __init__(self, path, nhwc_input=False):
        import onnxruntime as ort
        so = ort.SessionOptions()
        so.log_severity_level = 3
        self.s = ort.InferenceSession(str(path), so)
        self.name = self.s.get_inputs()[0].name
        self.nhwc = nhwc_input

    def __call__(self, x):
        inp = x[None] if self.nhwc else x.transpose(2, 0, 1)[None]
        boxes, scores = self.s.run(None, {self.name: inp})
        return boxes[0, :, :4], scores[0, :, 0]


class TFLiteEngine:
    def __init__(self, path, reference=False):
        """reference=True: LiteRT's reference kernels (BUILTIN_REF), the
        same integer arithmetic as TFLM's reference kernels. The default
        resolver uses optimized kernels + XNNPACK, whose requantization
        rounds differently -- fine for accuracy, not for bit-exact checks."""
        from ai_edge_litert.interpreter import Interpreter, OpResolverType
        kw = dict(experimental_op_resolver_type=OpResolverType.BUILTIN_REF) if reference else {}
        self.it = Interpreter(model_path=str(path), **kw)
        self.it.allocate_tensors()
        self.inp = self.it.get_input_details()[0]
        outs = self.it.get_output_details()
        # identify outputs by shape: [1, N, 4] boxes, [1, N, 1] scores
        self.boxes = next(o for o in outs if o["shape"][-1] == 4)
        self.scores = next(o for o in outs if o["shape"][-1] == 1)

    def quantize_input(self, x):
        scale, zp = self.inp["quantization"]
        if self.inp["dtype"] == np.float32:
            return x[None].astype(np.float32)
        q = np.round(x / scale) + zp
        return np.clip(q, -128, 127).astype(self.inp["dtype"])[None]

    def raw(self, x):
        self.it.set_tensor(self.inp["index"], self.quantize_input(x))
        self.it.invoke()
        return self.it.get_tensor(self.boxes["index"]), self.it.get_tensor(self.scores["index"])

    def __call__(self, x):
        b, s = self.raw(x)
        return dequant(b, self.boxes)[0], dequant(s, self.scores)[0, :, 0]


def dequant(t, detail):
    scale, zp = detail["quantization"]
    if t.dtype == np.float32:
        return t
    return (t.astype(np.float32) - zp) * scale


def negatives():
    """Images with no hand: flat black/grey, noise, and a horizontal
    gradient. A detector that fires here is miscalibrated."""
    rng = np.random.default_rng(1)
    yield "black", np.zeros((240, 320, 3), np.uint8)
    yield "grey", np.full((240, 320, 3), 128, np.uint8)
    yield "noise", rng.integers(0, 256, (240, 320, 3), dtype=np.uint8)
    yield "gradient", np.tile(np.linspace(0, 255, 320, dtype=np.uint8)[None, :, None], (240, 1, 3))


def main():
    engines = {}
    if RES == 192:
        engines["zoo ONNX"] = OnnxEngine(ORIGINAL_ONNX, nhwc_input=True)
    engines["surgered ONNX"] = OnnxEngine(OUT / "palm_det_surgered.onnx")
    engines["TFLite f32"] = TFLiteEngine(OUT / "model_f32.tflite")
    int8 = engines["TFLite int8"] = TFLiteEngine(OUT / "model_int8.tflite")

    rows = []
    for jpg, gt in palm_samples():
        rgb = load_rgb(jpg)
        x, lb = letterbox(rgb, RES)
        cells = []
        for eng in engines.values():
            dets = decode(*eng(x), lb, RES)
            cells.append(f"{dets[0][0]:.3f} / IoU {iou(dets[0][1:], gt):.2f} / {len(dets)}" if dets else "**none**")
        rows.append((jpg.name, cells))
    for name, rgb in negatives():
        x, lb = letterbox(rgb, RES)
        cells = []
        for eng in engines.values():
            dets = decode(*eng(x), lb, RES, score_thr=0.0)
            cells.append(f"max {dets[0][0]:.3f}" if dets else "-")
        rows.append((f"(no hand) {name}", cells))

    # Golden vectors from palm1 for the on-device check, computed with
    # reference kernels so TFLM on the device should match bit for bit.
    jpg, _ = next(iter(palm_samples()))
    x, _ = letterbox(load_rgb(jpg), RES)
    ref = TFLiteEngine(OUT / "model_int8.tflite", reference=True)
    q = ref.quantize_input(x)
    b, s = ref.raw(x)
    bx, sx = int8.raw(x)
    xnn_vs_ref = max(int(np.abs(bx.astype(int) - b).max()), int(np.abs(sx.astype(int) - s).max()))
    q.tofile(OUT / "golden_input.bin")
    b.tofile(OUT / "golden_boxes.bin")
    s.tofile(OUT / "golden_scores.bin")
    # One blob for the device (`make flash-golden`, `det golden`):
    # "GOLD", u32 input/boxes/scores lengths (LE), then the three tensors.
    import struct
    blob = b"GOLD" + struct.pack("<III", q.nbytes, b.nbytes, s.nbytes) + q.tobytes() + b.tobytes() + s.tobytes()
    (OUT / "golden.bin").write_bytes(blob)

    iq, bq, sq = int8.inp["quantization"], int8.boxes["quantization"], int8.scores["quantization"]
    lines = [
        f"# Palm detector at {RES}x{RES}", "",
        f"{num_anchors(RES)} anchors. Cells: best score / IoU of best box with ground truth / detections "
        "after threshold 0.5 + NMS 0.3. No-hand rows: highest score anywhere.", "",
        "| image | " + " | ".join(engines) + " |",
        "|---|" + "---|" * len(engines),
    ]
    for name, cells in rows:
        lines.append(f"| {name} | " + " | ".join(cells) + " |")
    lines += [
        "",
        "int8 tensors (scale, zero point):",
        f"- input {tuple(int8.inp['shape'])} {iq}",
        f"- boxes {tuple(int8.boxes['shape'])} {bq}",
        f"- scores {tuple(int8.scores['shape'])} {sq}",
        "",
        f"Host int8, optimized/XNNPACK vs reference kernels on palm1: max |diff| {xnn_vs_ref} LSB "
        "(golden vectors use the reference kernels).",
        "",
    ]
    report = "\n".join(lines)
    (OUT / "report.md").write_text(report)
    print(report)


if __name__ == "__main__":
    main()
