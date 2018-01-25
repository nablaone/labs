

#include <avr/io.h>

#define LED1 PB3
#define LED2 PB2
#define DELAY 250

void delay_ms(uint8_t ms) {
    uint16_t delay_count = 1000000 / 17500;
    volatile uint16_t i;
    
    while (ms != 0) {
        for (i=0; i != delay_count; i++);
        ms--;
    }
}

int main(void) {
    //CKSEL = 8;
    //int8_t i;
    DDRB = _BV(LED1) | _BV(LED2);
    
    for(;;) {
        PORTB = _BV(LED1);
        delay_ms(DELAY);
        PORTB = _BV(LED2);
        delay_ms(DELAY);
    }
    
    return 0;
}
