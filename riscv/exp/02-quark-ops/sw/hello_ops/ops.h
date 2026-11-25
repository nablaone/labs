// Custom instructions of experiment 02, in the custom-0 opcode space.
//
// The assembler doesn't know them, so .insn spells out the fields:
//   .insn r <opcode>, <funct3>, <funct7>, <rd>, <rs1>, <rs2>
// The two macros below make "puts a0" and "abort" usable as if they were
// real mnemonics, in inline asm here or in a .S file.
#ifndef EXP02_OPS_H
#define EXP02_OPS_H

__asm__(
    ".macro puts rs\n"
    "    .insn r 0x0B, 0, 0, x0, \\rs, x0\n"
    ".endm\n"
    ".macro abort\n"
    "    .insn r 0x0B, 1, 0, x0, x0, x0\n"
    ".endm\n");

// "memory" clobber: the core reads the string from RAM, so the compiler
// must have stored it before this instruction runs.
static inline void hw_puts(const char *s)
{
    __asm__ volatile("puts %0" : : "r"(s) : "memory");
}

static inline __attribute__((noreturn)) void hw_abort(void)
{
    __asm__ volatile("abort");
    __builtin_unreachable();
}

#endif
