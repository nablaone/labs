"""Stage 5: ONNX -> TFLite (float32 and full-integer int8) with onnx2tf.

onnx2tf rebuilds the graph as a TensorFlow model (transposing NCHW ONNX
into NHWC TF on the way), then the TFLite converter quantizes it:
per-channel int8 weights, per-tensor int8 activations calibrated on
calib.npy, int8 input and output tensors. Full integer is what TFLM's
reference kernels run without any float math.

Writes build/rRES/model_f32.tflite and model_int8.tflite, and prints the
TFLite operator set (that's the list of kernels the firmware's op
resolver has to register).
"""
import shutil
import subprocess

from common import OUT


def ops_used(path):
    from ai_edge_litert.interpreter import Interpreter
    it = Interpreter(model_path=str(path))
    return sorted({op["op_name"] for op in it._get_ops_details()})


def main():
    src = OUT / "palm_det_surgered.onnx"
    tmp = OUT / "onnx2tf"
    shutil.rmtree(tmp, ignore_errors=True)
    cmd = [
        "onnx2tf", "-i", str(src), "-o", str(tmp),
        # TensorFlow's own converter (MLIR quantizer). onnx2tf 2.x defaults to
        # a "flatbuffer_direct" writer whose per-channel int8 output sets
        # quantized_dimension=3 on rank-1 tensors, which LiteRT rejects.
        "--tflite_backend", "tf_converter",
        "-oiqt",                     # also emit integer-quantized models
        "-qt", "per-channel",
        # calibration data is already in [0, 1]; mean 0 / std 1 = no extra normalisation
        "-cind", "input", str(OUT / "calib.npy"), "[[[[0.0,0.0,0.0]]]]", "[[[[1.0,1.0,1.0]]]]",
        "-iqd", "int8", "-oqd", "int8",
        "-n",                        # non-verbose
    ]
    print("  " + " ".join(cmd))
    subprocess.run(cmd, check=True)

    picks = {"model_f32.tflite": "*_float32.tflite", "model_int8.tflite": "*_full_integer_quant.tflite"}
    for dst, pattern in picks.items():
        found = sorted(tmp.glob(pattern))
        assert found, f"onnx2tf produced no {pattern}"
        shutil.copy(found[0], OUT / dst)
        size = (OUT / dst).stat().st_size
        print(f"  {dst}: {size / 1024:.0f} KB, ops: {', '.join(ops_used(OUT / dst))}")


if __name__ == "__main__":
    main()
