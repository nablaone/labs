# Experiment 02: FemtoRV32 Quark + custom-0 instructions 'puts rs1' and 'abort'.
# Paths are relative to this experiment's directory.
EXP_RTL   := rtl/femtorv32_quark.v
EXP_TB    := sim/tb.v
EXP_DEFS  := -DBENCH
# Programs: looked up in this experiment's sw/<name>/ first, then common/sw/<name>/.
# hello still runs unchanged: the core remains a plain RV32I core.
EXP_PROGS := hello hello_ops
EXP_MARCH := rv32i
