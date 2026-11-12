# Plan: autotuning the inference schedule (search instead of hand-picking)

Status: **plan, not started** (2026-11-12). Runs alongside
[own-runtime-plan.md](own-runtime-plan.md), and ties into its phases.

## Why

Every optimization so far was a hand-made choice of one variant for
the whole network. Several looked promising and were rejected after
measurement, and in each case it depended on the surroundings:

| tried | alone | in context |
|---|---|---|
| PRELU as a `[256][C]` table | fast (standalone PRELU) | slower inside the fused conv step: 64 KB table evicts the weights from the 32 KB cache |
| split output channels between cores | halves the weight stream per core | 2–3× slower on 6×6/3×3 layers: two weight blocks + inputs don't fit the cache |
| activations in internal RAM | ~1 cycle access | doesn't fit at 96×96; at 64×64 it pushes the depthwise scratch to PSRAM |
| fusing ADD into conv | saves a pass over the tensor | 4 of 25 can't fuse: TFLM's buffer overlap |

The choices interact through the cache, the internal RAM budget and
the buffer plan. Coordinate-by-hand doesn't scale to 124 ops × several
variants × placement × fusion boundaries. So: make the choices explicit
as a **genome**, measure candidates on the board, and search.

What search does **not** do: invent kernels. The big steps (PIE,
native PRELU, the reworks) were new code. Search picks among existing
variants and their parameters, and shows which variants never win, so
they can be deleted (less code, not more).

## Approach (option C)

Two tracks, sharing one protocol and one driver:

1. **Now, on TFLM:** expose today's variants as per-layer runtime knobs.
   Per-layer exhaustive tuning, then a small GA over the coupled genes
   (fusion, core split). Comes for free with own-runtime phase 0, which
   turns the kernels into TFLM-free cores selected by plain params.
2. **After own-runtime phase 3/4:** the converter takes a
   `schedule.json` (the genome) next to the `.tflite`. Placement,
   fusion chains and tiling become real choices; the GA searches them.
   The hand-picked schedule is the default `schedule.json` and the
   baseline every run is compared with.

## Measurement speed decides the design

| candidate applied by | per candidate | per hour |
|---|---|---|
| firmware rebuild (Docker) + flash + run | ~60 s | ~60 |
| new blob in the `model` partition | ~10–15 s | ~300 |
| **config over serial, no flash** | **~2–3 s** (5 runs + golden) | **~1500** |

Rule: all variants are compiled in; a candidate is a config sent at
runtime. A flash only happens when the code changes, never per
candidate. For track 2 the blob can be uploaded into PSRAM over serial
(`det tune load`) instead of flashed.

## Genome

**Per op** (124 ops, genes only where the op kind has alternatives):

| gene | values | applies to |
|---|---|---|
| `kernel` | variant id (e.g. dot loop: register-resident pixel / streamed; output step: generic / specialized) | CONV, DEPTHWISE |
| `prelu` | table / arithmetic | PRELU, fused PRELU |
| `fuse` | none / +ADD / +ADD+PRELU / +PRELU | CONV 1×1 |
| `split` | pixels / channels / one core | CONV, DEPTHWISE, ADD |
| `block` | cache block size (output channels per weight block) | CONV 1×1 |
| `place` | SRAM / PSRAM (track 2) | output tensor |
| `tile` | rows per tile, or none (track 2) | DEPTHWISE→1×1 |

**Global:** SRAM budget, depthwise ring size, PIE scratch placement.

Encoding: one record per op, grouped by **residual block**
([PAD] → DW → 1×1 → ADD → PRELU). Genes that interact sit in the same
block.

## Operators

- **Crossover:** at block boundaries only (uniform over blocks). A
  block that works keeps its fusion + placement + split together.
- **Mutation:** per gene, low rate; plus a block-level mutation that
  re-draws a whole block from the per-layer best list.
- **Repair, not penalty:** an invalid genome (SRAM over budget, buffer
  overlap that blocks a fusion, a variant that doesn't apply to the
  shape) goes through the planner, which fixes it deterministically
  (demote the largest SRAM tensor to PSRAM, unfuse, fall back to the
  generic variant). Only valid candidates are measured.
- **Seeding:** the hand-picked schedule, the per-layer winners, and
  random valid genomes.
- **Selection:** tournament; elitism keeps the baseline in the
  population so a run can never report something worse than today.

## Fitness

Lexicographic, in this order:

1. `det golden` **bit-exact**: hard filter, never a score. A failure is
   a bug in a variant, logged with the genome so it can be reproduced.
2. SRAM peak ≤ budget (guaranteed by repair, checked anyway).
3. **Median** of N inferences (N = 5, more for the final candidates).

Noise control: camera DMA off (`cam off`; frames into PSRAM cost up to
32 ms), detector task stopped, same input every time, report the spread.
A candidate within the noise of the best one is re-measured before it
replaces it.

Optional multi-objective view (NSGA-II): latency × SRAM peak × number
of distinct variants used. The Pareto front is the answer to "how much
complexity / memory buys how many ms", the tension behind the rejected
experiments.

## Search, in stages

1. **Per-layer exhaustive.** Each op's variants measured in place, the
   rest of the network fixed (`det pie stats` per-layer times). Cheap
   (~124 × few) and finds most of the additive gain.
2. **GA over coupled genes**, seeded from stage 1: fusion, split,
   placement, tiles. Population ~32, ~50 generations ≈ 1600 candidates
   ≈ 1 h on the board.
3. **Surrogate model** (only if measurement becomes the bottleneck):
   fit per-op time from the measured candidates (features: shape,
   variant, placement, neighbour placement), screen offspring with it,
   measure only the top ones. This is how TVM's Ansor works.

## Pieces

| where | what |
|---|---|
| `main/autotune.c` | `det tune` CLI: `set <op> <gene>=<v>`, `load` (config/blob over serial), `run N` → median/min/max µs + golden result, `dump` (current schedule), `reset` |
| `main/pie_*.cc` | variant tables selected per op from the schedule instead of compile-time choices |
| `model/tune/` | driver (pyserial): genome ↔ config, GA, repair, results log (`runs/<date>.jsonl`, every candidate with its genome and timings) |
| `model/pipeline/hdm.py` | track 2: reads `schedule.json` |

The driver talks USB, so it runs on the host like `capture` and
`flash-golden`; GA logic is plain Python (no TF). The repair needs the
planner, which lives in the converter: share that module, run it in
the container if it pulls in TF.

Wire protocol for `det tune run`: one line, machine-readable, e.g.
`TUNE ok=1 n=5 med=358123 min=357900 max=358610 sram=0 psram=291k`.

## Phases

| phase | content | done when |
|---|---|---|
| T0 | `det tune` CLI with today's variants as runtime knobs (with own-runtime phase 0) | `det tune run` reproduces today's 358 ms; golden bit-exact for every knob value |
| T1 | driver + per-layer exhaustive search | per-layer winners table in performance.md; gain vs hand-picked |
| T2 | GA over fusion / split / block | measured gain or a clear "no gain" (also a result) |
| T3 | track 2: `schedule.json` in the converter, placement + tile genes (after own-runtime phase 4) | GA on the own engine beats the hand-picked own-engine schedule |
| T4 | prune: drop variants no winning schedule uses | smaller code, same speed |
| T5 | optional: surrogate, Pareto front | |

## Emulator?

Short answer: **not for fitness, maybe for correctness, and a host
build of the cores is the better option for that.**

- **Espressif QEMU** (`idf.py qemu`, ESP32-S3 supported since IDF 5.x)
  is not cycle-accurate: no 32 KB data cache model, no PSRAM latency,
  no bus contention between cores. Every effect that made the rejected
  experiments lose (cache eviction, shared weight blocks, PSRAM traffic)
  is invisible there. Timings from it would steer the search wrong.
- Whether it executes the **PIE `ee.*` instructions** at all is
  unverified (not mentioned in its docs). Test with a tiny program
  (`ee.vmulas.s8.accx` on known vectors) before relying on it.
- **Host build of the cores** (after own-runtime phase 0): the
  TFLM-free kernel cores with a scalar C implementation of each PIE
  primitive (bit-exact by construction, same accumulator widths) compile
  on the Mac. That checks every variant × shape combination against
  LiteRT reference kernels in milliseconds, so the board only ever sees
  candidates that are already correct. The asm itself still needs the
  board (`det pie verify on`).

So: correctness pre-filter on the host, fitness only on the board.

## Risks

- **Search finds noise.** Differences of a few ms are within run-to-run
  spread; re-measure before accepting, report medians with spread.
- **Overfitting to the golden input.** Timing barely depends on the
  data (no data-dependent branches in the kernels), but check the final
  schedule on live camera frames too.
- **Variant bugs found by search.** The GA will hit combinations nobody
  tried by hand. Golden + verify mode catch them; the log keeps the
  genome to reproduce.
- **Effort vs gain.** If T1 shows < 5% over hand-picked, stop at T1 and
  put the time into new kernels instead.

## Next session

Own-runtime phase 0 first (kernel cores), with T0 alongside: while
splitting `pw_conv` out of `ConvEval`, make its variant choice a
per-layer parameter, add `det tune set/run`, and check that `det tune
run` matches today's numbers.
