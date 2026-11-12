# Plan: own runtime and converter (replace TFLM)

Status: **plan, not started** (2026-11-12). Pick up from "Next session" at
the bottom.

## Why

The original toolchain comparison (option D) proposed our own converter
and runtime next to the TFLM baseline. What exists today is half of it:
TFLM still loads the model, plans memory, computes quantization
parameters (Prepare) and calls the ops in order. All ops with real cost
run our own code (`main/pie_*.cc`), and that took inference from 137 s
to 0.36 s ([performance.md](performance.md)).

The remaining TFLM half now blocks the next gains:

| limit today | caused by | own runtime |
|---|---|---|
| activations can't use internal RAM: one contiguous ≥ 216 KB arena, largest SRAM block 160 KB | TFLM plans one arena for the whole graph | place each tensor: small/late ones in SRAM, big early ones in PSRAM |
| no DEPTHWISE→1×1 fusion through SRAM row tiles | TFLM runs whole ops, one at a time | run both ops interleaved, a few rows at a time |
| 4 of 25 ADDs can't fuse (30 ms) | TFLM's planner may overlap the buffers fusion needs | our planner knows the fused chains and keeps buffers apart |
| fusion learned at the first inference, checked at runtime | bolted on around TFLM (`pie_fusion.cc`) | planned once in the converter |
| PAD before every stride-2 depthwise (two copies) | separate op in the graph | folded into the depthwise padding by the converter |
| weight repacking and requant params computed at boot | TFLM Prepare + our first-call setup | precomputed by the converter |

Expected: **358 → ~250–280 ms** (estimate, mostly SRAM placement and
DEPTHWISE→1×1 fusion). Plus: no interpreter, smaller firmware, and full
understanding of every stage from model file to output, which was the
point of option D.

## Ground rules (same as so far)

- **Bit-exact with TFLM/LiteRT reference kernels.** `det golden` must
  keep printing BIT-EXACT; the own engine is checked against TFLM tensor
  by tensor while both exist.
- All Python/TF in the `handdetector-model` container.
- No Espressif ML libraries; our PIE kernels.
- TFLM stays in the firmware until the own engine matches and is
  faster: `det engine tflm|own` switches, for A/B timing.

## Architecture

```
model_int8.tflite ──▶ converter (container, Python) ──▶ model.hdm (flat binary)
                        - read flatbuffer                    │ esptool, model partition
                        - fold PAD, plan fusions             ▼
                        - lifetimes, SRAM/PSRAM placement   runtime (firmware, C++)
                        - pack weights, requant params        - mmap blob
                                                              - allocate 2 pools
                                                              - run step list with pie kernels
```

### Converter input: the int8 `.tflite`, not ONNX

Its quantization (scales, zero points, per-channel filter scales) is
exactly what `det golden` was generated from. Re-quantizing from ONNX
would mean starting the validation over. The converter reads it with
`tensorflow.lite.python.schema_py_generated` (already used in
`model/pipeline/convert.py` and our analysis scripts).

### The model: 124 ops, 9 kinds

CONV_2D 30 (29 pointwise + the first 5×5 stride 2), DEPTHWISE_CONV_2D 23
(5×5), ADD 25, PRELU 26, PAD 8, MAX_POOL_2D 4, RESIZE_BILINEAR 2,
RESHAPE 4, CONCATENATION 2. Typical block: [PAD] → DEPTHWISE → CONV 1×1 →
ADD(skip) → PRELU; the skip path has MAX_POOL and a channel PAD at
stride-2 blocks. RESHAPE and CONCAT only at the output heads.

### Requantization parameters must match TFLM exactly

TFLM computes them at Prepare from float scales:
- conv/depthwise: per channel, `effective = (double)input_scale *
  filter_scale[c] / output_scale`, then `QuantizeMultiplier`
  (`std::frexp`, then `TfLiteRound(q * 2^31)`: round half **away from
  zero**; Python's `round()` is banker's rounding, so use
  `math.floor(x + 0.5)` for x ≥ 0).
- ADD: TFLM's `left_shift = 20`, twice-max-input-scale scheme
  (`micro/kernels/add_common.cc`), input multipliers via
  `QuantizeMultiplierSmallerThanOneExp`.
- PRELU: `real_multiplier_1 = input_scale / output_scale`,
  `real_multiplier_2 = input_scale * alpha_scale / output_scale`
  (`micro/kernels/prelu_common.cc`).
- Mind float32 vs double: TFLM stores scales as float and widens to
  double in specific places; replicate the same casts.

**Check:** a firmware command `det dump params` prints TFLM's computed
multipliers/shifts per layer (from `OpDataConv`, `OpDataAdd`,
`PreluParams`). The converter's values must match them 1:1 before
anything else is trusted.

### Binary format `model.hdm` (sketch)

```
header   magic "HDM1", version, counts, offsets, input/output tensor ids
tensors  [id] shape (N,H,W,C), dtype, zero point, pool (SRAM|PSRAM), offset, size
steps    [i] kind, in/out tensor ids, params offset, weights offset
params   per step: strides, padding, requant tables (DwChan-style: bias,
         mult, half, ls, rs, nz per channel), ADD tables (r_skip/r_conv),
         PRELU params (alpha + offset, m1/s1/m2/s2), fusion flags
weights  pre-packed: pointwise [oc][k16] (+16 B tail), depthwise
         [group][tap][16], 16-byte aligned
```

All offsets 16-byte aligned. Weights stay in flash (mmap) or are copied
to PSRAM at boot, as now. Place the blob in the `model` partition next
to the `.tflite` (e.g. at offset 0x140000; the golden vectors stay at
0x300000), so both engines can run from one flash image.

### Runtime

- `main/engine.cc`: mmap the blob, check magic/version, allocate the
  SRAM pool and the PSRAM pool, resolve tensor pointers, run the steps.
- Step kinds map 1:1 onto kernels; a fused chain is one step.
- No dynamic allocation per inference; scratch (depthwise row rings,
  im2col rows) sized by the converter.
- Same stats as now (`det pie stats` style: per step time, phase
  counters).

### Memory plan

- Lifetimes from the step order (after fusion, so fused intermediates
  have no lifetime at all).
- Two pools: SRAM (budget set by the converter, e.g. 96–128 KB, leaving
  room for NimBLE, stacks, depthwise rings) and PSRAM (the rest).
- Greedy placement: decide per tensor which pool, then offsets inside
  each pool with TFLM's greedy-by-size algorithm. First heuristic: put a
  tensor in SRAM if it fits the budget for its whole lifetime, preferring
  most bytes read per byte stored (the skip tensors of residual blocks
  are read twice).
- Fused chains: the planner guarantees the final output doesn't overlap
  the conv input or the skip tensor (or sits exactly on the skip,
  in place), so all 25 ADDs can fuse.
- Report: peak per pool, which tensors went to SRAM, printed by the
  converter and by `det`.

### Fusions planned by the converter

1. CONV → ADD → PRELU and CONV → PRELU (as now, but planned).
2. PAD (spatial, before a stride-2 depthwise) folded into the
   depthwise's padding; PAD (channel, on the skip path) folded into the
   ADD (the extra channels are zero point → ADD with only the conv side).
3. Later: DEPTHWISE → CONV 1×1 (+ADD+PRELU) in row tiles: the depthwise
   writes K rows into an SRAM tile, the 1×1 consumes them, the full
   depthwise output never exists in PSRAM.

## Kernel refactor (prerequisite)

Today's kernels take TFLM types (`TfLiteEvalTensor`, `OpDataConv`,
`TfLiteNode`). Split each into a TFLM-free core + a thin TFLM wrapper:

```
pie_core.h   pw_conv(const PwParams&, const int8_t *in, int8_t *out, int pixels, const Epi*)
             dw_conv(const DwParams&, const int8_t *in, int8_t *out, ...)
             im2col_conv(...), add(...), prelu(...), pad(...), max_pool(...), resize(...)
```

The TFLM wrappers fill the params structs from TFLM's op data and call
the cores; the own engine fills them from the blob. Same code path for
both engines, so speed comparisons are fair and bit-exactness carries
over.

## Validation

1. `det dump params` vs converter output: identical multipliers, shifts,
   offsets per layer.
2. Host side: the converter can also run its plan in numpy with the same
   integer arithmetic and compare with LiteRT's intermediate tensors
   (`experimental_preserve_all_tensors=True`), before touching the board.
3. On the board: `det engine compare` runs TFLM and the own engine on
   the golden input and compares every intermediate tensor (map by
   tflite tensor index; fused intermediates skipped).
4. `det golden` BIT-EXACT with `det engine own`.

## Phases

| phase | content | done when |
|---|---|---|
| 0 | kernel refactor: TFLM-free cores + wrappers | golden bit-exact, same speed |
| 1 | `det dump params`; converter reads tflite, computes params | params match 1:1 |
| 2 | converter v1: no fusion, one PSRAM pool, same op order; runtime v1; `det engine` switch; `det engine compare` | golden bit-exact with own engine; speed ≈ TFLM |
| 3 | fusions in the converter (CONV+ADD+PRELU, PAD folding); planner without the overlap problem | all 25 ADDs fused; faster than TFLM path |
| 4 | SRAM/PSRAM placement | measured gain per placement heuristic |
| 5 | DEPTHWISE→1×1 row-tile fusion | measured gain |
| 6 | cleanup: TFLM optional in the build; docs, talk notes | |

Each phase ends with numbers in [performance.md](performance.md), like
the steps so far.

## Risks

- **Requant parameter mismatch** (float/double rounding details): caught
  by phase 1 before anything runs.
- **SRAM budget vs NimBLE/camera:** internal RAM is ~257 KB free at
  init, largest block ~160 KB; the plan must leave the depthwise rings
  (2 × ≤ 13 KB) and the BLE stack room. Make the budget a converter flag
  and log free RAM after init.
- **Scope creep:** only the 9 op kinds this model uses; anything else
  is a converter error, not a runtime fallback.

## Next session

Start with phase 0 on `main/pie_conv.cc` (pointwise first: `pw_conv`
core taking a plain params struct; `ConvEval` becomes a wrapper), check
`det golden` and the timings, then the same for depthwise, im2col and
the elementwise/data ops. Then phase 1: `det dump params` and the
converter skeleton in `model/pipeline/hdm.py`.
