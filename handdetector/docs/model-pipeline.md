# ONNX → ESP32-S3: palm detector pipeline

The experiment: take a generic model off the internet (an ONNX file, not
something made for microcontrollers) and run it on the ESP32-S3 with only
generic, non-vendor tooling. Can it be done, how, and what does it cost?

- **Model:** OpenCV zoo `palm_detection_mediapipe_2023feb.onnx`
  (MediaPipe palm detector, Apache 2.0).
- **Conversion:** onnx2tf → TensorFlow Lite int8.
- **Runtime:** upstream TensorFlow Lite Micro at a pinned commit, reference
  kernels only. No ESP-NN, no esp-tflite-micro, no Espressif ML libraries.
- **Output:** the heartbeat LED blinks fast while a hand is detected.

Measured 2026-11-11 on the FireBeetle 2 ESP32-S3 (240 MHz, octal PSRAM).

## Result

| | 192×192 (zoo original) | 128×128 | 96×96 (default) |
|---|---|---|---|
| conv MACs | 278 M | 124 M | 70 M |
| anchors | 2016 | 896 | 504 |
| int8 .tflite | 1175 KB | 1175 KB | 1175 KB |
| TFLM arena | 1239 KB (PSRAM) | — | 375 KB (PSRAM) |
| **inference on the S3** | **137 s** | ~60 s (est.) | **33.8 s** |
| host int8 palm scores (3 test images) | 0.95–0.96 | 0.97–0.98 | 0.90–0.94 |
| host int8 worst no-hand score | 0.24 | 0.07 | 0.05 |

**It runs, and it's correct.** The device output is bit-exact with host
LiteRT running reference kernels (`det golden`). **It's slow:** about
2 M MAC/s, roughly 120 cycles per multiply-accumulate. The cause is
TFLM's reference kernels (see "Where the time goes").

## Pipeline

Everything with Python/TensorFlow runs in the `handdetector-model`
container (`model/Dockerfile`: python 3.12 + `onnx2tf[tensorflow]==2.6.9`,
which pins TF 2.21, onnx 1.20.1, onnxruntime 1.26). The host only does
USB: `capture`, `flash-model`, `flash-golden`.

```sh
cd model
make                  # fetch -> inspect -> surgery -> calib -> convert -> validate (RES=96)
make RES=192          # same at another input size, outputs in build/r192/
make tflm             # generate TFLM sources into ../components/tflite-micro/tflm
make capture N=20     # pull calibration frames from the board's camera
make flash-model      # .tflite -> "model" flash partition (0x310000)
make flash-golden     # host test vectors -> end of the model partition
```

| stage | script | what it does |
|---|---|---|
| fetch | `pipeline/fetch.py` | model, zoo reference wrapper and test images, pinned to a zoo commit and SHA-256 |
| inspect | `pipeline/inspect_model.py` | I/O, operators, params, MACs, largest activations → `build/fetch/original.md` |
| surgery | `pipeline/surgery.py` | graph edits (below), onnx-simplifier, ONNX Runtime check against the original |
| calib | `pipeline/calib.py` | int8 calibration set: augmented zoo palms + `data/camera/*.jpg` (our sensor) |
| convert | `pipeline/convert.py` | onnx2tf → `model_f32.tflite`, `model_int8.tflite` (full integer, per-channel) |
| validate | `pipeline/validate.py` | zoo ONNX / surgered ONNX / TFLite f32 / TFLite int8 on palms + no-hand images → `report.md`; golden vectors |

### Surgery

| what | why |
|---|---|
| NHWC `input_1` + Transpose → NCHW `input` | tf2onnx artefact. onnx2tf assumes NCHW ONNX and gives the TFLite model a clean NHWC input. |
| Resize `sizes` (12×12, 24×24) → `scales` [1,1,2,2] | the hard-coded sizes only work at 192. With scales, any input size works. |
| regressor heads: 18 → 4 values per anchor | keypoints aren't needed, so the conv weights are sliced (108→24, 36→8 channels). Saves 5.2 M MACs and makes the box output 4.5× smaller. |
| outputs renamed `boxes` [1,N,4], `scores` [1,N,1] | stable names. N follows the input size. |

At 192 the surgered model is **bit-identical** to the original on the
test images (max |diff| 0.0).

### Pre/post-processing (identical on host and device)

- **Letterbox:** keep the aspect ratio, scale the long side to RES, black
  padding, centred.
- **Input:** RGB / 255. The int8 input has scale 1/255 and zero point
  −128, so a quantized input byte is simply `pixel − 128`.
- **Anchors:** stride 8 with 2 per cell, stride 16 with 6 per cell, centre
  of each cell, row-major. Generated for any RES, and checked against the
  zoo's hard-coded table at 192.
- **Decode:** box = (anchor + d/RES) in letterbox space, then mapped back to
  source pixels. Score = sigmoid(logit).
- **On the device:** only the best anchor is decoded (the LED only needs
  "is there a hand"), and the threshold is applied to the logit.

## Findings along the way

1. **onnx2tf 2.x's default `flatbuffer_direct` backend writes broken int8.**
   Per-channel quantization puts `quantized_dimension=3` on rank-1 tensors,
   and LiteRT refuses to load the model. Fix: `--tflite_backend
   tf_converter` (TensorFlow's own MLIR quantizer).
2. **The tf_converter path decomposes PRelu.** It becomes
   ABS/MUL/RELU/SUB/ADD: 5 ops and extra full-size tensors instead of one
   PRELU (TFLM supports PRELU). The decomposition costs about 7% of
   inference time. Getting a native PRELU back (the flatbuffer_direct path
   does emit it) is a possible optimization.
3. **The zoo's own int8 ONNX isn't usable as a source.** It's ONNX Runtime
   QOperator format with `com.microsoft` opsets, and PRelu stays float
   (31 Quantize / 28 Dequantize nodes). We quantize from the float model
   ourselves.
4. **Host LiteRT's default kernels aren't a bit-exact reference.** The
   default resolver uses optimized kernels plus XNNPACK, whose
   requantization rounds differently. On palm1 that's up to 16 LSB off the
   reference kernels, with the same detections. The golden vectors are
   therefore computed with `OpResolverType.BUILTIN_REF`.
5. **esp32-camera channel order differs by source format.** Its
   `fmt2rgb888()` gives R,G,B for JPEG (TJpgDec) but B,G,R for RGB565 (BMP
   order). The detector unpacks RGB565 itself (big-endian).
6. **This TFLM version moved the schema** to
   `tensorflow/lite/micro/schema/schema_generated.h`. It carries its own
   `fixedpoint.h`, so gemmlowp and ruy aren't needed anymore.
7. **An inference blocks core 1 for tens of seconds.** The idle-task
   watchdog on CPU1 is therefore disabled
   (`CONFIG_ESP_TASK_WDT_CHECK_IDLE_TASK_CPU1=n`). Core 0 stays watched.

## Where the time goes

`det prof` hooks `tflite::MicroProfilerInterface` and times every kernel.
One 96×96 inference:

| op | calls | ms | % |
|---|---|---|---|
| CONV_2D | 30 | 25 476 | 75.3 |
| DEPTHWISE_CONV_2D | 23 | 5 647 | 16.7 |
| ADD | 51 | 1 039 | 3.1 |
| SUB / ABS / MUL / RELU (PRelu decomposition) | 104 | 1 283 | 3.8 |
| TRANSPOSE | 56 | 278 | 0.8 |
| MAX_POOL_2D / RESIZE_BILINEAR / PAD / RESHAPE / CONCAT | 15 | 101 | 0.3 |
| **total** | | **33 824** | |

Convolutions are 92%. TFLM's reference conv kernels are written for
clarity:
- for every multiply-accumulate they recompute 4-D tensor offsets (several
  multiplies) and check the padding bounds;
- per output they do a 64-bit fixed-point requantization;
- all tensors are in PSRAM (behind the cache) and the weights in
  memory-mapped flash.

TFLM's only "xtensa" optimized kernels target Cadence HiFi/Vision DSPs,
which the S3's LX7 doesn't have.

## PIE SIMD kernels (own, not portable)

The S3's **PIE** unit (Processor Instruction Extensions, TRM chapter 1)
has eight 128-bit `q` registers, sixteen 8-bit multipliers, a 40-bit
`ACCX` accumulator and 16-lane `QACC` accumulators. GCC never emits these
instructions by itself: the disassembled TFLM reference kernels had
**0** `ee.*` instructions. So the kernels are hand-written with inline
asm (`main/pie_conv.cc`).

How they plug into TFLM: `PieRegister_CONV_2D()` wraps the reference
registration.
- **Prepare** is TFLM's own, so the quantization parameters are
  identical.
- **Invoke** dispatches eligible layers (int8, 1×1, stride 1, IC % 16
  == 0) to PIE and everything else to the reference kernel.

**Pointwise 1×1** (29 layers, 74% of MACs): an output is a dot product of
a pixel's IC channels with a filter row.
- `ee.zero.accx`, then IC/16 × (`ee.vld.128.ip` x2 + `ee.vmulas.s8.accx`),
  then `rur.accx_0`.
- The input offset is folded into the bias (Σ(x+off)·w = Σx·w + off·Σw),
  and requantization is TFLM's `MultiplyByQuantizedMultiplier`.
- Weights are copied once into 16-byte-aligned PSRAM, because PIE loads
  ignore the low 4 address bits and the flatbuffer in flash has no
  alignment guarantee.

| 96×96 inference | reference kernels | + PIE pointwise |
|---|---|---|
| total | 33.8 s | **11.7 s** (2.9×) |
| pointwise 1×1 (29 layers) | ~22.8 s | **0.66 s** (~35×) |
| depthwise 5×5 (reference) | 5.6 s | 5.6 s |
| first conv 5×5 s2 (reference) | 2.7 s | 2.7 s |
| ADD/SUB/ABS/MUL/RELU | 2.3 s | 2.3 s |
| correctness | bit-exact | **bit-exact** (`det golden`, `det pie verify on`: 0 mismatches in all 29 layers) |

`det pie off` switches back to the reference kernels for A/B timing.
`det pie stats` gives per-layer path and time.

## Correctness check on the device

`make flash-golden` + `det golden` runs the host's int8 input for palm1
through TFLM on the board and compares the raw output tensors:

```
inference 33824.7 ms; best score 0.941 (anchor 155)
vs host LiteRT (reference kernels): boxes 0/2016 differ, scores 0/504 differ, max |diff| 0 LSB -> BIT-EXACT
```

## Next steps (ideas, not done)

- **PIE depthwise 5×5** (now 48% of the time): 16 channels per vector,
  `ee.vmulas.s8.qacc` over the 25 taps, padding by a zero-point border.
  QACC lanes are 20 bits and have to be unpacked to int32 for exact
  requantization.
- **PIE first conv** (5×5 stride 2, 3→32): repack 5×5×3 = 75 inputs per
  output into a 16-aligned vector (im2col), then use the ACCX dot product.
- **Elementwise ADD/SUB/MUL/ABS/RELU:** native PRELU (one op instead of
  five), or fused PIE versions.
- **Pointwise tuning:** fused load+MAC (`ee.vmulas.s8.accx.ld.ip`), keep
  the pixel vector in `q` registers across output channels, zero-overhead
  `loop`.
- **SRAM arena:** at 96 the arena is 375 KB. Putting the hot tensors in
  internal RAM avoids PSRAM latency.
- **Native PRELU:** -7% time and fewer intermediate tensors.
- **Calibration data:** more frames from our own camera, with hands.

## Device commands

| | |
|---|---|
| `det` | status: model, quant params, last score/box, timing per stage |
| `det on` / `det off` | pause the detector (e.g. to free the camera/CPU) |
| `det thr <0..1>` | score threshold (default 0.5) |
| `det prof` | per-op time of the last inference |
| `det golden` | run the host test vectors, compare bit for bit |
| `det dump` | last inference snapshot (model input + raw outputs) as hex, same bytes as BLE `detsnap` |

The web page's device panel shows detector state, last score and
inference time (BLE info characteristic, `det` object).
