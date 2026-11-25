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

## 2026-11-25 — [01] Docker toolchain, hello on QEMU and on FemtoRV32 Quark

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

## 2026-11-25 — restructure: numbered experiments and a common bench

Why: there will be many CPU variants, and every one should stay readable
and runnable side by side.

New layout. exp/NN-name/ holds one experiment: its own copy of the CPU in
rtl/, a thin testbench in sim/tb.v, its own programs in sw/, exp.mk and
README.md. common/ holds what all experiments share: sim/soc.v (clock,
reset, RAM, UART probe, exit device), sw/rt/ (crt0.S, link.ld, lib.h
with putc_/puts_/puthex/rdcycle), the shared programs (sw/hello) and the
make rules (mk/rules.mk).

The first experiment is exp/01-femto-quark, a copy of
vendor/femtorv32/femtorv32_quark.v, unchanged.

Make: make list, make run E=01 [P=hello] [SIMARGS=+vcd], make lint E=01,
make qemu P=hello, make compare (one line per experiment and program),
make new E=NN FROM=MM NAME=x.

Changes in the bench along the way:
The hex image is now padded to the full 64 KB (objcopy --pad-to), so
$readmemh no longer warns about a short file.
The bench takes +maxcycles=<n>, default 1000000.
The linker warning about an RWX segment is silenced, since one flat RAM
region is intended.

Gotcha, a reset race in the testbench. After the move, hello reported
1189 cycles instead of 1190, with a byte-identical binary. The bench
released reset with a blocking "reset_n = 1" right after
@(posedge clk). Whether the CPU sees the new value on that same edge
depends on the order the simulator happens to run things in, and adding
a port hop (reset now comes out of soc.v) flipped it. Fixed with a
non-blocking "reset_n <= 1", which always changes after the edge, the
way a real flip-flop output does. Result: back to 1190 cycles, now
deterministic. Lesson: in a testbench, drive anything the DUT samples on
a clock edge with <=, or away from the edge.

Results:
make compare prints "01-femto-quark hello PASS 1190 cycles".
make qemu P=hello still prints hello.
make new E=99 FROM=01 NAME=scratch was tested and the scratch copy then
deleted.

Next: unchanged from the previous entry (waveform walk-through, then an
int8 dot-product benchmark in common/sw/ as the baseline).

## 2026-11-25 — [02] custom opcodes: puts rs1 and abort

Goal: see how far the simple core has to change to take new
instructions. Started with make new E=02 FROM=01 NAME=quark-ops.

Encoding: the custom-0 major opcode, 0x0B, which the RISC-V spec
reserves for exactly this. R-type layout, with funct3 selecting the
operation:
puts rs1 is funct3=000, so puts a0 = 0x0005000b
abort is funct3=001, giving 0x0000100b
The assembler doesn't know them. .insn r 0x0B, f3, 0, rd, rs1, rs2
encodes them, and GNU as macros in exp/02-quark-ops/sw/hello_ops/ops.h
make "puts a0" and "abort" read like real mnemonics.

What it took in the core. It's 96 added or changed lines, every one
marked [exp02].
A decoder fix. Quark decodes JAL from instr[3] alone, an area trick that
holds only while JAL is the only opcode with bit 3 set. All four custom
opcodes (0x0B, 0x2B, 0x5B, 0x7B) have bit 3 set, so puts would have run
as a jump. Replaced it with a full opcode compare. Lesson: minimal cores
cut decode corners, so check what an unused opcode actually does before
claiming it.
Three new one-hot states: PUTS_RD, PUTS_WR and HALT.
New datapath for puts: a pointer register, a byte selector on
mem_rdata, two new inputs on the address mux (pointer, PUTS_ADDR), and a
mux on write-data lane 0.
Write-back disabled for custom ops, and a new output pin, halted.
The bus handshakes are respected (PUTS_RD waits for !mem_wbusy), so a
real, slow UART would still work.

Results:
In hello_ops, the 17-character line takes 311 cycles with the software
puts_ and 43 with the hardware puts, 7.2x faster. That's 2 cycles per
character, against roughly 5 instructions x 3-4 cycles in software.
hello_ops ends with ABORT at pc 0x8000011c after 1292 cycles.
Plain hello on 02 is 1177 cycles, identical to 01.
make lint E=02 is clean, after marking the unused upper bits of the
byte selector with lint_off.

Bug found on the way, in common: an unaligned .bss. In hello_ops the
"\n" of the string disappeared in both the software and hardware print,
which pointed away from the new hardware. Cause: __bss_start was
0x80000192. crt0 clears .bss with sw, and Quark ignores the low address
bits of sw, so it zeroed the word at 0x80000190, the last bytes of
.rodata. Fixed with ALIGN(4) before __bss_start in common/sw/rt/link.ld.

Result change caused by the fix: 01 hello went from 1190 to 1177 cycles.
hello has no .bss, but with an unaligned start the clear loop still ran
once and overwrote a harmless 0 byte. Now the loop is skipped, saving
13 cycles. The 01 README is updated.

make compare now also reports ABORT:
01-femto-quark hello PASS 1177 cycles
02-quark-ops hello PASS 1177 cycles
02-quark-ops hello_ops ABORT 1292 cycles

Ideas this opens up: puts is a tiny DMA engine, a multi-cycle instruction
that owns the bus. A MAC or dot-product instruction can use the same
pattern, streaming two int8 vectors from RAM while the FSM loops.
