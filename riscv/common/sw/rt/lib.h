// Tiny runtime shared by all programs. Header-only, no libc.
// Memory map (same as QEMU virt and common/sim/soc.v):
//   0x10000000  UART transmit register (write a byte, it is printed)
#ifndef LAB_LIB_H
#define LAB_LIB_H
#include <stdint.h>

#define UART_THR (*(volatile uint8_t *)0x10000000)

static inline void putc_(char c) { UART_THR = (uint8_t)c; }

static inline void puts_(const char *s) { while (*s) putc_(*s++); }

static inline void puthex(uint32_t v)
{
    puts_("0x");
    for (int i = 28; i >= 0; i -= 4)
        putc_("0123456789abcdef"[(v >> i) & 0xf]);
}

// rdcycle reads the cycle counter. FemtoRV Quark answers any SYSTEM
// opcode with its cycle counter; QEMU implements the real CSR (but its
// value has no relation to real timing).
static inline uint32_t rdcycle(void)
{
    uint32_t c;
    __asm__ volatile("rdcycle %0" : "=r"(c));
    return c;
}

#endif
