

#include <avr/io.h>
#include <util/delay.h>

int main(void) {
    
    DDRB |= _BV(DDB3);
    
    for(;;) {
        _delay_ms(500);
        PORTB ^= _BV(PB3);    
    }
    
    return 0;
}
