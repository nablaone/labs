"""Stage 3: graph surgery on the ONNX model.

Two edits, neither changes the math:

1. PRelu slopes (1, C, 1, 1) -> (C, 1, 1).
   Both shapes broadcast the same way, so ONNX Runtime doesn't care. But
   onnx2torch (next stage) only maps PRelu to torch's native prelu when
   the slope is (C,) or (C, 1, 1). For (1, C, 1, 1) it falls back to
   `out[mask] = x[mask]` (boolean-mask indexing), which Core ML lowers
   to ~10 CPU-only ops (non_zero, gather_nd, scatter_nd, ...) per PRelu.
   That cuts the graph into 26 CPU islands and nothing runs on the
   Neural Engine. See README, "The PRelu trap".

2. Rename the tensors tf2onnx left behind: input_1 -> input,
   Identity -> boxes, Identity_1 -> scores.

Checked with ONNX Runtime: the output must be bit-identical.
"""
import sys

import numpy as np
import onnx
import onnxruntime as ort
from onnx import numpy_helper

from common import ORIGINAL_ONNX, SURGERED_ONNX, letterbox, palm_samples, load_rgb

RENAMES = {"input_1": "input", "Identity": "boxes", "Identity_1": "scores"}


def reshape_prelu_slopes(g):
    inits = {i.name: i for i in g.initializer}
    users = {}
    for n in g.node:
        for name in n.input:
            users[name] = users.get(name, 0) + 1
    done = 0
    for n in g.node:
        if n.op_type != "PRelu":
            continue
        init = inits[n.input[1]]
        assert users[init.name] == 1, f"slope {init.name} is shared, copy it first"
        a = numpy_helper.to_array(init)
        if a.ndim == 4 and a.shape[0] == 1:
            init.CopyFrom(numpy_helper.from_array(a.reshape(a.shape[1:]), init.name))
            done += 1
    return done


def rename(g, old, new):
    for n in g.node:
        n.input[:] = [new if x == old else x for x in n.input]
        n.output[:] = [new if x == old else x for x in n.output]
    for v in [*g.input, *g.output, *g.value_info]:
        if v.name == old:
            v.name = new


def main():
    model = onnx.load(ORIGINAL_ONNX)
    g = model.graph
    print(f"PRelu slopes reshaped: {reshape_prelu_slopes(g)}")
    for old, new in RENAMES.items():
        rename(g, old, new)
    onnx.checker.check_model(model)
    onnx.save(model, SURGERED_ONNX)
    print(f"wrote {SURGERED_ONNX.relative_to(SURGERED_ONNX.parent.parent)}")

    a = ort.InferenceSession(ORIGINAL_ONNX, providers=["CPUExecutionProvider"])
    b = ort.InferenceSession(SURGERED_ONNX, providers=["CPUExecutionProvider"])
    worst = 0.0
    for jpg, _ in palm_samples():
        x, _ = letterbox(load_rgb(jpg))
        ra = a.run(None, {"input_1": x})
        rb = b.run(["boxes", "scores"], {"input": x})
        worst = max(worst, *(float(np.abs(p - q).max()) for p, q in zip(ra, rb)))
    print(f"ONNX Runtime, original vs surgered, max abs diff: {worst}")
    sys.exit(0 if worst == 0.0 else 1)


if __name__ == "__main__":
    main()
