# handdetector-mac

The palm detector from [`../handdetector`](../handdetector) (MediaPipe's
palm detector, as ONNX from the OpenCV zoo), this time on a Mac: from the
model file to live hand detection on the built-in camera, through every
runtime and every piece of hardware the Mac has (CPU, GPU, **Neural
Engine**).

It's an educational experiment. The point is to see each step of
running a neural network, and why the same network takes **1 ms** on
one path and **230 ms** on another on the same machine.

```
make setup     # uv sync: Python 3.12 venv with onnxruntime, torch, coremltools
make           # the pipeline, stages 1-6 (about 2 min, needs internet once)
make run       # live camera, palm boxes + 7 keypoints
```

Needs macOS 14.4+ on Apple silicon (measured on an M1 Pro) and
[uv](https://docs.astral.sh/uv/). The first `make run` asks for camera
access for your terminal app.

## The pipeline

```
 opencv_zoo  ──1 fetch──▶ palm_detection_mediapipe_2023feb.onnx  (SHA-256 pinned)
                               │
                          2 inspect ──▶ build/inspect.md   ops, params, MACs, anchors check
                               │
                          3 surgery ──▶ build/palm_surgered.onnx
                               │         PRelu slope (1,C,1,1)→(C,1,1), names input/boxes/scores
                               │
             ┌─────────────────┼──────────────────────────┐
             ▼                 ▼                          ▼
      ONNX Runtime        onnx2torch               onnx2torch → torch.jit.trace
      (CPU kernels, or    (PyTorch CPU,              → coremltools
       Core ML provider)   or GPU via MPS)          4 convert ──▶ build/palm_fp16.mlpackage
             │                 │                          │       build/palm_fp32.mlpackage
             │                 │                          │       build/convert.md (what runs where)
             └────────── pipeline/engines.py ─────────────┘
                     one interface: infer(x) → boxes, scores
                               │
              5 bench  ──▶ build/bench.md       every engine, same input
              6 detect ──▶ build/detect/<engine>/*.jpg   + IoU with ground truth
                 run   ──▶ app.py                 camera, switch engines live
```

| stage | file | what you learn there |
|---|---|---|
| 1 fetch | `pipeline/fetch.py` | where the model comes from; pinning by commit + SHA-256 |
| 2 inspect | `pipeline/inspect_model.py` | an ONNX graph: inputs, outputs, op types, MACs, activations |
| 3 surgery | `pipeline/surgery.py` | editing a graph without changing its math, and proving it (bit-identical) |
| 4 convert | `pipeline/convert.py` | ONNX → PyTorch → Core ML; fp16 vs fp32; Core ML's compute plan |
| 5 bench | `pipeline/bench.py` | load time, first call, steady state, numerical drift per engine |
| 6 detect | `pipeline/detect.py` | the full chain on images with ground truth |
| run | `app.py` | the same chain on live video, time per stage on screen |
| | `pipeline/common.py` | everything outside the network: letterbox, anchors, decode, NMS, drawing |
| | `pipeline/engines.py` | the 14 ways to run the network |

### Outside the network (`common.py`)

The network only maps pixels to numbers. Turning them into hands is
plain code:

1. **Letterbox:** scale the frame so its long side is 192, pad to
   192×192 with black, divide by 255. Input is `(1, 192, 192, 3)`,
   NHWC, RGB, float32 in [0, 1].
2. **Network:** `boxes (1, 2016, 18)` and `scores (1, 2016, 1)`. One row
   per **anchor**: a fixed point on a 24×24 grid (2 per cell) and a 12×12
   grid (6 per cell), 2016 in total. Each row is an offset from its
   anchor: box centre (dx, dy), size (w, h) and 7 keypoints (wrist and
   the base of each finger), in input pixels.
3. **Decode:** sigmoid on the score logit, keep ≥ 0.5, add the anchor,
   undo the letterbox to get source pixels.
4. **NMS:** neighbouring anchors all see the same palm; keep the best,
   drop boxes overlapping it by IoU > 0.3.

Stage 2 checks the generated anchors against the zoo's hard-coded table.

## Runtimes and hardware

| engine | runtime | hardware | precision |
|---|---|---|---|
| `onnx-cpu` | ONNX Runtime, its own kernels (MLAS) | CPU | fp32 |
| `onnx-coreml-{cpu,gpu,ane,all}` | ONNX Runtime, Core ML execution provider | Core ML decides | fp32 / fp16 |
| `torch-cpu`, `torch-mps` | PyTorch (onnx2torch module) | CPU / GPU via Metal | fp32 |
| `coreml-{cpu,gpu,ane,all}` | Core ML, `palm_fp16.mlpackage` | Core ML decides | fp16 |
| `coreml32-{cpu,gpu}` | Core ML, `palm_fp32.mlpackage` | CPU / GPU | fp32 |
| `trap-ane` | Core ML, converted *without* the surgery | Core ML decides | fp16 |

The **Neural Engine (ANE)** is only reachable through Core ML. Core ML's
`compute_units` (`CPU_AND_NE`, `ALL`, ...) is a permission, not an
order: Core ML still places each op on the device it can and wants to.
`MLComputePlan` (macOS 14.4+) shows the choice per op; stage 4 writes it
to `build/convert.md`.

## Results (M1 Pro, macOS, palm1.jpg)

| engine | median ms | p90 ms | first call | boxes diff (px) | palm score |
|---|---|---|---|---|---|
| **coreml-ane** | **0.94** | 1.05 | 4 ms | 0.13 | 0.9541 |
| coreml-all | 0.96 | 1.05 | 4 ms | 0.13 | 0.9541 |
| onnx-coreml-all | 3.75 | 3.84 | 15 ms | 0.18 | 0.9541 |
| coreml-cpu (fp16) | 3.98 | 4.15 | 37 ms | 0.83 | 0.9540 |
| onnx-coreml-ane | 4.03 | 5.43 | 15 ms | 0.18 | 0.9541 |
| coreml-gpu (fp16) | 4.72 | 6.31 | 577 ms | 0.16 | 0.9541 |
| coreml32-gpu | 5.66 | 8.37 | 38 ms | 0.00 | 0.9541 |
| coreml32-cpu | 6.01 | 6.27 | 7 ms | 0.00 | 0.9541 |
| onnx-coreml-cpu | 7.54 | 7.65 | 8 ms | 0.00 | 0.9541 |
| onnx-cpu (reference) | 8.50 | 8.93 | 9 ms | 0 | 0.9541 |
| onnx-coreml-gpu | 8.66 | 9.63 | 41 ms | 0.25 | 0.9539 |
| torch-mps | 20.99 | 26.42 | 2002 ms | 0.00 | 0.9541 |
| trap-ane | 51.38 | 51.78 | 63 ms | 0.79 | 0.9534 |
| torch-cpu | 229.64 | 234.74 | 228 ms | 0.00 | 0.9541 |

283 M multiply-accumulates per inference: on the Neural Engine that's
~0.3 TMAC/s, far below its rated peak (probably because a 1 M-parameter
network at batch 1 can't keep it busy; not measured). For scale: the
same network at 96×96 (70 M MACs) takes 360 ms on the ESP32-S3 in
`../handdetector`, so roughly 1.5 s at 192 if it scales with MACs.

Timings vary between runs when the machine is busy: in a second full
`make` run, `onnx-cpu` measured 22.7 ms and `onnx-coreml-ane` 7.6 ms, while
the Core ML ANE numbers stayed at 0.94–0.95 ms. Compare engines within one
`make bench` run.

All engines find the palm with the same score to 3 decimals; fp16 moves
boxes by under a pixel. The test palms (`make detect`): scores 0.945–0.958,
IoU 0.74–0.80 with the zoo's ground truth.

In the app (tested headless on a still image), a frame takes ~1.3 ms
end to end with `coreml-ane`: pre-processing ~0.2 ms, inference ~1 ms,
decode ~0.1 ms. With the camera, its frame rate is the limit.

## Findings

### 1. The PRelu trap: one tensor shape decides CPU vs Neural Engine

The first conversion ran at 41–58 ms and Core ML put **nothing** on the
ANE, though the network is 53 convolutions, 26 PRelu and 25 adds, all
ANE-friendly. The compute plan showed 602 ops instead of ~129, most of
them CPU-only: `non_zero`, `gather_nd`, `scatter_nd`, `tile`, `select`,
26 of each.

Cause: the ONNX file stores PRelu slopes as `(1, C, 1, 1)`. onnx2torch
maps PRelu to `torch.nn.functional.prelu` only for `(C,)` or
`(C, 1, 1)`; otherwise it falls back to

```python
output = input_tensor.clone() * slope
mask = input_tensor >= 0
output[mask] = input_tensor[mask]      # boolean-mask indexing
```

Boolean-mask indexing has a data-dependent shape, which Core ML can only
do on the CPU. 26 of those cut the graph into CPU islands; moving data
between the ANE and the CPU 26 times costs more than the ANE saves, so
Core ML kept almost all of it on the CPU (`CPU_AND_NE`: 567 ops CPU,
35 ANE) or GPU.

Fix (stage 3): reshape the slopes to `(C, 1, 1)`, mathematically the
same broadcast, bit-identical in ONNX Runtime. Result: 129 ops, **all
on the ANE**, 51 ms → 0.94 ms. `trap-ane` keeps the unfixed model for
comparison.

Lesson: converters pattern-match. The same math in a slightly different
shape takes a different code path, and the hardware placement follows.
Look at what the converter produced (op counts, compute plan), not just
whether it runs.

### 2. fp16: Neural Engine only, and not all fp16 is equal

The ANE computes in fp16 only. The fp32 package with `CPU_AND_NE` runs
entirely on the CPU (`convert.md`). Accuracy cost here is negligible:
boxes move 0.13 px out of 192, the score is identical to 4 decimals.

Surprise: fp16 on the **CPU** (`coreml-cpu`) drifts 6× more than fp16
on the ANE (0.83 vs 0.13 px). The likely reason is where each path
rounds: how wide the accumulators are and which intermediates get
stored as fp16 differs per device. Not verified; Apple doesn't document
the ANE's internals.

### 3. PyTorch on the CPU: depthwise convolutions are 27× slower

`torch-cpu` takes 230 ms against 8.5 ms for ONNX Runtime on the same
CPU, same fp32 math. The profiler shows 84% in
`aten::_slow_conv2d_forward`, called 3486 times per inference: once per
channel of the 23 depthwise convs (3456 channels) plus once for each of
the other 30 convs. PyTorch's CPU backend takes its generic path here
and runs a grouped convolution as one tiny convolution per group. ONNX
Runtime has a dedicated depthwise kernel.

### 4. ONNX Runtime's Core ML provider: partial graphs

- ORT hands Core ML only the nodes it knows how to convert: 117 of 124,
  in 6 partitions (it logs this at session creation). Between partitions
  data goes back to ORT's CPU, which is the likely reason for 3.8 ms vs
  0.94 ms through Core ML directly.
- Its newer `MLProgram` format fails to compile this graph in ORT 1.31
  ("Required param 'pad' is missing"), so `engines.py` uses the older
  `NeuralNetwork` format.

### 5. First call and load time

Compilation happens on load or on the first call: Core ML compiles
the package for the ANE when loading (~1 s), Core ML GPU compiles
shaders on the first call (~0.6 s), PyTorch MPS ~2 s. `engines.make()`
does one call before timing anything; in an app, warm up before the
first real frame.

## Pitfalls

- **coremltools vs torch versions:** coremltools 9.0 is tested with
  torch 2.7.0; newer torch converts this model too but prints warnings.
  Pinned in `pyproject.toml`.
- **coremltools has no ONNX importer** (removed in 6.0), hence the
  detour through PyTorch.
- **Google Drive downloads can stall** with no error; `fetch.py` sets a
  timeout and retries.
- **Camera access** belongs to the terminal app, not to Python. Without
  it, OpenCV opens the camera but gets no frames (the app says so).

## Ideas for further experiments

- Weight quantization (`coremltools.optimize`: int8, palettization) and
  its effect on ANE speed and accuracy.
- The second MediaPipe stage: hand landmarks (21 points) on the crop
  around each palm.
- Xcode's Core ML performance report (open the `.mlpackage`) next to
  `convert.md`.
- 96×96 input (the ESP32's resolution) via the Resize surgery from
  `../handdetector/model/pipeline/surgery.py`: how fast can it go?
- Batch several frames per call: the ANE is underused at batch 1.
