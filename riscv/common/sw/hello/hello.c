// Bare-metal hello for RV32I: prints a line and times it with rdcycle.
#include "lib.h"

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
