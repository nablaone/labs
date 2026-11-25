# riscv lab log

## 2026-11-25 — start, picking a reference core

Goal of the lab: learn RISC-V by reading and building a core in Verilog,
move it to an FPGA later, then add custom opcodes for neural inference.

State: empty directory. No HDL tools installed yet (iverilog, verilator,
yosys and a RISC-V GCC all missing).

Candidate cores looked at (GitHub stars and licence as of today):

FemtoRV32 Quark, from BrunoLevy/learn-fpga (3.8k stars, BSD-3-Clause).
https://github.com/BrunoLevy/learn-fpga
File: FemtoRV/RTL/PROCESSOR/femtorv32_quark.v, 420 lines, RV32I, one file.
The repo also has the "From Blinker to RISC-V" tutorial
(FemtoRV/TUTORIALS/FROM_BLINKER_TO_RISCV), which builds a core step by
step. Simplest thing to read end to end. Chosen as the first core.

PicoRV32, from YosysHQ/picorv32 (4.4k stars, ISC).
https://github.com/YosysHQ/picorv32
File: picorv32.v, about 95 KB, RV32IMC, heavily configurable, battle-tested.
Has the PCPI (Pico Co-Processor Interface): undecoded instructions are
handed to an external module, which is exactly the hook needed for custom
neural-inference opcodes. Likely the base for step 4.

SERV, from olofk/serv (1.9k stars, ISC).
https://github.com/olofk/serv
Bit-serial RV32I, the smallest core around, split over about 18 files.
Interesting for area, slow, and harder to follow as a first core.

DarkRISCV, from darklife/darkriscv (2.6k stars, BSD-3-Clause).
https://github.com/darklife/darkriscv
Small pipelined RV32I/E, lots of FPGA board ports. Possible reference
when picking a board.

Decision: read and simulate FemtoRV32 Quark first, keep PicoRV32 in mind
for the custom-opcode phase.

Next: install icarus-verilog, verilator, gtkwave, yosys and a RISC-V GCC;
vendor femtorv32_quark.v; write a minimal testbench with a ROM running a
few instructions and look at the waveforms.

## 2026-11-25 — Docker toolchain, hello on QEMU and on FemtoRV32 Quark

Core: FemtoRV32 Quark, vendored unchanged into vendor/femtorv32/ from
BrunoLevy/learn-fpga commit f5ada24ae6cca67d99889daec2c4ee44a6e59417,
file FemtoRV/RTL/PROCESSOR/femtorv32_quark.v plus the repo LICENSE
(BSD-3-Clause).

Toolchain: Docker image riscv-lab, built from docker/Dockerfile on
ubuntu:24.04 (arm64 host), 1.01 GB. Versions inside:
Icarus Verilog 12.0
Verilator 5.020
riscv64-unknown-elf-gcc 13.2.0 (Ubuntu package, used with
-march=rv32i -mabi=ilp32; its multilib libgcc for rv32i links fine)
QEMU 8.2.2, qemu-system-riscv32

Memory map, the same for QEMU virt and the testbench:
RAM 0x80000000, 64 KB (QEMU loads -kernel there when -bios none)
UART THR 0x10000000 (write a byte, it gets printed)
exit device 0x00100000 (sifive_test: 0x5555 means pass,
(code<<16)|0x3333 means fail)

Gotcha: Quark's default ADDR_WIDTH is 24, which would alias 0x10000000
to 0x0 and 0x80000000 to 0x0. The testbench sets ADDR_WIDTH=32 and
RESET_ADDR=0x80000000.

Program: sw/hello, made of crt0.S (stack, zero .bss, call main, report
the exit code) and hello.c (prints through the UART, times one puts with
rdcycle). It's 329 bytes of text.

Testbench: sim/tb_femto.v, synchronous RAM loaded with $readmemh from
build/hello.hex (made with objcopy -O binary, then od -tx4, one
little-endian word per line). make sim SIMARGS=+vcd dumps build/tb.vcd.

Results:
make qemu prints "hello from rv32i", with rdcycle 0x15261 for the line.
That number is meaningless, because QEMU TCG doesn't model cycles.
make sim prints "hello from rv32i", with rdcycle 0x13a = 314 cycles for
17 characters, and the whole program PASSes in 1190 cycles. Quark takes
3 to 4 cycles per instruction (fetch, wait, execute, plus a wait state
for loads, stores and shifts), and shifts cost 1 cycle per bit.
make lint shows only one Verilator warning, DECLFILENAME (the file name
differs from the module name).

Gotcha: host make thought sw and sim were up to date because
directories with those names exist. Fixed by marking them .PHONY on the
host side of the Makefile.

Next: look at the waveform (make sim with +vcd, GTKWave on the host),
walk through one instruction per state of the Quark FSM, then write a
small benchmark (int8 dot product) as the baseline for custom opcodes.
