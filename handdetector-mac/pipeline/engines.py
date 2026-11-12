"""Every way this repo runs the network, behind one interface.

    engine = make("coreml-ane")
    boxes, scores = engine.infer(x)   # x: (1, 192, 192, 3) float32 in [0, 1]
                                      # boxes (2016, 18), scores (2016,) logits

Three runtimes, each with a choice of hardware:

  onnx-*    ONNX Runtime on the surgered .onnx file.
            onnx-cpu: its own CPU kernels (MLAS), float32.
            onnx-coreml-*: hands the parts of the graph it can to Core ML
            (the "Core ML execution provider"), the rest stays on its CPU.
  torch-*   PyTorch on the onnx2torch module: CPU, or the GPU via Metal
            Performance Shaders (mps).
  coreml-*  Core ML directly on the converted .mlpackage. The compute
            units are a *permission*, not an order: Core ML may still put
            an op on the CPU if the GPU/ANE can't run it (convert.md
            shows what it chose).

The Neural Engine is only reachable through Core ML (directly or via
ONNX Runtime's provider); PyTorch and ONNX Runtime's own kernels never
use it.
"""
import time

import numpy as np

from common import COREML_FP16, COREML_FP32, SURGERED_ONNX, BUILD

COREML_TRAP = BUILD / "palm_trap_fp16.mlpackage"

# name -> (description, factory)
ENGINES = {}


def engine(name, description):
    def register(factory):
        ENGINES[name] = (description, factory)
        return factory
    return register


class Engine:
    name = "?"

    def infer(self, x):
        raise NotImplementedError


# --- ONNX Runtime ----------------------------------------------------------

class OnnxEngine(Engine):
    def __init__(self, providers):
        import onnxruntime as ort
        opts = ort.SessionOptions()
        opts.log_severity_level = 3          # hide the "N partitions" warning
        self.session = ort.InferenceSession(SURGERED_ONNX, opts, providers=providers)

    def infer(self, x):
        boxes, scores = self.session.run(["boxes", "scores"], {"input": x})
        return boxes[0], scores[0, :, 0]


@engine("onnx-cpu", "ONNX Runtime, own CPU kernels, fp32")
def _():
    return OnnxEngine(["CPUExecutionProvider"])


def onnx_coreml(units):
    # ModelFormat MLProgram fails to compile this graph in onnxruntime
    # 1.31 ("Required param 'pad' is missing"), so the older NeuralNetwork
    # format. ORT gives Core ML 117 of the 124 nodes, in 6 partitions.
    return OnnxEngine([("CoreMLExecutionProvider",
                        {"ModelFormat": "NeuralNetwork", "MLComputeUnits": units}),
                       "CPUExecutionProvider"])


for _units, _suffix, _what in [("CPUOnly", "cpu", "CPU"), ("CPUAndGPU", "gpu", "CPU+GPU"),
                               ("CPUAndNeuralEngine", "ane", "CPU+ANE"), ("ALL", "all", "any")]:
    engine(f"onnx-coreml-{_suffix}", f"ONNX Runtime -> Core ML provider, {_what}")(
        lambda u=_units: onnx_coreml(u))


# --- PyTorch ---------------------------------------------------------------

class TorchEngine(Engine):
    def __init__(self, device):
        import onnx
        import onnx2torch
        import torch
        self.torch, self.device = torch, device
        self.model = onnx2torch.convert(onnx.load(SURGERED_ONNX)).eval().to(device)

    def infer(self, x):
        with self.torch.no_grad():
            boxes, scores = self.model(self.torch.from_numpy(x).to(self.device))
        # .cpu() waits for the GPU, so the timing includes the real work
        return boxes[0].cpu().numpy(), scores[0, :, 0].cpu().numpy()


@engine("torch-cpu", "PyTorch, CPU, fp32")
def _():
    return TorchEngine("cpu")


@engine("torch-mps", "PyTorch, GPU via Metal (MPS), fp32")
def _():
    return TorchEngine("mps")


# --- Core ML ---------------------------------------------------------------

class CoreMLEngine(Engine):
    def __init__(self, package, units):
        import coremltools as ct
        self.model = ct.models.MLModel(str(package), compute_units=getattr(ct.ComputeUnit, units))

    def infer(self, x):
        out = self.model.predict({"input": x})
        return out["boxes"][0], out["scores"][0, :, 0]


for _units, _suffix, _what in [("CPU_ONLY", "cpu", "CPU"), ("CPU_AND_GPU", "gpu", "CPU+GPU"),
                               ("CPU_AND_NE", "ane", "CPU+ANE"), ("ALL", "all", "any")]:
    engine(f"coreml-{_suffix}", f"Core ML fp16 .mlpackage, {_what}")(
        lambda u=_units: CoreMLEngine(COREML_FP16, u))

engine("coreml32-cpu", "Core ML fp32 .mlpackage, CPU")(lambda: CoreMLEngine(COREML_FP32, "CPU_ONLY"))
engine("coreml32-gpu", "Core ML fp32 .mlpackage, CPU+GPU")(lambda: CoreMLEngine(COREML_FP32, "CPU_AND_GPU"))
engine("trap-ane", "Core ML fp16, *without* the PRelu surgery, CPU+ANE")(
    lambda: CoreMLEngine(COREML_TRAP, "CPU_AND_NE"))


def make(name):
    """Build an engine; .load_s is how long loading/compiling took."""
    if name not in ENGINES:
        raise SystemExit(f"unknown engine {name!r}; one of: {', '.join(ENGINES)}")
    t = time.perf_counter()
    e = ENGINES[name][1]()
    e.name, e.load_s = name, time.perf_counter() - t
    # The first call often compiles or uploads weights (ANE, MPS): do it now.
    t = time.perf_counter()
    e.infer(np.zeros((1, 192, 192, 3), np.float32))
    e.first_s = time.perf_counter() - t
    return e
