"""Stage 4: ONNX -> PyTorch -> Core ML.

coremltools has no ONNX importer any more (removed in 6.0), so the route
is: onnx2torch rebuilds the graph as a torch.fx module, torch.jit.trace
records it, and coremltools converts the trace into an ML Program
(.mlpackage).

Outputs:
  build/palm_fp16.mlpackage   weights and math in float16 (the Neural
                              Engine only computes in fp16)
  build/palm_fp32.mlpackage   float32, runs on CPU/GPU only
  build/palm_trap_fp16.mlpackage  the *original* ONNX converted the same
                              way, to show what the PRelu surgery fixes
  build/convert.md            accuracy vs ONNX Runtime and the compute
                              plan: which device Core ML picks per op

The compute plan (MLComputePlan, macOS 14.4+) is Core ML's own answer to
"what runs where": for every op it lists the devices that *could* run it
and the one it prefers, plus an estimated relative cost.
"""
import collections
import warnings

import coremltools as ct
import numpy as np
import onnx
import onnx2torch
import onnxruntime as ort
import torch
from coremltools.models.compute_plan import MLComputePlan

from common import (BUILD, COREML_FP16, COREML_FP32, ORIGINAL_ONNX, RES,
                    SURGERED_ONNX, letterbox, load_rgb, md_table, palm_samples)

COREML_TRAP = BUILD / "palm_trap_fp16.mlpackage"

COMPUTE_UNITS = {
    "CPU_ONLY": ct.ComputeUnit.CPU_ONLY,
    "CPU_AND_GPU": ct.ComputeUnit.CPU_AND_GPU,
    "CPU_AND_NE": ct.ComputeUnit.CPU_AND_NE,
    "ALL": ct.ComputeUnit.ALL,
}

DEVICE = {"MLCPUComputeDevice": "CPU", "MLGPUComputeDevice": "GPU",
          "MLNeuralEngineComputeDevice": "ANE"}


def to_coreml(onnx_path, precision):
    torch_model = onnx2torch.convert(onnx.load(onnx_path)).eval()
    example = torch.zeros(1, RES, RES, 3)
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        traced = torch.jit.trace(torch_model, example)
    return ct.convert(
        traced,
        inputs=[ct.TensorType(name="input", shape=example.shape)],
        outputs=[ct.TensorType(name="boxes"), ct.TensorType(name="scores")],
        compute_precision=precision,
        minimum_deployment_target=ct.target.macOS14,
    )


def compute_plan(mlmodel, units):
    """Ops per preferred device and the share of the estimated cost."""
    plan = MLComputePlan.load_from_path(path=mlmodel.get_compiled_model_path(),
                                        compute_units=COMPUTE_UNITS[units])
    ops, cost = collections.Counter(), collections.Counter()
    for op in plan.model_structure.program.functions["main"].block.operations:
        usage = plan.get_compute_device_usage_for_mlprogram_operation(op)
        if usage is None:                     # const ops: no device
            continue
        dev = DEVICE[type(usage.preferred_compute_device).__name__]
        ops[dev] += 1
        est = plan.get_estimated_cost_for_mlprogram_operation(op)
        cost[dev] += est.weight if est else 0.0
    return ops, cost


def accuracy(mlmodel, units):
    """Max |diff| vs ONNX Runtime CPU (float32) on the zoo test images."""
    ref = ort.InferenceSession(SURGERED_ONNX, providers=["CPUExecutionProvider"])
    m = ct.models.MLModel(str(mlmodel.package_path), compute_units=COMPUTE_UNITS[units])
    db = ds = 0.0
    for jpg, _ in palm_samples():
        x, _ = letterbox(load_rgb(jpg))
        rb, rs = ref.run(["boxes", "scores"], {"input": x})
        out = m.predict({"input": x})
        db = max(db, float(np.abs(out["boxes"] - rb).max()))
        ds = max(ds, float(np.abs(out["scores"] - rs).max()))
    return db, ds


def main():
    models = [
        ("fp16", SURGERED_ONNX, ct.precision.FLOAT16, COREML_FP16),
        ("fp32", SURGERED_ONNX, ct.precision.FLOAT32, COREML_FP32),
        ("trap fp16 (no surgery)", ORIGINAL_ONNX, ct.precision.FLOAT16, COREML_TRAP),
    ]
    plan_rows, acc_rows = [], []
    for label, src, precision, dst in models:
        print(f"== {label}: {src.name} -> {dst.name}")
        ml = to_coreml(src, precision)
        ml.save(str(dst))
        ml = ct.models.MLModel(str(dst))
        for units in COMPUTE_UNITS:
            ops, cost = compute_plan(ml, units)
            total = sum(cost.values()) or 1.0
            plan_rows.append([label, units, sum(ops.values()),
                              " / ".join(f"{d} {ops[d]}" for d in ("CPU", "GPU", "ANE") if ops[d]),
                              " / ".join(f"{d} {100 * cost[d] / total:.0f}%"
                                         for d in ("CPU", "GPU", "ANE") if cost[d])])
        for units in ("CPU_ONLY", "ALL"):
            db, ds = accuracy(ml, units)
            acc_rows.append([label, units, f"{db:.4f}", f"{ds:.4f}"])

    text = "\n".join([
        "# Core ML conversion", "",
        "## Compute plan: preferred device per op", "",
        "Ops = non-const ML Program ops. Cost = Core ML's estimated share of the work.", "",
        md_table(["model", "compute units", "ops", "ops per device", "cost per device"], plan_rows), "",
        "## Accuracy vs ONNX Runtime CPU (float32), zoo test images", "",
        "boxes in input pixels (0..192), scores as logits.", "",
        md_table(["model", "compute units", "max abs diff, boxes", "max abs diff, scores"], acc_rows), "",
    ])
    print(text)
    (BUILD / "convert.md").write_text(text)


if __name__ == "__main__":
    main()
