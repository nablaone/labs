"""Stage 2: describe an ONNX model -- I/O, operators, parameters, MACs,
activation sizes. Writes a Markdown report next to the model.

usage: inspect_model.py MODEL.onnx [REPORT.md]
"""
import collections
import pathlib
import sys

import numpy as np
import onnx
from onnx import numpy_helper, shape_inference


def describe(path):
    m = shape_inference.infer_shapes(onnx.load(str(path)))
    g = m.graph
    init = {i.name: numpy_helper.to_array(i) for i in g.initializer}

    def dims(v):
        return [d.dim_value or d.dim_param for d in v.type.tensor_type.shape.dim]

    shapes = {v.name: dims(v) for v in list(g.value_info) + list(g.input) + list(g.output)}
    static = lambda s: s and all(isinstance(d, int) and d > 0 for d in s)

    macs = 0
    acts = []
    for n in g.node:
        if n.op_type == "Conv":
            w, out = init.get(n.input[1]), shapes.get(n.output[0])
            if w is not None and static(out):
                macs += int(np.prod(out)) * int(np.prod(w.shape[1:]))
        for o in n.output:
            if static(shapes.get(o)):
                acts.append((int(np.prod(shapes[o])), n.op_type, shapes[o]))
    acts.sort(key=lambda a: -a[0])

    lines = [f"# {path.name}", ""]
    lines += ["| | |", "|---|---|"]
    lines.append(f"| opsets | {', '.join(f'{o.domain or 'ai.onnx'} {o.version}' for o in m.opset_import)} |")
    for i in g.input:
        if i.name not in init:
            lines.append(f"| input `{i.name}` | {dims(i)} |")
    for o in g.output:
        lines.append(f"| output `{o.name}` | {dims(o)} |")
    params = sum(a.size for a in init.values() if a.dtype.kind == "f")
    lines.append(f"| parameters | {params / 1e6:.3f} M ({params * 4 / 1e6:.2f} MB float32, ~{params / 1e6:.2f} MB int8) |")
    lines.append(f"| conv MACs | {macs / 1e6:.1f} M |")
    lines.append(f"| nodes | {len(g.node)} |")
    ops = collections.Counter(n.op_type for n in g.node)
    lines.append(f"| operators | {', '.join(f'{k} x{v}' for k, v in ops.most_common())} |")
    lines += ["", "Largest activations (elements = bytes at int8):", ""]
    for size, op, s in acts[:5]:
        lines.append(f"- {size} ({op} {s})")
    return "\n".join(lines) + "\n"


def main():
    path = pathlib.Path(sys.argv[1])
    report = describe(path)
    print(report)
    if len(sys.argv) > 2:
        pathlib.Path(sys.argv[2]).write_text(report)


if __name__ == "__main__":
    main()
