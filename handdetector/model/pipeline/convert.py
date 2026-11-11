"""Stage 5: ONNX -> TFLite (float32 and full-integer int8) with onnx2tf.

onnx2tf rebuilds the graph in NHWC and writes TFLite directly
("flatbuffer_direct", its 2.x default): per-channel int8 weights,
per-tensor int8 activations calibrated on calib.npy, int8 input and
output tensors. Full integer is what TFLM's reference kernels run
without any float math.

Why this writer and not TensorFlow's converter (`--tflite_backend
tf_converter`, used before): the TF path turns every PRelu into
ABS/SUB/MUL/RELU/ADD wrapped in TRANSPOSEs (~300 ops instead of 124),
which was 40% of the on-device inference time. flatbuffer_direct emits
native PRELU and no TRANSPOSE, but its int8 output has two bugs that are
fixed here:

1. Depthwise conv biases (rank 1) get the weights' quantized_dimension
   (3), which LiteRT and TFLM reject. Fixed at the source by wrapping
   onnx2tf's bias quantizer (runtime patch, the installed package is
   untouched).
2. Ops that pass int8 values through unchanged (MAX_POOL_2D, RESHAPE,
   RESIZE_BILINEAR, CONCATENATION, PAD) get different quantization on
   input and output. Their kernels don't rescale, so the values would be
   silently misread (TFLM even refuses MAX_POOL/CONCAT at Prepare).
   Fixed afterwards in the flatbuffer: every group of tensors connected
   through such ops gets one shared scale/zero point covering all their
   ranges -- what TensorFlow's quantizer enforces too.

Writes build/rRES/model_f32.tflite and model_int8.tflite, and prints the
TFLite operator set (the kernels the firmware's op resolver needs).
"""
import dataclasses
import shutil
import sys

import numpy as np

from common import OUT

# ops whose int8 kernels copy/compare values without rescaling:
# input 0 and output 0 must share quantization (CONCATENATION: all inputs)
PASSTHROUGH = {"MAX_POOL_2D": [0], "RESHAPE": [0], "RESIZE_BILINEAR": [0], "PAD": [0], "CONCATENATION": None}


def ops_used(path):
    from ai_edge_litert.interpreter import Interpreter
    it = Interpreter(model_path=str(path))
    return sorted({op["op_name"] for op in it._get_ops_details()})


def run_onnx2tf(src, tmp):
    import onnx2tf
    import onnx2tf.tflite_builder.quantization as q

    orig = q._quantize_bias_tensor

    def bias_fixed(**kw):
        orig(**kw)
        t = kw["tensor"]
        if np.ndim(t.data) <= 1 and t.quantization.quantized_dimension != 0:
            t.quantization = dataclasses.replace(t.quantization, quantized_dimension=0)

    q._quantize_bias_tensor = bias_fixed
    argv = [
        "onnx2tf", "-i", str(src), "-o", str(tmp),
        "-oiqt",                     # also emit integer-quantized models
        "-qt", "per-channel",
        # calibration data is already in [0, 1]; mean 0 / std 1 = no extra normalisation
        "-cind", "input", str(OUT / "calib.npy"), "[[[[0.0,0.0,0.0]]]]", "[[[[1.0,1.0,1.0]]]]",
        "-iqd", "int8", "-oqd", "int8",
        "-n",                        # non-verbose
    ]
    print("  " + " ".join(argv) + "  (+ bias quantized_dimension fix)")
    sys.argv = argv
    try:
        onnx2tf.main()
    except Exception as e:  # noqa: BLE001
        # -oiqt also builds an int16-activation variant, which fails strict
        # validation; we only need the int8 one.
        if not sorted(tmp.glob("*_full_integer_quant.tflite")):
            raise
        print(f"  (ignored, int8 model was written: {type(e).__name__}: {str(e).splitlines()[0][:120]})")


def unify_passthrough_qparams(src, dst):
    """Give every group of tensors linked by pass-through ops one shared
    quantization (see module docstring, bug 2). Returns a report string."""
    import flatbuffers
    from tensorflow.lite.python import schema_py_generated as s

    m = s.ModelT.InitFromPackedBuf(bytearray(src.read_bytes()), 0)
    names = {v: k for k, v in vars(s.BuiltinOperator).items() if not k.startswith("_")}
    g = m.subgraphs[0]

    parent = list(range(len(g.tensors)))

    def find(i):
        while parent[i] != i:
            parent[i] = parent[parent[i]]
            i = parent[i]
        return i

    for op in g.operators:
        code = m.operatorCodes[op.opcodeIndex]
        name = names.get(max(code.builtinCode, code.deprecatedBuiltinCode))
        if name not in PASSTHROUGH:
            continue
        idx = [i for i in op.inputs if i >= 0] if PASSTHROUGH[name] is None else [op.inputs[k] for k in PASSTHROUGH[name]]
        for i in idx + [op.outputs[0]]:
            parent[find(i)] = find(op.outputs[0])

    groups = {}
    for i, t in enumerate(g.tensors):
        if t.type == s.TensorType.INT8 and t.quantization is not None and t.quantization.scale is not None:
            groups.setdefault(find(i), []).append(i)

    lines = []
    for members in groups.values():
        qs = {(float(g.tensors[i].quantization.scale[0]), int(g.tensors[i].quantization.zeroPoint[0])) for i in members}
        if len(members) < 2 or len(qs) == 1:
            continue
        fixed = [i for i in members if i in g.inputs]
        if fixed:  # never change the model input's quantization
            t = g.tensors[fixed[0]].quantization
            scale, zp = float(t.scale[0]), int(t.zeroPoint[0])
        else:
            lo = min((-128 - z) * sc for sc, z in qs)
            hi = max((127 - z) * sc for sc, z in qs)
            lo, hi = min(lo, 0.0), max(hi, 0.0)
            scale = (hi - lo) / 255.0
            zp = int(np.clip(round(-128 - lo / scale), -128, 127))
        for i in members:
            t = g.tensors[i].quantization
            t.scale, t.zeroPoint, t.min, t.max = [scale], [zp], None, None
        lines.append(f"    {len(members)} tensors: {len(qs)} different qparams -> scale {scale:.5g} zp {zp}")

    b = flatbuffers.Builder(1024)
    b.Finish(m.Pack(b), file_identifier=b"TFL3")
    dst.write_bytes(b.Output())
    return "\n".join(lines) if lines else "    (nothing to unify)"


def main():
    src = OUT / "palm_det_surgered.onnx"
    tmp = OUT / "onnx2tf"
    shutil.rmtree(tmp, ignore_errors=True)
    run_onnx2tf(src, tmp)

    f32 = sorted(tmp.glob("*_float32.tflite"))
    i8 = sorted(tmp.glob("*_full_integer_quant.tflite"))
    assert f32 and i8, "onnx2tf produced no float32 / full_integer_quant model"
    shutil.copy(f32[0], OUT / "model_f32.tflite")
    print("  unifying quantization across pass-through ops:")
    print(unify_passthrough_qparams(i8[0], OUT / "model_int8.tflite"))
    for dst in ("model_f32.tflite", "model_int8.tflite"):
        size = (OUT / dst).stat().st_size
        print(f"  {dst}: {size / 1024:.0f} KB, ops: {', '.join(ops_used(OUT / dst))}")


if __name__ == "__main__":
    main()
