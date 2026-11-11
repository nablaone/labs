"""Stage 3: model surgery on the zoo ONNX graph.

1. Input: the graph takes NHWC `input_1` and immediately Transposes it to
   NCHW (a tf2onnx artefact). Replace that with a plain NCHW input
   `input` [1, 3, RES, RES] -- onnx2tf assumes ONNX is NCHW and gives the
   TFLite model a clean NHWC input from it.
2. Resize: the two FPN upsamplings carry hard-coded target `sizes`
   (12x12, 24x24 -- only valid at 192). Swap them for `scales` [1,1,2,2]
   so the graph works at any input size.
3. Heads: each regressor conv emits A x 18 values per cell (box 4 + 7
   palm keypoints x 2). Only the box is needed, so slice the conv
   weights/bias down to A x 4 and reshape to [1, -1, 4]. Less compute in
   the heads and a 4.5x smaller output tensor.
4. Name outputs `boxes` [1, N, 4] and `scores` [1, N, 1] (logits), fold
   constants, run onnx-simplifier.

Verified against the original with ONNX Runtime: at RES=192 boxes and
scores must match the original's [:, :, 0:4] / scores.
"""
import sys

import numpy as np
import onnx
from onnx import helper, numpy_helper, shape_inference

from common import ORIGINAL_ONNX, OUT, RES, letterbox, load_rgb, num_anchors, palm_samples


def consumers(g, name):
    return [n for n in g.node if name in n.input]


def producer(g, name):
    return next((n for n in g.node if name in n.output), None)


def add_init(g, name, arr):
    g.initializer.append(numpy_helper.from_array(arr, name))
    return name


def remove_init(g, name):
    for i, t in enumerate(g.initializer):
        if t.name == name:
            del g.initializer[i]
            return


def operate(model):
    g = model.graph

    # 1. NHWC input + Transpose  ->  NCHW input
    old_in = g.input[0]
    tr = consumers(g, old_in.name)
    assert len(tr) == 1 and tr[0].op_type == "Transpose", "expected input Transpose"
    perm = list(helper.get_attribute_value(tr[0].attribute[0]))
    assert perm == [0, 3, 1, 2], perm
    for n in consumers(g, tr[0].output[0]):
        for k, name in enumerate(n.input):
            if name == tr[0].output[0]:
                n.input[k] = "input"
    g.node.remove(tr[0])
    g.input.remove(old_in)
    g.input.insert(0, helper.make_tensor_value_info("input", onnx.TensorProto.FLOAT, [1, 3, RES, RES]))

    # 2. Resize sizes -> scales
    scales = add_init(g, "upsample_x2_scales", np.array([1, 1, 2, 2], dtype=np.float32))
    for n in [n for n in g.node if n.op_type == "Resize"]:
        x = n.input[0]
        old_sizes = n.input[3] if len(n.input) > 3 else None
        del n.input[:]
        n.input.extend([x, "", scales])
        if old_sizes and not consumers(g, old_sizes):
            remove_init(g, old_sizes)

    # 3. Regressor heads: keep box (first 4 of every 18)
    init = {t.name: t for t in g.initializer}
    for rs in [n for n in g.node if n.op_type == "Reshape"]:
        shape = numpy_helper.to_array(init[rs.input[1]])
        if shape.tolist() != [1, -1, 18]:
            continue
        tr = producer(g, rs.input[0])
        conv = producer(g, tr.input[0])
        assert tr.op_type == "Transpose" and conv.op_type == "Conv"
        w = numpy_helper.to_array(init[conv.input[1]])
        b = numpy_helper.to_array(init[conv.input[2]])
        a = w.shape[0] // 18
        keep = [i * 18 + k for i in range(a) for k in range(4)]
        conv.input[1] = add_init(g, conv.input[1] + "_box", w[keep])
        conv.input[2] = add_init(g, conv.input[2] + "_box", b[keep])
        rs.input[1] = add_init(g, rs.name + "_shape_box", np.array([1, -1, 4], dtype=np.int64))
        print(f"  head {conv.name.split('/')[2]}: {w.shape[0]} -> {len(keep)} channels ({a} anchors/cell)")

    # 4. Outputs
    n_anchors = num_anchors(RES)
    rename = {g.output[0].name: "boxes", g.output[1].name: "scores"}
    for n in g.node:
        for k, name in enumerate(n.output):
            if name in rename:
                n.output[k] = rename[name]
    del g.output[:]
    g.output.extend([
        helper.make_tensor_value_info("boxes", onnx.TensorProto.FLOAT, [1, n_anchors, 4]),
        helper.make_tensor_value_info("scores", onnx.TensorProto.FLOAT, [1, n_anchors, 1]),
    ])

    # Drop unused initializers and stale shape info (it encodes 192).
    used = {i for n in g.node for i in n.input}
    for t in [t for t in g.initializer if t.name not in used]:
        g.initializer.remove(t)
    del g.value_info[:]
    return model


def simplify(model):
    import onnxsim
    model, ok = onnxsim.simplify(model)
    assert ok, "onnxsim check failed"
    return model


def verify(original_path, new_path):
    import onnxruntime as ort
    so = ort.SessionOptions()
    so.log_severity_level = 3
    new = ort.InferenceSession(str(new_path), so)
    worst = 0.0
    for jpg, _ in palm_samples():
        x, _ = letterbox(load_rgb(jpg), RES)
        boxes, scores = new.run(None, {"input": x.transpose(2, 0, 1)[None]})
        assert boxes.shape == (1, num_anchors(RES), 4), boxes.shape
        assert scores.shape == (1, num_anchors(RES), 1), scores.shape
        if RES == 192:
            orig = ort.InferenceSession(str(original_path), so)
            r, s = orig.run(None, {orig.get_inputs()[0].name: x[None]})
            worst = max(worst, float(np.abs(r[..., :4] - boxes).max()), float(np.abs(s - scores).max()))
    if RES == 192:
        print(f"  max |original - surgered| over test images: {worst:.2e}")
        assert worst < 1e-3, "surgery changed the model's output"
    else:
        print(f"  runs at {RES}x{RES}: {num_anchors(RES)} anchors (no reference at this size)")


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    model = onnx.load(str(ORIGINAL_ONNX))
    model = operate(model)
    model = shape_inference.infer_shapes(model)
    onnx.checker.check_model(model)
    model = simplify(model)
    dst = OUT / "palm_det_surgered.onnx"
    onnx.save(model, str(dst))
    print(f"  wrote {dst.relative_to(OUT.parent.parent)}")
    verify(ORIGINAL_ONNX, dst)


if __name__ == "__main__":
    sys.exit(main())
