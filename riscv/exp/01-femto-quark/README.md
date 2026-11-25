# 01-femto-quark: FemtoRV32 Quark, unmodified

Derived from: vendor/femtorv32/femtorv32_quark.v (upstream
BrunoLevy/learn-fpga commit f5ada24), copied into rtl/ unchanged.

What it is: the baseline. A multi-cycle RV32I core (plus rdcycle) driven
by a 4-state one-hot FSM: fetch, wait for instruction, execute, and wait
for a load, store or shift. It takes 3 to 4 cycles per instruction, and
shifts cost 1 cycle per bit.

Testbench: sim/tb.v, which wires the core to common/sim/soc.v. The core
gets ADDR_WIDTH=32 and RESET_ADDR=0x80000000.

Results (make run E=01):
hello PASS after 1177 cycles (1190 before the .bss alignment fix in
common/sw/rt/link.ld, see LOG.md), and the 17-character line takes 314 cycles.
