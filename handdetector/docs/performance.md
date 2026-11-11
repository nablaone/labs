# Making the palm detector fast on the ESP32-S3

How one inference of the OpenCV-zoo palm detector went from **137 s** to
**1.63 s** on the FireBeetle 2 ESP32-S3 (240 MHz, octal PSRAM). It still
uses upstream TensorFlow Lite Micro and no vendor ML libraries. The model
pipeline itself is in [model-pipeline.md](model-pipeline.md).

Ground rules for the whole effort:
- **No Espressif ML stack** (ESP-NN, esp-tflite-micro, ESP-DL). The
  question is what a generic model plus generic tooling can do. Our own
  kernels using the chip's instruction set are fine.
- **Bit-exact or it doesn't count.** Every faster kernel must produce the
  same bytes as TFLM's reference kernels. A speed-up that changes results
  would make every later comparison meaningless.

## Starting point

The first working build ran the converted int8 model on TFLM's reference
kernels:

| input | conv MACs | inference |
|---|---|---|
| 192×192 (zoo original) | 278 M | **137 s** |
| 96×96 | 70 M | **33.8 s** |

That is about 2 M MAC/s, roughly 120 CPU cycles per multiply-accumulate.
Per-op profile at 96×96 (`det prof`):

| op | ms | % |
|---|---|---|
| CONV_2D (30) | 25 476 | 75.3 |
| DEPTHWISE_CONV_2D (23) | 5 647 | 16.7 |
| SUB / ABS / MUL / RELU (104) | 1 283 | 3.8 |
| ADD (51) | 1 039 | 3.1 |
| TRANSPOSE (56) | 278 | 0.8 |
| rest | 101 | 0.3 |

### Why it was slow

- **TFLM's reference kernels are written for clarity.** For every
  multiply-accumulate they recompute 4-D tensor offsets and check padding
  bounds. Every output gets a 64-bit fixed-point requantization.
  Elementwise ops call a function through a pointer for every element.
- **No SIMD.** The ESP32-S3 has a SIMD unit (PIE, Processor Instruction
  Extensions), but GCC never emits its instructions. The disassembled TFLM
  kernels contained **0** `ee.*` instructions. TFLM's only "xtensa"
  optimized kernels target Cadence HiFi/Vision DSPs, which the S3's LX7
  core doesn't have.
- **Memory.** The tensor arena is in PSRAM, behind the cache. The weights
  are read straight from memory-mapped flash.
- **The converted graph is bloated.** onnx2tf's `tf_converter` path splits
  every PRelu into ABS/SUB/MUL/RELU/ADD and wraps them in TRANSPOSEs. That
  adds about 180 ops on top of the 53 convolutions.

## What changed, step by step

| step | inference (96×96) | gain |
|---|---|---|
| reference kernels, 192×192 | 137 s | |
| input 192 → 96 | 33.8 s | 4× (4× fewer pixels) |
| + PIE pointwise 1×1 conv | 11.7 s | 2.9× |
| + PIE depthwise 5×5, im2col for the first conv | 3.9 s | 3.0× |
| + lookup-table elementwise ops | 2.6 s | 1.5× |
| + native PRELU (different converter output) | **1.63 s** | 1.6× |

Overall: **21× faster than reference kernels at the same input size,
84× faster than the starting point.**

### 1. Smaller input: 192 → 96

The cheapest win. Model surgery replaced the hard-coded Resize sizes with
scales, so the same ONNX converts at any input size. Anchors are
generated for whatever size the model has.

The cost is accuracy. Host int8 palm scores on the test images drop from
0.95–0.96 (192) to 0.90–0.94 (96). On a live camera frame of a hand, the
96 model gives 0.86.

### 2. PIE pointwise kernel (33.8 → 11.7 s)

29 of the 53 convolutions are 1×1 and hold 74% of the MACs. Each output is
a dot product of one pixel's channels with one filter row.
`ee.vmulas.s8.accx` multiplies 16 int8 pairs and sums them into a 40-bit
accumulator, so a 256-channel dot product is 16 instructions. The kernel
plugs into TFLM by wrapping the reference registration:
- **Prepare** is TFLM's own, so quantization parameters are identical.
- **Invoke** takes the PIE path when a layer fits and falls back to the
  reference otherwise.

Things that had to be right:
- **Input offset folding.** Σw·(x+off) = Σw·x + off·Σw. The second term
  goes into a per-channel bias, once per layer.
- **Alignment.** PIE loads silently ignore the low 4 address bits. The
  weights in the flash-mapped flatbuffer aren't aligned, so they're copied
  once into 16-byte-aligned PSRAM.
- **Requantization** reuses TFLM's own `MultiplyByQuantizedMultiplier`.

Pointwise went from ~22.8 s to 0.66 s (35×).

### 3. PIE depthwise and im2col (11.7 → 3.9 s)

**Depthwise 5×5** (23 layers) handles 16 channels at a time:
- `ee.vmulas.s8.qacc` multiplies lane by lane into sixteen 20-bit
  accumulators, one instruction per tap.
- The input is first copied into a buffer padded with the input zero
  point. Every tap is then in bounds and aligned, and a padded tap adds
  w·(zp − zp) = 0, exactly like the reference skipping it.
- The accumulator registers are written to memory and the 20-bit lanes
  unpacked in C. The lane layout is in the TRM's instruction pages.
- A 20-bit lane holds up to 32 taps of |w·x| ≤ 2¹⁴, so 5×5 fits.

Depthwise went from 5.6 s to 0.40 s (14×).

**Every other convolution** (here only the first 5×5 stride-2 3→32 conv)
uses im2col. Each output pixel's 75 input bytes are gathered into a
16-aligned row of 80, then the same dot product as pointwise. That conv
went from 2.7 s to 0.05 s (51×).

### 4. Lookup tables for the elementwise ops (3.9 → 2.6 s)

With convolutions fast, the 155 elementwise layers from the PRelu
decomposition became 60% of the time. An int8 input has only 256 values,
so most of the work becomes a table lookup:

| op | how | before → after |
|---|---|---|
| ABS, RELU | `out = T[x]`, table filled by running TFLM's own kernel on all 256 inputs | 0.50 s → 0.09 s |
| MUL by per-channel slope | one 256-entry table per channel | 0.24 s → 0.10 s |
| ADD, SUB | per-input rescale tables; the final requantization stays per element | 1.62 s → 0.84 s |

Not SIMD: plain C, but no function pointer per element and one
requantization instead of three.

## How correctness was kept

- **`det golden`** runs the host's int8 input for a zoo palm image on the
  board and compares the raw output tensors with host LiteRT. The golden
  vectors come from LiteRT's **reference** kernels (`BUILTIN_REF`). Its
  default XNNPACK kernels round differently, up to 16 LSB off. Result at
  every step: 0 differing bytes.
- **`det pie verify on`** reruns the reference kernel after every PIE or
  table layer and counts differing output bytes. Current result: 0 in all
  53 conv layers and 155 elementwise layers.
- **Live frames:** a camera frame's model input was dumped (`det dump`)
  and run on the host. The scores matched the device byte for byte.

### 5. Native PRELU (2.6 → 1.63 s)

The table kernels made the PRelu decomposition cheaper, but it was still
~180 ops (five per PRelu plus TRANSPOSEs), about 1 s. The fix was in the
converter, not the firmware.

onnx2tf's own TFLite writer (`flatbuffer_direct`) emits native PRELU and
no TRANSPOSE at all: 124 ops instead of ~300. Its int8 output was unusable
at first, so `pipeline/convert.py` fixes two bugs:
- **Depthwise biases** got the weights' `quantized_dimension=3` on a 1-D
  tensor, and the model wouldn't load. Fixed by wrapping onnx2tf's bias
  quantizer at runtime.
- **Pass-through ops** (MAX_POOL, RESHAPE, RESIZE, CONCAT) got different
  input and output quantization, and their kernels don't rescale. Fixed
  afterwards in the flatbuffer by giving each linked group of tensors one
  shared scale and zero point.

On the device, PRELU uses a 256×C table per layer, filled by TFLM's own
PRELU on a fake 256×C input, so it's exact by construction.

| | before | after |
|---|---|---|
| ops | ~300 | 124 |
| PRelu cost (SUB/ABS/MUL/RELU + half the ADDs → PRELU) | ~0.76 s | 0.057 s |
| TRANSPOSE | 0.28 s | 0 (none left) |
| tensor arena | 375 KB | 291 KB |
| inference | 2.6 s | **1.63 s** |

This is a different int8 model, so it's not bit-identical to the old one.
It has its own golden vectors, and the device matches them bit for bit.
Accuracy is about the same: host palms 0.84–0.96 (before 0.90–0.94),
worst no-hand 0.015 (before 0.05), a live camera hand 0.90 (before 0.87).

## Current status

96×96, 1.63 s per inference (`det prof`):

| op | ms | % | kernel |
|---|---|---|---|
| CONV_2D (30) | 744 | 45.6 | PIE pointwise + im2col |
| DEPTHWISE_CONV_2D (23) | 438 | 26.9 | PIE |
| ADD (25) | 261 | 16.0 | LUT + per-element requant |
| PRELU (26) | 57 | 3.5 | LUT |
| MAX_POOL_2D (4) | 54 | 3.3 | reference |
| PAD (8) | 40 | 2.5 | reference |
| RESIZE_BILINEAR (2) | 38 | 2.3 | reference |
| CONCAT, RESHAPE | 0.2 | 0.0 | reference |
| **total** | **1 632** | | |

Around it: grabbing and decoding a QVGA JPEG takes 80–170 ms, the letterbox
preprocessing 9 ms, and postprocessing 2 ms. A detection result arrives
about every 1.7 s.

Convolutions now run at about 57 M MAC/s (70 M MACs in ~1.2 s). That's
28× the reference, but far from PIE's theoretical 16 MAC/cycle (3.8
GMAC/s). The time goes to memory (activations in PSRAM, weights copied to
PSRAM) and to the scalar 64-bit requantization of every output.

## What's left

The convolutions are now 72% of the time. Ordered by expected payoff:

1. **Cheaper exact requantization.** It runs once per output in every
   conv and once per element in ADD. A hand-written 32-bit high-multiply
   instead of the generic int64 code, still exact.
2. **Internal SRAM for hot tensors.** The arena at 96×96 is 291 KB. Small
   late layers and scratch buffers could live in internal RAM instead of
   PSRAM.
3. **Pointwise tuning.** Fused load+multiply (`ee.vmulas.s8.accx.ld.ip`),
   keep a pixel's vector in registers across output channels, use the
   zero-overhead `loop`.
4. **MAX_POOL / PAD / RESIZE** on reference kernels: 0.13 s together.

## Pitfalls hit along the way

- **The idle-task watchdog on core 1 had to go.** An inference blocks
  core 1 for seconds, so `CONFIG_ESP_TASK_WDT_CHECK_IDLE_TASK_CPU1=n`.
  Verify mode on the console (core 0) takes ~35 s and makes the core-0
  watchdog print backtraces. These are warnings, not crashes.
- **`det prof` while the detector runs** shows the inference in progress
  (counters reset at its start), so read it after `det off` or `det golden`.
- **Results lag the scene.** A result describes the frame grabbed when that
  inference started. At 33 s per inference a hand looked "undetected";
  hold it still for two cycles.
