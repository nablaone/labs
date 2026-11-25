# riscv

Learning lab for RISC-V CPU design in Verilog. The path, roughly in order:

1. Read and simulate a small existing RV32I core: FemtoRV32 Quark, vendored
   in `vendor/femtorv32/`.
2. Write our own core, or fork one, and run it in simulation.
3. Put it on an FPGA (board not chosen yet).
4. Add custom opcodes for neural inference (MAC, dot product, int8 SIMD,
   activation functions) using the RISC-V custom-0..3 opcode space, and
   measure the speedup against plain RV32I code.

Conventions:

- Lab log lives in `LOG.md`, one shared log for all experiments. Add a
  dated entry (`## YYYY-MM-DD — [NN] title`, where NN is the experiment)
  for every session: what was tried, the commands, the numbers, what
  worked, what didn't, the next step. Write it as plain prose and lists,
  never tables. Newest entry at the bottom.
- Layout: one numbered directory per experiment in `exp/NN-name/`, plus
  shared code in `common/`. An experiment holds only what is specific to
  it: `rtl/` (its own copy of the CPU, even when unchanged, so every
  variant can be read on its own and `diff -r exp/01-* exp/02-*` shows
  exactly what changed), `sim/tb.v` (CPU wired to `common/sim/soc.v`),
  `sw/` (programs only it can run), `exp.mk` (RTL files, defines, program
  list, `-march`) and `README.md`. The README's first line is
  `# NN-name: one-line description` (`make list` shows it), followed by
  what it was derived from, what changed, and the results.
- `common/`: `sim/soc.v` (lab bench with clock, reset, RAM, UART probe
  and exit device on the FemtoRV bus; a CPU with another bus gets an
  adapter in its own `sim/`), `sw/rt/` (crt0.S, link.ld, lib.h), shared
  programs in `sw/<prog>/`, make rules in `mk/rules.mk`.
- Programs are looked up in `exp/NN/sw/<prog>` first, then
  `common/sw/<prog>`.
- New experiments start with `make new E=NN FROM=MM NAME=short-name`.
  Never edit a committed experiment to try something new: make a new one.
- Changes to `common/` must not silently change the results of older
  experiments. Run `make compare` before and after, and if a number
  moves, explain why in LOG.md.
- Third-party cores go in `vendor/<name>/`, pristine, with their upstream
  URL and commit hash noted in LOG.md. Experiments copy from there.
- 32-bit only: RV32I, `-march=rv32i -mabi=ilp32`. Don't add RV64 targets.
- All tools run in the `riscv-lab` Docker image (Ubuntu 24.04, defined in
  `docker/Dockerfile`): Icarus Verilog, Verilator, riscv64-unknown-elf GCC
  (used in rv32 mode), qemu-system-riscv32. Host `make <target>` re-runs
  itself inside the container (`IN_DOCKER=1`). Targets: `image`, `shell`,
  `list`, `run E=NN [P=prog] [SIMARGS=+vcd]`, `lint E=NN`, `qemu P=prog`,
  `compare`, `new`, `clean`. Output goes to `build/<exp>/<prog>/`
  (gitignored).
- One memory map shared by QEMU `virt` and the testbench, so the same ELF
  runs on both: RAM at 0x80000000 (64 KB), UART TX byte at 0x10000000, and
  the exit device at 0x00100000 (sifive_test: 0x5555 means pass,
  `(code<<16)|0x3333` means fail). Keep new peripherals consistent with
  QEMU virt where possible.
- Quark has no CSRs, traps or ecall. Any SYSTEM opcode returns the cycle
  counter, so firmware must stay bare-metal (see `common/sw/rt/crt0.S`).
- It's a learning project. Explain the hardware reasoning in comments and
  docs (datapath, pipeline stages, timing) rather than just writing code.
  The user has an EE/DSP background, so signal-flow and block-diagram
  explanations work better than algebra.
- The labs repo is public: no private hostnames or LAN IPs.
