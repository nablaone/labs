# Talk notes: making a palm detector fast on an ESP32-S3

Material for a presentation about this optimization work. Each `##` section
is roughly one slide; the numbers are measured on the board unless marked
as an estimate. Details live in [performance.md](performance.md) and
[model-pipeline.md](model-pipeline.md).

---

## 1. The setup

- **Board:** DFRobot FireBeetle 2, ESP32-S3: 2 × Xtensa LX7 at 240 MHz,
  512 KB internal SRAM, 8 MB PSRAM, OV2640 camera.
- **Model:** MediaPipe palm detector (OpenCV zoo ONNX) → int8 TFLite →
  TensorFlow Lite Micro (TFLM), upstream reference kernels.
- **Rules we set ourselves:**
  - no vendor ML libraries (ESP-NN, ESP-DL); own kernels only
  - every optimization must stay **bit-exact** with the reference
    (`det golden` compares the raw output tensors with host LiteRT)
  - all TensorFlow tooling in a container

---

## 2. Result: 137 s → 0.48 s

| step | inference | gain |
|---|---|---|
| reference kernels, 192×192 input | 137 s | |
| input 192 → 96 | 33.8 s | 4× |
| PIE SIMD 1×1 conv | 11.7 s | 2.9× |
| PIE depthwise 5×5, im2col first conv | 3.9 s | 3.0× |
| lookup tables for elementwise ops | 2.6 s | 1.5× |
| native PRELU (fixed converter bugs) | 1.63 s | 1.6× |
| fast exact requantization + cache-blocked weights | 1.04 s | 1.6× |
| both CPU cores | 0.81 s | 1.28× |
| CONV→ADD→PRELU fusion | 0.78 s | 1.06× |
| own MAX_POOL / PAD / RESIZE kernels | 0.66 s | 1.18× |
| depthwise rework (measured first) | **0.48 s** | 1.39× |

**288× overall, 71× at the same input size, still bit-exact.**

---

## 3. Four levels of optimization

| level | what changes | examples here |
|---|---|---|
| **kernel** | how one op is computed (SIMD, loop order, memory layout) | PIE conv/depthwise, LUTs, fast requant, own pool/pad/resize |
| **graph** | how ops connect and run together; math unchanged | fusion, native PRELU (124 ops instead of ~300), 2-core split |
| **model** | the network itself; results change | 192 → 96 input; ideas: 64×64, gray, pruning, cascade |
| **system** | everything around inference | camera format, overlapping grab with inference, RAM placement |

Kernel and graph work are free in accuracy (bit-exact). Model changes
trade accuracy for speed and must be measured.

---

## 4. The same playbook as GPUs

| here | GPU world |
|---|---|
| hand-written PIE SIMD kernels | CUDA kernels, cuDNN, CUTLASS, Triton |
| PSRAM / internal SRAM / 32 KB cache | HBM / shared memory / registers |
| weight blocking (16 KB tiles stay in cache) | tiling a matmul into shared memory |
| CONV→ADD→PRELU fusion | kernel fusion in TensorRT, XLA, `torch.compile` |
| depthwise→1×1 fusion through SRAM (planned) | FlashAttention: tiles never leave on-chip SRAM |
| int8 with exact requantization | INT8 / FP8 inference |
| 2-core split per layer | thousands of cores, multi-GPU |
| `det golden` bit-exact check | testing kernels against a reference |
| cycles per output vs hardware peak | roofline analysis |

**The shared lesson: arithmetic is cheap, moving data is expensive.**

---

## 5. Where the time goes now (0.48 s)

| op | ms | % |
|---|---|---|
| CONV_2D (mostly 1×1, with fused ADD/PRELU) | 340 | 72 |
| DEPTHWISE_CONV_2D 5×5 | 73 | 15 |
| ADD (unfused rest) | 30 | 6 |
| PRELU (unfused rest) | 11 | 2 |
| PAD | 9.9 | 2 |
| RESIZE_BILINEAR | 7.0 | 1.5 |
| MAX_POOL_2D | 3.4 | 0.7 |

Every op with real cost runs on our own code. The 1×1 conv is now
the one big target.

---

## 6. Surprise: the time follows outputs, not math

The four 1×1 conv stages do **identical** arithmetic (resolution halves
per side, channels double), yet:

| layer | shape | MACs | outputs | time |
|---|---|---|---|---|
| 2 | 48×48, 32→32 | 2.36 M | 73,728 | 19.0 ms |
| 10 | 24×24, 64→64 | 2.36 M | 36,864 | 11.8 ms |
| 18 | 12×12, 128→128 | 2.36 M | 18,432 | 8.6 ms |
| 26 | 6×6, 256→256 | 2.36 M | 9,216 | 7.1 ms |

Each output value has a fixed cost (requantize, fused ADD/PRELU lookups,
store, loop setup). At 32 input channels its dot product is only 2 SIMD
instructions, so the overhead dominates. → The early, high-resolution
layers are the expensive ones, and optimization means **cutting the cost
per output**.

---

## 7. How far from the hardware limit?

Per inference: 69 M MACs (56.7 M conv + 12.8 M depthwise), 1.16 M conv +
depthwise outputs, ~6.2 MB activation traffic + 0.9 MB weights.

| limit | time |
|---|---|
| pure arithmetic (16 int8 MACs/cycle × 2 cores) | ~9 ms |
| memory (PSRAM ~55 MB/s) | ~100–130 ms |
| before the depthwise rework | 660 ms |
| **now** | **476 ms** |

- Measured before the rework: **~250 CPU cycles per output** in both conv
  kernels, against ~5–10 cycles of useful SIMD work. Depthwise is now
  ~65 cycles per output; the 1×1 conv is still ~250.
- Estimate: 2–3× more is realistic (≈ 200–300 ms) with better kernels;
  ~150–200 ms with depthwise→1×1 fusion through SRAM.

---

## 8. Using both cores

```
core 1: detector ──[grab+decode]──[pre]──┬ layer 0, first half ┬ layer 1, first half ┬ ...
                                         │ notify               │ wait                │
core 0: worker   (idle) ─────────────────└ layer 0, second half ┘ layer 1, second half ┘
```

- Fork-join per layer: the caller does [0, n/2), a worker pinned to the
  other core does [n/2, n). Each half writes only its own outputs.
- 75 splits per inference; halves 622 vs 613 ms; the caller waits 3 ms in total.
- Yet only **~1.4×**, not 2×: both cores share one cache and the PSRAM
  bus, so each runs ~40% slower while the other works.

---

## 9. Operator fusion

The model's blocks: DEPTHWISE → CONV 1×1 → ADD(skip) → PRELU (23×).
Fused, the conv's output step computes per element:

```
c   = requant(conv accumulator)          what CONV would store
a   = requant(r_skip[skip] + r_conv[c])  what ADD would store
out = prelu_table[a][channel]            what PRELU would store
```

- Chains are learned on the first inference; only single-consumer
  intermediates fuse, and a runtime check rejects buffer overlaps from
  TFLM's memory planner.
- Gain only 6%: fusion removes memory passes, but the per-element work
  (requant + 2 lookups) remains, now inside the conv loop.

---

## 10. Experiment: tensors in internal RAM (not kept)

Idea: move TFLM's activation arena from PSRAM into the fast on-chip SRAM.

- TFLM can split persistent data and activations into two arenas.
- At 96×96 the activations need **≥ 216 KB contiguous** (three 48×48×32
  tensors live at once); the largest free internal block is **168 KB**.
- Freed 70 KB of our own `.bss` (a lookup-table array) → 260 KB free,
  still not contiguous enough.
- 64×64 stand-in fits: 278 ms with internal activations (the PSRAM
  comparison run was not done).
- Activations and the depthwise scratch buffer compete for the same RAM.

Lesson: on this chip, keeping hot data in SRAM must come from the
kernels (tiling, fusion), not from moving the whole arena.

---

## 11. Ideas examined and what the data said

**Cascade (cheap first stage, full model to confirm)**
- Splitting the network in half doesn't work: no score mid-network, and
  the first half is the expensive one.
- Viable: the same weights at 64×64 as a gate (~280 ms vs 660 when no
  hand; ~940 ms when a hand is in view), a motion gate, or tracking.

**Gray input instead of RGB**
- Only the input tensor shrinks (27 KB → 9 KB) and the first conv
  (~31 → ~10 ms); every later tensor has 32–256 channels regardless.
- First-layer weights: 46% of the weight energy is in color
  differences; 7 of 32 filters are pure color opponents (e.g. R vs G,
  corr −0.99), only 6 are nearly brightness-only. → The model uses
  color; gray input would need an accuracy check.

**Ditching or averaging weights without training**
- Removing a channel shifts every downstream sum; errors compound over
  ~50 layers and residual connections tie channels together across blocks.
- Exact without training only for dead, constant or duplicate channels;
  how many exist here is unmeasured.

**JPEG vs raw camera frames**
- JPEG grab 20 ms + ~60 ms CPU decode; raw RGB565/gray 40 ms by DMA, no
  decode. A cheap system-level win.

---

## 12. Depthwise: measure before optimizing

264 ms, ~230 cycles per output value. Per-phase cycle counters showed:

| phase | before | after |
|---|---|---|
| padded copy of the input | 34 ms, one core, **in PSRAM** | ring of 5 rows per core in SRAM: 9 c/o |
| 25-tap multiply loop | 53 c/o | 15 c/o (fused multiply + load) |
| unpack + requantize + store | **160 c/o** | 40 c/o |
| **depthwise total** | **264 ms** | **73 ms** |

The two surprises, both invisible without measuring:
- The 86 KB padded buffer **silently fell back to PSRAM**: no internal
  block was that big.
- A 4-byte `memcpy` used to unpack each 20-bit accumulator lane compiled
  to **a real function call, 16 times per group**, and the inlined output
  step spilled most of its values to the stack.

The SIMD multiply loop was never the main problem; the scalar code around
it was. Fixing it: 0.66 → 0.48 s, still bit-exact.

---

## 13. Pitfalls worth telling

- **Bit-exact means matching the reference's rounding exactly:** TFLM's
  requantization is double-rounding, and our 10-cycle replacement had to
  match it (checked on 400k random inputs, 0 mismatches).
- **Converter bugs:** onnx2tf's direct flatbuffer writer gave a wrong
  quantization axis for depthwise biases and mismatched qparams on
  pass-through ops; fixed in our conversion script.
- **A 4 KB stack array crashed the board**: the console task (which runs
  `det golden`) has a small stack. Moved to static.
- **Measuring with the detector still running** mixed two inferences'
  stats; always `det off` first.
- **Verify mode** (`det pie verify on`) reruns the reference after every
  layer and compares bytes. It caught issues the end-to-end check would
  hide.

---

## 14. Next steps

1. **1×1 conv** (340 ms, 72%): same method. Measure phases first;
   likely a scalar output step (requant + fused ADD/PRELU per value)
   around a short dot product. Then: 16 output channels per QACC pass,
   fused load+multiply, output step out of line.
2. **Depthwise→1×1 fusion** in row tiles through internal RAM.
3. **System:** RGB565 instead of JPEG; overlap grab with inference.
   Today a result arrives every ~0.56 s.
