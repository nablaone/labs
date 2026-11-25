// Bare-metal hello for RV32I. Prints through a 16550-style UART at
// 0x10000000 (QEMU virt). Only the transmit register is used, and
// nothing waits on the status register, which is fine for QEMU and the
// testbench.
#include <stdint.h>

#define UART_THR (*(volatile uint8_t *)0x10000000)

static void putc_(char c) { UART_THR = (uint8_t)c; }

static void puts_(const char *s) { while (*s) putc_(*s++); }

static void puthex(uint32_t v)
{
    puts_("0x");
    for (int i = 28; i >= 0; i -= 4)
        putc_("0123456789abcdef"[(v >> i) & 0xf]);
}

// rdcycle reads the cycle counter. FemtoRV Quark answers any SYSTEM
// opcode with its cycle counter, and QEMU implements the real CSR.
static inline uint32_t rdcycle(void)
{
    uint32_t c;
    __asm__ volatile("rdcycle %0" : "=r"(c));
    return c;
}

int main(void)
{
    uint32_t t0 = rdcycle();
    puts_("hello from rv32i\n");
    uint32_t t1 = rdcycle();
    puts_("cycles for that line: ");
    puthex(t1 - t0);
    putc_('\n');
    return 0;
}
