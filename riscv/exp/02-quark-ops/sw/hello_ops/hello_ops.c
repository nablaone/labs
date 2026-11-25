// hello, using the experiment-02 custom instructions. Prints the same
// line with the software puts_ (lib.h) and with the hardware 'puts', times
// both with rdcycle, and ends with 'abort' instead of returning to crt0.
#include "lib.h"
#include "ops.h"

static const char msg[] = "hello from rv32i\n";

int main(void)
{
    uint32_t t0 = rdcycle();
    puts_(msg);
    uint32_t t1 = rdcycle();
    hw_puts(msg);
    uint32_t t2 = rdcycle();

    hw_puts("software puts_: ");
    puthex(t1 - t0);
    hw_puts(" cycles\nhardware puts:  ");
    puthex(t2 - t1);
    hw_puts(" cycles\n");

    hw_abort();
}
