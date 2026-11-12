"""Stage 2: look inside the ONNX model before running it.

Prints and writes build/inspect.md: inputs/outputs, operator counts,
parameters, multiply-accumulates of the convolutions, the largest
activations, and checks our anchor generation against the zoo's table.
"""
import collections
import importlib.util
import sys

import numpy as np
import onnx
from onnx import numpy_helper, shape_inference

from common import ANCHORS, BUILD, FETCH, ORIGINAL_ONNX, md_table


def dims(value_info):
    return [d.dim_value or d.dim_param for d in value_info.type.tensor_type.shape.dim]


def main(path=ORIGINAL_ONNX):
    model = shape_inference.infer_shapes(onnx.load(path))
    g = model.graph
    shapes = {v.name: dims(v) for v in [*g.value_info, *g.input, *g.output]}
    inits = {i.name: numpy_helper.to_array(i) for i in g.initializer}

    ops = collections.Counter(n.op_type for n in g.node)
    params = sum(a.size for a in inits.values())

    # Conv MACs = output elements * (kernel h * kernel w * input channels / group)
    macs = 0
    for n in g.node:
        if n.op_type != "Conv":
            continue
        w = inits[n.input[1]]                       # (OC, IC/group, KH, KW)
        out = shapes[n.output[0]]                   # (1, OC, H, W)
        macs += int(np.prod(out)) * int(np.prod(w.shape[1:]))

    acts = sorted(((int(np.prod(s)), name, n.op_type, s)
                   for n in g.node for name in n.output
                   if (s := shapes.get(name)) and all(isinstance(d, int) for d in s)),
                  reverse=True)[:5]

    lines = [f"# {path.name}", ""]
    lines += [md_table(["", ""], [
        ["opsets", ", ".join(f"{o.domain or 'ai.onnx'} {o.version}" for o in model.opset_import)],
        *[[f"input `{i.name}`", dims(i)] for i in g.input],
        *[[f"output `{o.name}`", dims(o)] for o in g.output],
        ["parameters", f"{params / 1e6:.3f} M ({params * 4 / 1e6:.2f} MB float32)"],
        ["conv MACs", f"{macs / 1e6:.1f} M"],
        ["nodes", len(g.node)],
        ["operators", ", ".join(f"{k} x{v}" for k, v in ops.most_common())],
    ]), ""]
    lines += ["Largest activations (float32 elements):", ""]
    lines += [f"- {n} ({op} {s})" for n, _, op, s in acts]
    lines += [""]

    # PRelu slope shapes matter for the next stage (surgery).
    slopes = collections.Counter(str(inits[n.input[1]].shape)
                                 for n in g.node if n.op_type == "PRelu")
    lines += ["PRelu slope shapes: " + ", ".join(f"{k} x{v}" for k, v in slopes.items()), ""]

    # Anchors: ours vs the hard-coded table in the zoo's reference wrapper.
    spec = importlib.util.spec_from_file_location("mp_palmdet", FETCH / "mp_palmdet.py")
    zoo = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(zoo)
    zoo_anchors = zoo.MPPalmDet._load_anchors(None)
    diff = float(np.abs(zoo_anchors - ANCHORS).max())
    lines += [f"Anchors: {len(ANCHORS)} generated, zoo table {zoo_anchors.shape}, "
              f"max |diff| {diff:.2e}", ""]

    text = "\n".join(lines)
    print(text)
    BUILD.mkdir(exist_ok=True)
    (BUILD / "inspect.md").write_text(text)
    sys.exit(0 if diff < 1e-6 else 1)


if __name__ == "__main__":
    main()
