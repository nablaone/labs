/* One RAM region at 0x80000000, the address QEMU virt loads -kernel to
 * with -bios none. The Verilog testbench maps its RAM to the same place. */
OUTPUT_ARCH(riscv)
ENTRY(_start)

MEMORY { RAM (rwx) : ORIGIN = 0x80000000, LENGTH = 64K }

SECTIONS
{
    .text : { *(.text.start) *(.text*) } > RAM
    .rodata : { *(.rodata*) *(.srodata*) } > RAM
    .data : { *(.data*) *(.sdata*) } > RAM
    /* crt0 clears .bss with word stores, so both ends must be 4-byte
     * aligned. Quark ignores the low address bits of sw: an unaligned
     * __bss_start makes it wipe the last bytes of .rodata. */
    .bss : {
        . = ALIGN(4);
        __bss_start = .;
        *(.bss*) *(.sbss*) *(COMMON)
        . = ALIGN(4);
        __bss_end = .;
    } > RAM
    __stack_top = ORIGIN(RAM) + LENGTH(RAM);
}
