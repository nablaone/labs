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

- Lab log lives in `LOG.md`. Add a dated entry (`## YYYY-MM-DD — title`)
  for every session: what was tried, the commands, the numbers, what
  worked, what didn't, the next step. Write it as plain prose and lists,
  never tables. Newest entry at the bottom.
- Third-party cores go in `vendor/<name>/` with their upstream URL and
  commit hash noted in LOG.md. Don't edit them in place: copy into `rtl/`
  before changing anything.
- Our own RTL in `rtl/`, testbenches in `sim/`, firmware and test programs
  in `sw/`, notes in `docs/`.
- 32-bit only: RV32I, `-march=rv32i -mabi=ilp32`. Don't add RV64 targets.
- All tools run in the `riscv-lab` Docker image (Ubuntu 24.04, defined in
  `docker/Dockerfile`): Icarus Verilog, Verilator, riscv64-unknown-elf GCC
  (used in rv32 mode), qemu-system-riscv32. Host `make <target>` re-runs
  itself inside the container (`IN_DOCKER=1`). Targets: `image`, `sw`,
  `qemu`, `sim`, `lint`, `shell`, `clean`. Output goes to `build/`
  (gitignored).
- One memory map shared by QEMU `virt` and the testbench, so the same ELF
  runs on both: RAM at 0x80000000 (64 KB), UART TX byte at 0x10000000, and
  the exit device at 0x00100000 (sifive_test: 0x5555 means pass,
  `(code<<16)|0x3333` means fail). Keep new peripherals consistent with
  QEMU virt where possible.
- Quark has no CSRs, traps or ecall. Any SYSTEM opcode returns the cycle
  counter, so firmware must stay bare-metal (see `sw/hello/crt0.S`).
- It's a learning project. Explain the hardware reasoning in comments and
  docs (datapath, pipeline stages, timing) rather than just writing code.
  The user has an EE/DSP background, so signal-flow and block-diagram
  explanations work better than algebra.
- The labs repo is public: no private hostnames or LAN IPs.
