# Experiment 01: FemtoRV32 Quark, unmodified (baseline).
# Paths are relative to this experiment's directory.
EXP_RTL   := rtl/femtorv32_quark.v
EXP_TB    := sim/tb.v
EXP_DEFS  := -DBENCH
# Programs: looked up in this experiment's sw/<name>/ first, then common/sw/<name>/.
EXP_PROGS := hello
EXP_MARCH := rv32i
