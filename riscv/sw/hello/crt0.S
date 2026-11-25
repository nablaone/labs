# Startup for a bare RV32I machine: no CSRs, no traps, no libc.
# Set the stack, zero .bss, call main, then report main's return value
# to the exit device at 0x100000 (QEMU's "sifive_test" device; the
# testbench decodes it the same way):
#   0x5555                 -> pass, exit 0
#   (code << 16) | 0x3333  -> fail with exit code

    .section .text.start
    .globl _start
_start:
    la   sp, __stack_top

    la   t0, __bss_start
    la   t1, __bss_end
1:  bgeu t0, t1, 2f
    sw   zero, 0(t0)
    addi t0, t0, 4
    j    1b
2:
    call main

    li   t0, 0x100000
    bnez a0, 3f
    li   t1, 0x5555
    sw   t1, 0(t0)
    j    4f
3:  slli t1, a0, 16
    li   t2, 0x3333
    or   t1, t1, t2
    sw   t1, 0(t0)
4:  j    4b
