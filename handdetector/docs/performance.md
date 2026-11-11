# Making the palm detector fast on the ESP32-S3

How one inference of the OpenCV-zoo palm detector went from **137 s** to
**0.66 s** on the FireBeetle 2 ESP32-S3 (240 MHz, octal PSRAM). It still
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
| + native PRELU (different converter output) | 1.63 s | 1.6× |
| + fast exact requantization, cache-blocked pointwise | 1.04 s | 1.6× |
| + both CPU cores | 0.81 s | 1.28× |
| + CONV→ADD→PRELU fusion | 0.78 s | 1.06× |
| + own MAX_POOL / PAD / RESIZE | **0.66 s** | 1.18× |

Overall: **51× faster than reference kernels at the same input size,
207× faster than the starting point.**

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

### 6. Requantization and cache blocking (1.63 → 1.04 s)

A cycle-count microbenchmark (`det pie bench`) showed where the time per
output value went:

| building block | cycles |
|---|---|
| TFLM `MultiplyByQuantizedMultiplier` (per output) | 80 |
| 32-channel dot product, weights cached | 32 |
| 256-channel dot product, weights cached | 145 |
| 256-channel dot product, weights from PSRAM | 1 075 |
| PSRAM streaming read | 69 per 16 B (55 MB/s) |

Two fixes:

**Requantization: 80 → ~20 cycles, exact.** TFLM's version is a
non-inlined call with a 64-bit division. Two identities make it cheap
(`pie_mbqm()` in `main/pie_kernels.h`):
- The rounding doubling high-multiply, `trunc((a·M + nudge) / 2³¹)` with
  a sign-dependent nudge, equals `floor((a·M + 2³⁰) / 2³¹)` for every
  product: one 32×32→64 multiply, an add and a shift.
- The final round-half-away-from-zero shift is
  `(x + 2^(e−1) − (x<0)) >> e`.

`det pie bench` compares it with TFLM's function on 400 000 random inputs
(0 mismatches), and verify mode checks every layer. It replaces the
per-output requantization in pointwise, im2col, depthwise, ADD and MUL.

**Cache blocking in pointwise convs.** The data cache is 32 KB. Going pixel
by pixel over all output channels re-reads every weight row for every
pixel, so a 256→256 layer (64 KB of weights) streamed its weights from
PSRAM per pixel, ~1 100 cycles per output. Output channels are now
processed in blocks whose weights fit in 16 KB, and all pixels pass
through one block before the next. Small layers are a single block.

| layer | before | after |
|---|---|---|
| 256→256 @6×6 (×7) | 47.5 ms | 9.5 ms |
| 256→256 @3×3 (×4) | 12.0 ms | 3.2 ms |
| 256→128 @12×12 | 42.7 ms | 18.8 ms |
| 128→256 @6×6 | 16.7 ms | 6.0 ms |
| 32→32 @48×48 (×3, requantization only) | 40.8 ms | 27.1 ms |
| pointwise total (29 layers) | 690 ms | 302 ms |
| depthwise total (23 layers) | 438 ms | 367 ms |
| ADD (25) | 261 ms | 144 ms |

### 7. Both cores (1.04 → 0.81 s)

Core 1 ran the inference while core 0 mostly idled (BLE, console). Now
every own kernel splits its work: `main/pie_parallel.cc` keeps one worker
task per core, and `pie_parallel_for(n, f)` runs half of `[0, n)` on the
caller's core and half on the other.
- Conv, first conv and depthwise split by output rows; ADD, PRELU and MUL
  by elements.
- Shared setup (tables, folded biases, the padded depthwise input) is done
  once before the split, and each half writes only its own outputs.
- PIE accumulators are per core, so the halves don't interfere. The first
  conv gets one gathered-row buffer per core.
- Workers run at priority 3: above the detector, below BLE, the console
  and the NimBLE host.
- `det pie cores 1|2` switches for A/B timing; `det pie` shows how the
  halves balanced.

| | 1 core | 2 cores | speed-up |
|---|---|---|---|
| depthwise (23) | 367 ms | 265 ms | 1.39× |
| pointwise + im2col (30) | 341 ms | 240 ms | 1.42× |
| ADD (25) | 144 ms | 124 ms | 1.16× |
| PRELU (26) | 57 ms | 51 ms | 1.12× |
| **inference** | 1.04 s | **0.81 s** | 1.28× |

**Why not 2×: memory.** The halves are balanced (each ~630 ms per
inference, the caller waits 3 ms in total). But the single-core time of
these ops is ~910 ms, so each core runs ~40% slower while the other one
is busy. Both share the cache and the PSRAM bus, and the activations live
in PSRAM. The memory-bound ops (ADD, PRELU: a table lookup per byte)
barely gain; the compute-heavy convolutions gain the most.

### 8. Operator fusion (0.83 → 0.78 s)

Fusing elementwise work into the producing op's output step is standard:
TFLite fuses ReLU/ReLU6 into conv (`fused_activation_function`),
converters fold BatchNorm into conv weights, and TensorRT, oneDNN
("post-ops"), XNNPACK and TVM fuse residual adds and activations. The
point is memory: each separate elementwise op is a full extra read and
write of a tensor.

This model's blocks are DEPTHWISE → CONV 1×1 → ADD(skip) → PRELU (23×),
plus 3 CONV → PRELU. In each, the intermediate tensors have exactly one
reader. With fusion (`main/pie_fusion.cc`), the conv's output step
computes per element:

```
c   = requant(conv)                       what CONV would store
a   = requant(r_skip[skip] + r_conv[c])   what ADD would store
out = prelu_table[a][channel]             what PRELU would store
```

It writes only `out`, so it's bit-exact with the separate ops.
- **Learning the chains:** the first inference runs unfused. ADD and PRELU
  record their tables in the producing conv's entry, but only if the
  intermediate tensor has exactly one consumer (counted from the model).
  From then on the conv does the chain, and ADD/PRELU return immediately.
- **Safety:** TFLM's memory planner may put the final tensor on top of
  the conv's input, which is still being read. The conv checks the real
  buffer addresses and fuses only if there's no overlap with its input,
  and either none with the skip tensor or exactly on it (in place, same
  index). 21 ADDs and 22 PRELUs fuse; the rest run separately.
- **Switches:** off in verify mode (that compares each op's own output);
  `det pie fuse on|off` for A/B.

| | unfused | fused |
|---|---|---|
| CONV_2D | 240 ms | 340 ms (now includes the ADD/PRELU work) |
| ADD + PRELU | 175 ms | 41 ms (only the unfused ones left) |
| inference (golden) | 830 ms | **781 ms** |
| live detector | 812 ms | **770 ms** |

**Smaller than expected (−6%).** Fusion only removes the memory passes.
Each element still needs the ADD requantization and two table lookups,
now inside the conv's loop. The passes it saves (one write and two
read/write pairs of a mostly-cached tensor per block) were a smaller part
of ADD/PRELU's 175 ms than the arithmetic.

### 9. Own MAX_POOL, PAD and RESIZE (0.78 → 0.66 s)

After the convolutions got fast, the three remaining TFLM reference ops
were 135 ms, 17% of the inference, for work that is almost pure memory
movement: onnx2tf leaves their input and output qparams equal, so there
is no requantization. The reference versions are generic N-d loops with
an `Offset()` call per element, on one core. `main/pie_data.cc`:

- **MAX_POOL_2D** (2×2, stride 2, VALID; 4 layers): per output pixel and
  16 channels, four 16-byte loads and three `ee.vmax.s8`. Same activation
  clamp as the reference (a no-op here, so the SIMD path requires it to
  be the full int8 range).
- **PAD** (8 layers): the model pads either H/W by (1, 2) before the
  stride-2 depthwise convs, or channels (32 → 64 etc.) on the residual
  skip path. Rows of padding are one `memset`, interior pixels
  `memset` + `memcpy` + `memset`. Pad value as TFLM: the output zero
  point.
- **RESIZE_BILINEAR** (2× up, half-pixel centers; 2 layers): TFLM's
  integer formula with source indices and 10-bit weights computed once
  per row and column, the per-channel sum in 32 bits (it can't overflow:
  |v| ≤ 128 and the absolute weights sum to < 1536²), and the same
  symmetric rounding.

All split across both cores and checked by `det pie verify on`.

| op | before | after |
|---|---|---|
| MAX_POOL_2D (4) | 55 ms | 3.5 ms |
| PAD (8) | 41 ms | 9.9 ms |
| RESIZE_BILINEAR (2) | 39 ms | 6.8 ms |
| inference (golden) | 781 ms | **663 ms** |
| live detector | 770 ms | **657 ms** |

The first version kept RESIZE's two 128-entry tap tables (4 KB) on the
stack and crashed: `det golden` runs on the console task, whose stack
is smaller than the detector's. They are static now.

## Current status

96×96, 0.66 s per inference (`det prof`, golden input):

| op | ms | % | kernel |
|---|---|---|---|
| CONV_2D (30) | 338 | 51.0 | PIE pointwise + im2col + fused ADD/PRELU, 2 cores |
| DEPTHWISE_CONV_2D (23) | 264 | 39.8 | PIE, 2 cores |
| ADD (25: 4 unfused) | 30 | 4.5 | LUT + requant, 2 cores |
| PRELU (26: 4 unfused) | 11 | 1.6 | LUT, 2 cores |
| PAD (8) | 9.9 | 1.5 | own, 2 cores |
| RESIZE_BILINEAR (2) | 6.8 | 1.0 | own, 2 cores |
| MAX_POOL_2D (4) | 3.5 | 0.5 | PIE, 2 cores |
| CONCAT, RESHAPE | 0.2 | 0.0 | reference |
| **total** | **663** | | |

Around it: grabbing and decoding a QVGA JPEG takes 80–170 ms, the letterbox
preprocessing 9 ms, and postprocessing 2 ms. A detection result arrives
about every 0.87 s.

## What's left

Ordered by expected payoff:

1. **Depthwise** (264 ms): a padded copy of every input, QACC write-out
   and 20-bit unpacking in C, 25 load pairs per 16 channels.
2. **Pointwise inner loop:** fused load+multiply
   (`ee.vmulas.s8.accx.ld.ip`), several outputs per input load, the
   zero-overhead `loop`.
3. **PSRAM traffic:** both cores share it (each runs ~40% slower when
   both work). Small late-layer tensors in internal RAM, or the 64 KB
   data cache.

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
