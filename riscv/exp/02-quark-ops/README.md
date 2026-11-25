# 02-quark-ops: Quark + custom-0 instructions puts rs1 and abort

Derived from: 01-femto-quark.

What changed. Every change in rtl/femtorv32_quark.v is marked [exp02].

Two new instructions in the custom-0 opcode space (0x0B, R-type layout,
funct3 selects the operation).

puts rs1 (funct3=000, puts a0 = 0x0005000b) prints the NUL-terminated
string at rs1. The core walks the string itself, using two new FSM
states. PUTS_RD puts the pointer on the bus and strobes a read. PUTS_WR
picks the byte out of the returned word; a zero ends the instruction,
otherwise it writes the byte to PUTS_ADDR, increments the pointer and
goes back to PUTS_RD. That's 2 cycles per character. The new hardware is
one 32-bit pointer register, a byte selector, two more inputs on the
address mux, and a mux on write-data lane 0.

abort (funct3=001, 0x0000100b) moves the FSM to a new HALT state that
only reset leaves. A new output pin, halted, goes high. The testbench
watches it and ends the simulation.

Decoder fix needed first. Quark decodes JAL from the single bit instr[3],
and all four custom opcodes have that bit set, so without the fix puts
would run as a jump. It's now a full 5-bit opcode compare.

Custom ops don't write rd. Unknown custom-0 funct3 values run as a NOP.

The UART address is a new parameter, PUTS_ADDR, because the core now
depends on one address in the SoC memory map.

Software. sw/hello_ops/ops.h defines assembler macros, so "puts a0" and
"abort" work as mnemonics (they expand to .insn r 0x0B, ...), plus the C
wrappers hw_puts() and hw_abort(). QEMU doesn't know these opcodes, so
hello_ops runs only in the simulator. The plain hello still runs
unchanged.

Results (make run E=02):
hello passes in 1177 cycles, the same as 01, because the extensions
don't slow down RV32I code.
In hello_ops, the 17-character line takes 311 cycles with the software
puts_ and 43 cycles with the hardware puts, so 7.2x faster. That's
2 cycles per character plus about 9 for fetch, execute and the NUL.
hello_ops ends with ABORT after 1292 cycles.
